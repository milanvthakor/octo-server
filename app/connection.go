package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	CRLF = "\r\n"
)

// ConnHandler binds the connection with methods for parsing the request details and serving multiple endpoints
type ConnHandler struct {
	conn net.Conn

	// request details
	reqLine   *RequestLine
	reqHeader Headers

	// response details
	respStatus  string
	respHeaders Headers
}

// RequestLine represents the details of the request like http method, target, and http version.
type RequestLine struct {
	HTTPMethod    string
	RequestTarget string
	HTTPVersion   string
}

func NewConnHandler(conn net.Conn) (*ConnHandler, error) {
	// Read the request line from the connection
	reqLine, err := ReadRequestLine(conn)
	if err != nil {
		fmt.Println("Error reading the request line: ", err)
		return nil, err
	}

	// Read the request header
	reqHeader, err := ReadRequestHeader(conn)
	if err != nil {
		fmt.Println("Error reading the connection: ", err)
		os.Exit(1)
	}

	return &ConnHandler{
		conn:        conn,
		reqLine:     reqLine,
		reqHeader:   reqHeader,
		respHeaders: make(Headers),
	}, nil
}

// ReadRequestBody reads the request body
func (c *ConnHandler) ReadRequestBody() ([]byte, error) {
	strContLen, ok := c.reqHeader["Content-Length"]
	if !ok {
		return nil, errors.New("header 'Content-Length' is missing")
	}

	contLen, err := strconv.Atoi(strContLen)
	if err != nil {
		return nil, err
	}

	data := make([]byte, contLen)
	_, err = c.conn.Read(data)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return data, nil
}

// Status set the status for the response
func (c *ConnHandler) Status(status string) {
	c.respStatus = status
}

// Header set the header for the response
func (c *ConnHandler) Header(key string, val any) {
	c.respHeaders[key] = fmt.Sprint(val)
}

// Body sends the given body to the response
func (c *ConnHandler) Body(blob []byte) {
	// Create the response status
	status := "HTTP/1.1 " + strings.TrimSpace(c.respStatus)

	// Convert the map to the slice
	var header string
	for k, v := range c.respHeaders {
		header += k + ": " + v + "\r\n"
	}

	// Prepare the entire response
	resp := fmt.Appendf(nil, "%s\r\n%s\r\n%s", status, header, blob)

	if _, err := c.conn.Write(resp); err != nil {
		fmt.Println("Error returning response: ", err)
		os.Exit(1)
	}
}

// ReadUntilCRLF reads from the connection until it finds a CRLF sequence.
// It returns the string up to the CRLF sequence.
func ReadUntilCRLF(conn net.Conn) (string, error) {
	var data []byte

	for {
		aByte := make([]byte, 1)
		_, err := conn.Read(aByte)
		if err == io.EOF {
			// Connection closed by peer
			return string(data), io.EOF
		} else if err != nil {
			fmt.Println("Error reading the request data: ", err)
			return "", err
		}

		data = append(data, aByte...)

		// Check for the CRLF sequence
		if len(data) >= 2 && string(data[len(data)-2:]) == CRLF {
			return string(data[:len(data)-2]), nil
		}
	}
}

// ReadRequestLine reads the request line from the request connection
func ReadRequestLine(conn net.Conn) (*RequestLine, error) {
	rawReqLine, err := ReadUntilCRLF(conn)
	if err != nil {
		fmt.Println("Error reading the request line: ", err)
		return nil, err
	}

	tokens := strings.Split(rawReqLine, " ")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	return &RequestLine{
		HTTPMethod:    tokens[0],
		RequestTarget: tokens[1],
		HTTPVersion:   tokens[2],
	}, nil
}

// Headers represents the list of headers from the request/response
type Headers map[string]string

// ReadRequestHeader reads the request header from the request connection
func ReadRequestHeader(conn net.Conn) (Headers, error) {
	reqHeader := make(map[string]string)

	for {
		header, err := ReadUntilCRLF(conn)
		if err != nil {
			fmt.Println("Error reading the header: ", err)
			return nil, err
		}

		if header == "" {
			break
		}

		tokens := strings.Split(header, ":")
		if len(tokens) < 2 {
			return nil, fmt.Errorf("invalid header")
		}

		reqHeader[strings.TrimSpace(tokens[0])] = strings.TrimSpace(strings.Join(tokens[1:], ":"))
	}

	return reqHeader, nil
}
