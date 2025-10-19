package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
)

var (
	EchoEndpointRegx = regexp.MustCompile(`\/echo\/(?P<str>.*)`)
	FileEndpointRegx = regexp.MustCompile(`\/files\/(?P<str>.*)`)
)

// SendResponse sends the given resp to the client
func SendResponse(c *ConnHandler, resp []byte) {
	if _, err := c.conn.Write(resp); err != nil {
		fmt.Println("Error returning response: ", err)
		os.Exit(1)
	}
}

// RootHandler handles the root endpoint
func RootHandler(c *ConnHandler) {
	SendResponse(c, []byte("HTTP/1.1 200 OK\r\n\r\n"))
}

// NotFoundHandler handles the endpoint not found
func NotFoundHandler(c *ConnHandler) {
	SendResponse(c, []byte("HTTP/1.1 404 Not Found\r\n\r\n"))
}

// BadReqHandler sends the 400 - Bad Request response
func BadReqHandler(c *ConnHandler) {
	SendResponse(c, []byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
}

// InternalServerErrHandler sends the 500 - internal server error response
func InternalServerErrHandler(c *ConnHandler) {
	SendResponse(c, []byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
}

// EchoHandler handles the request for /echo/<str> endpoint
func EchoHandler(c *ConnHandler) {
	str := EchoEndpointRegx.FindStringSubmatch(c.reqLine.RequestTarget)[1]
	SendResponse(c, fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(str), str))
}

// UserAgentHandler handles the request for /user-endpoint endpoint
func UserAgentHandler(c *ConnHandler) {
	val, ok := c.reqHeader["User-Agent"]
	if !ok {
		fmt.Println("No 'User-Agent' header present!")
		os.Exit(1)
	}

	SendResponse(c, fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(val), val))
}

// GetFileHandler handles the request for the GET /files/{filename} endpoint
func GetFileHandler(c *ConnHandler, dir, filename string) {
	// Open the file
	file, err := os.Open(dir + "/" + filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			NotFoundHandler(c)
			return
		} else {
			fmt.Println("Error opening the file: ", err.Error())
			InternalServerErrHandler(c)
			return
		}
	}
	defer file.Close()

	// Read the file
	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("Failed to read the file: ", err.Error())
		InternalServerErrHandler(c)
		return
	}

	SendResponse(c, fmt.Appendf(nil, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", len(content), string(content)))
}

// SaveFileHandler handles the request for the POST /files/{filename} endpoint
func SaveFileHandler(c *ConnHandler, dir, filename string) {
	// Read the request payload
	rawBody, err := c.ReadRequestBody()
	if err != nil {
		fmt.Println("Failed to read the req body: ", err.Error())
		InternalServerErrHandler(c)
		return
	}

	// Write the data to the file
	if err := os.WriteFile(dir+"/"+filename, rawBody, os.ModePerm); err != nil {
		fmt.Println("Failed to write to the file: ", err.Error())
		InternalServerErrHandler(c)
		return
	}

	SendResponse(c, []byte("HTTP/1.1 201 Created\r\n\r\n"))
}

// HandleConnection handles the single connect request
func HandleConnection(c *ConnHandler, flags map[string]any) {
	defer c.conn.Close()

	// Select endpoint handler based on the request
	switch {
	case c.reqLine.RequestTarget == "/":
		RootHandler(c)

	case c.reqLine.RequestTarget == "/user-agent":
		UserAgentHandler(c)

	case EchoEndpointRegx.Match([]byte(c.reqLine.RequestTarget)):
		EchoHandler(c)

	case FileEndpointRegx.Match([]byte(c.reqLine.RequestTarget)):
		dir := IsDirExists(flags)
		if dir == "" {
			fmt.Println("Directory name not provided!")
			InternalServerErrHandler(c)
			return
		}

		filename := FileEndpointRegx.FindStringSubmatch(c.reqLine.RequestTarget)[1]
		if filename == "" {
			fmt.Println("No filename provided")
			BadReqHandler(c)
			return
		}

		if c.reqLine.HTTPMethod == "GET" {
			GetFileHandler(c, dir, filename)
		} else {
			SaveFileHandler(c, dir, filename)
		}

	default:
		NotFoundHandler(c)
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

		connHandler, err := NewConnHandler(conn)
		if err != nil {
			fmt.Println("Error creating the handler: ", err.Error())
			os.Exit(1)
		}

		// Handle the connection in a separate goroutine
		go HandleConnection(connHandler, flags)
	}
}
