package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	CRLF = "\r\n"
)

// ConnHandler binds the connection with methods for parsing the request details and serving multiple endpoints
type ConnHandler struct {
	conn net.Conn
	req  *Request
	resp *Response
}

// Request represents the details of the request
type Request struct {
	HTTPMethod    string
	RequestTarget string
	HTTPVersion   string

	Headers map[string]string
}

// Response represents the details of the response
type Response struct {
	StatusCode int
	Status     string

	Headers map[string]string
}

func NewConnHandler(conn net.Conn) (*ConnHandler, error) {
	c := &ConnHandler{
		conn: conn,
	}

	// Read the request line
	if req, err := readReqLine(conn); err != nil {
		return nil, err
	} else {
		c.req = req
	}

	// Read the request header
	if reqHeaders, err := readReqHeaders(conn); err != nil {
		return nil, err
	} else {
		c.req.Headers = reqHeaders
	}

	c.resp = &Response{
		Headers: make(map[string]string),
	}

	return c, nil
}

// ReadRequestBody reads the request body
func (c *ConnHandler) ReadRequestBody() ([]byte, error) {
	strContLen, ok := c.req.Headers["Content-Length"]
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

// Status sets the status for the response
func (c *ConnHandler) Status(statusCode int) {
	c.resp.StatusCode = statusCode

	switch statusCode {
	case 200:
		c.resp.Status = "OK"
	case 201:
		c.resp.Status = "Created"
	case 400:
		c.resp.Status = "Bad Request"
	case 404:
		c.resp.Status = "Not Found"
	case 500:
		c.resp.Status = "Internal Server Error"
	}
}

// Header sets the header for the response
func (c *ConnHandler) Header(key string, val any) {
	c.resp.Headers[key] = fmt.Sprint(val)
}

// Body sends the given body to the response
func (c *ConnHandler) Body(blob []byte) {
	// Create the response status
	status := fmt.Sprintf("HTTP/1.1 %d %s", c.resp.StatusCode, c.resp.Status)

	// Convert the map to the slice
	var header string
	for k, v := range c.resp.Headers {
		header += k + ": " + v + "\r\n"
	}

	// Prepare the entire response
	resp := fmt.Appendf(nil, "%s\r\n%s\r\n%s", status, header, blob)

	if _, err := c.conn.Write(resp); err != nil {
		fmt.Println("Error returning response: ", err)
		os.Exit(1)
	}
}

// readUntilCRLF reads from the connection until it finds a CRLF sequence.
// It returns the string up to the CRLF sequence.
func readUntilCRLF(conn net.Conn) (string, error) {
	conn.SetReadDeadline(time.Now().Add(time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Reset deadline

	reader := bufio.NewReader(conn)
	var buf bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// Connection closed by peer
				return buf.String(), io.EOF
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return buf.String(), nil
			}

			return "", err
		}

		buf.Write(line)

		// Check for the CRLF sequence
		result := buf.String()

		if len(result) >= 2 && result[len(result)-2:] == CRLF {
			return result[:len(result)-2], nil // Strip CRLF
		}
	}
}

// readReqLine reads the request line from the request connection
func readReqLine(conn net.Conn) (*Request, error) {
	rawReqLine, err := readUntilCRLF(conn)
	if err != nil {
		return nil, err
	}

	tokens := strings.Split(rawReqLine, " ")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	return &Request{
		HTTPMethod:    tokens[0],
		RequestTarget: tokens[1],
		HTTPVersion:   tokens[2],
	}, nil
}

// readReqHeaders reads the headers from the request connection
func readReqHeaders(conn net.Conn) (map[string]string, error) {
	headers := make(map[string]string)

	for {
		rawHeader, err := readUntilCRLF(conn)
		if err != nil {
			return nil, err
		}

		if rawHeader == "" {
			break
		}

		tokens := strings.Split(rawHeader, ":")
		if len(tokens) < 2 {
			return nil, fmt.Errorf("invalid header")
		}

		headers[strings.TrimSpace(tokens[0])] = strings.TrimSpace(strings.Join(tokens[1:], ":"))
	}

	return headers, nil
}
