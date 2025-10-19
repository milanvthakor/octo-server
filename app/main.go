package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
)

var (
	CRLF             = "\r\n"
	EchoEndpointRegx = regexp.MustCompile(`\/echo\/(?P<str>.*)`)
)

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

// RequestLine represents the details of the request like http method, target, and http version.
type RequestLine struct {
	HTTPMethod    string
	RequestTarget string
	HTTPVersion   string
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

// RequestHeader represents the list of headers from the request
type RequestHeader map[string]string

// ReadRequestHeader reads the request header from the request connection
func ReadRequestHeader(conn net.Conn) (RequestHeader, error) {
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

		reqHeader[tokens[0]] = strings.Join(tokens[1:], ":")
	}

	return reqHeader, nil
}

// SendResponse sends the given resp to the client
func SendResponse(conn net.Conn, resp []byte) {
	if _, err := conn.Write(resp); err != nil {
		fmt.Println("Error returning response: ", err)
		os.Exit(1)
	}
}

// RootHandler handles the root endpoint
func RootHandler(conn net.Conn) {
	SendResponse(conn, []byte("HTTP/1.1 200 OK\r\n\r\n"))
}

// NotFoundHandler handles the endpoint not found
func NotFoundHandler(conn net.Conn) {
	SendResponse(conn, []byte("HTTP/1.1 404 Not Found\r\n\r\n"))
}

// EchoHandler handles the request for /echo/<str> endpoint
func EchoHandler(conn net.Conn, reqLine *RequestLine) {
	matches := EchoEndpointRegx.FindStringSubmatch(reqLine.RequestTarget)
	SendResponse(conn, fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(matches[1]), matches[1]))
}

// UserAgentHandler handles the request for /user-endpoint header
func UserAgentHandler(conn net.Conn, reqHeader RequestHeader) {
	val, ok := reqHeader["User-Agent"]
	if !ok {
		fmt.Println("No 'User-Agent' header present!")
		os.Exit(1)
	}

	SendResponse(conn, fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(val), val))
}

func main() {
	// Creates an HTTP server
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	// Wait for a connection
	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	// Read the request line from the connection
	reqLine, err := ReadRequestLine(conn)
	if err != nil {
		fmt.Println("Error reading the request line: ", err)
		os.Exit(1)
	}

	// Read the request header
	reqHeader, err := ReadRequestHeader(conn)
	if err != nil {
		fmt.Println("Error reading the connection: ", err)
		os.Exit(1)
	}

	if reqLine.RequestTarget == "/" {
		RootHandler(conn)
	} else if reqLine.RequestTarget == "/user-agent" {
		UserAgentHandler(conn, reqHeader)
	} else if matches := EchoEndpointRegx.FindStringSubmatch(reqLine.RequestTarget); len(matches) > 0 {
		EchoHandler(conn, reqLine)
	} else {
		NotFoundHandler(conn)
	}
}
