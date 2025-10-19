package main

import (
	"errors"
	"flag"
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
	FileEndpointRegx = regexp.MustCompile(`\/files\/(?P<str>.*)`)
)

// RequestLine represents the details of the request like http method, target, and http version.
type RequestLine struct {
	HTTPMethod    string
	RequestTarget string
	HTTPVersion   string
}

// ConnHandler binds the connection with methods for parsing the request details and serving multiple endpoints
type ConnHandler struct {
	conn net.Conn
}

// ReadUntilCRLF reads from the connection until it finds a CRLF sequence.
// It returns the string up to the CRLF sequence.
func (c *ConnHandler) ReadUntilCRLF() (string, error) {
	var data []byte

	for {
		aByte := make([]byte, 1)
		_, err := c.conn.Read(aByte)
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
func (c *ConnHandler) ReadRequestLine() (*RequestLine, error) {
	rawReqLine, err := c.ReadUntilCRLF()
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
func (c *ConnHandler) ReadRequestHeader() (RequestHeader, error) {
	reqHeader := make(map[string]string)

	for {
		header, err := c.ReadUntilCRLF()
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
func (c *ConnHandler) SendResponse(resp []byte) {
	if _, err := c.conn.Write(resp); err != nil {
		fmt.Println("Error returning response: ", err)
		os.Exit(1)
	}
}

// RootHandler handles the root endpoint
func (c *ConnHandler) RootHandler() {
	c.SendResponse([]byte("HTTP/1.1 200 OK\r\n\r\n"))
}

// NotFoundHandler handles the endpoint not found
func (c *ConnHandler) NotFoundHandler() {
	c.SendResponse([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
}

// EchoHandler handles the request for /echo/<str> endpoint
func (c *ConnHandler) EchoHandler(reqLine *RequestLine) {
	matches := EchoEndpointRegx.FindStringSubmatch(reqLine.RequestTarget)
	c.SendResponse(fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(matches[1]), matches[1]))
}

// UserAgentHandler handles the request for /user-endpoint endpoint
func (c *ConnHandler) UserAgentHandler(reqHeader RequestHeader) {
	val, ok := reqHeader["User-Agent"]
	if !ok {
		fmt.Println("No 'User-Agent' header present!")
		os.Exit(1)
	}

	c.SendResponse(fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(val), val))
}

// FileHandler handles the request for /files/{filename} endpoint
func (c *ConnHandler) FileHandler(flags map[string]any, reqLine *RequestLine) {
	dir, ok := flags["directory"].(string)
	if !ok {
		fmt.Println("No director provided to serve the file")
		os.Exit(1)
	}
	// Check if the directory exists or not
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		fmt.Println("Directory doesn't exists")
		os.Exit(1)
	}

	filename := FileEndpointRegx.FindStringSubmatch(reqLine.RequestTarget)[1]
	// Open the file
	file, err := os.Open(dir + "/" + filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.NotFoundHandler()
			return
		} else {
			fmt.Println("Error opening the file: ", err.Error())
			os.Exit(1)
		}
	}
	defer file.Close()

	// Read the file
	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Failed to read the file: ", err.Error())
		os.Exit(1)
	}

	c.SendResponse(fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", len(content), string(content)))
}

// HandleConnection handles the single connect request
func (c *ConnHandler) HandleConnection(flags map[string]any) {
	defer c.conn.Close()

	// Read the request line from the connection
	reqLine, err := c.ReadRequestLine()
	if err != nil {
		fmt.Println("Error reading the request line: ", err)
		os.Exit(1)
	}

	// Read the request header
	reqHeader, err := c.ReadRequestHeader()
	if err != nil {
		fmt.Println("Error reading the connection: ", err)
		os.Exit(1)
	}

	if reqLine.RequestTarget == "/" {
		c.RootHandler()
	} else if reqLine.RequestTarget == "/user-agent" {
		c.UserAgentHandler(reqHeader)
	} else if matches := EchoEndpointRegx.FindStringSubmatch(reqLine.RequestTarget); len(matches) > 0 {
		c.EchoHandler(reqLine)
	} else if matches := FileEndpointRegx.FindStringSubmatch(reqLine.RequestTarget); len(matches) > 0 {
		c.FileHandler(flags, reqLine)
	} else {
		c.NotFoundHandler()
	}
}

func main() {
	// Directory flag for the /files/{filename} endpoint
	directory := flag.String("directory", "", "The directory from which files should be served.")
	// Parse the CLI args to populate the flag variables.
	flag.Parse()
	// Store it in the map
	flags := map[string]any{
		"directory": *directory,
	}

	// Creates an HTTP server
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		// Wait for a connection
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		c := &ConnHandler{
			conn,
		}
		// Handle the connection in a separate goroutine
		go c.HandleConnection(flags)
	}
}
