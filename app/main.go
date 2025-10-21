package main

import (
	"bytes"
	"compress/gzip"
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
	EchoEndpointRegx = regexp.MustCompile(`\/echo\/(?P<str>.*)`)
	FileEndpointRegx = regexp.MustCompile(`\/files\/(?P<str>.*)`)
)

// RootHandler handles the root endpoint
func RootHandler(c *ConnHandler) {
	c.Status(200)
	c.Body(nil)
}

// NotFoundHandler handles the endpoint not found
func NotFoundHandler(c *ConnHandler) {
	c.Status(404)
	c.Body(nil)
}

// BadReqHandler sends the 400 - Bad Request response
func BadReqHandler(c *ConnHandler) {
	c.Status(400)
	c.Body(nil)
}

// InternalServerErrHandler sends the 500 - internal server error response
func InternalServerErrHandler(c *ConnHandler) {
	c.Status(500)
	c.Body(nil)
}

// EchoHandler handles the request for /echo/<str> endpoint
func EchoHandler(c *ConnHandler) {
	str := EchoEndpointRegx.FindStringSubmatch(c.req.RequestTarget)[1]

	// Check if we can compress the body in gzip
	var shouldCompress bool
	if acceptEncoding, ok := c.req.Headers["Accept-Encoding"]; ok {
		encSchemes := strings.SplitSeq(acceptEncoding, ",")
		for encScheme := range encSchemes {
			if strings.TrimSpace(encScheme) == "gzip" {
				shouldCompress = true
				break
			}
		}
	}

	if shouldCompress {
		var b bytes.Buffer
		gzWriter := gzip.NewWriter(&b)
		if _, err := gzWriter.Write([]byte(str)); err != nil {
			fmt.Println("Failed to compress the data: ", err.Error())
			c.Status(500)
			c.Body(nil)
			return
		}

		gzWriter.Close()

		c.Status(200)
		c.Header("Content-Type", "text/plain")
		c.Header("Content-Encoding", "gzip")
		c.Header("Content-Length", len(b.Bytes()))
		c.Body(b.Bytes())
	} else {
		c.Status(200)
		c.Header("Content-Type", "text/plain")
		c.Header("Content-Length", len(str))
		c.Body([]byte(str))
	}
}

// UserAgentHandler handles the request for /user-endpoint endpoint
func UserAgentHandler(c *ConnHandler) {
	val, ok := c.req.Headers["User-Agent"]
	if !ok {
		fmt.Println("No 'User-Agent' header present!")
		os.Exit(1)
	}

	c.Status(200)
	c.Header("Content-Type", "text/plain")
	c.Header("Content-Length", len(val))
	c.Body([]byte(val))
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

	c.Status(200)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", len(content))
	c.Body(content)
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

	c.Status(201)
	c.Body(nil)
}

// HandleConnection handles the single connect request
func HandleConnection(conn net.Conn, flags map[string]any) {
	defer conn.Close()

	for {
		// Create the handler for the request
		c, err := NewConnHandler(conn)
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Println("Error creating the handler: ", err.Error())
			continue
		}

		// Add the connection close header in response if present in the request
		var shouldCloseConn bool
		if close, ok := c.req.Headers["Connection"]; ok && close == "close" {
			c.Header("Connection", "close")
			shouldCloseConn = true
		}

		// Select endpoint handler based on the request
		switch {
		case c.req.RequestTarget == "/":
			RootHandler(c)

		case c.req.RequestTarget == "/user-agent":
			UserAgentHandler(c)

		case EchoEndpointRegx.Match([]byte(c.req.RequestTarget)):
			EchoHandler(c)

		case FileEndpointRegx.Match([]byte(c.req.RequestTarget)):
			dir := isDirExists(flags)
			if dir == "" {
				fmt.Println("Directory name not provided!")
				InternalServerErrHandler(c)
				return
			}

			filename := FileEndpointRegx.FindStringSubmatch(c.req.RequestTarget)[1]
			if filename == "" {
				fmt.Println("No filename provided")
				BadReqHandler(c)
				return
			}

			if c.req.HTTPMethod == "GET" {
				GetFileHandler(c, dir, filename)
			} else {
				SaveFileHandler(c, dir, filename)
			}

		default:
			NotFoundHandler(c)
		}

		// Close the connection
		if shouldCloseConn {
			conn.Close()
			return
		}
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
			continue
		}

		// Handle the connection in a separate goroutine
		go HandleConnection(conn, flags)
	}
}
