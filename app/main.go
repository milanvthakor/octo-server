package main

import (
	"fmt"
	"net"
	"os"
	"bytes"
	"io"
	"strings"
)

var CRLF = []byte{'\r', '\n'}

// ReadUntilCRLF reads from the buffer until it finds a CRLF sequence.
// It returns the string up to the CRLF sequence. 
func ReadUntilCRLF(b *bytes.Buffer) (string, error) {
	// Search for the CRLF sequence in the unread portion of the buffer.
	index := bytes.Index(b.Bytes(), CRLF)
	if index >= 0 { // Sequence found
		// Read the data upto CRLF
		lineBytes := b.Next(index)
		return string(lineBytes), nil
	}

	// Sequence not found yet.
	// If the buffer is empty, we have reached the end
	if b.Len() == 0 {
		return "", io.EOF
	}

	// Although, sequence doesn't have the CRLF, but in our case, this could be
	// the request body. Hence, read the remaining data and return it.
	remainBytes := b.Bytes()
	return string(remainBytes), io.EOF
}

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	conn, err := l.Accept()
	if err != nil {
	 	fmt.Println("Error accepting connection: ", err.Error())
	 	os.Exit(1)
	}
	defer conn.Close()

	// Read the data from the connection
	var (
		reqData bytes.Buffer
		reqLine string
	)
	buffer := make([]byte, 1)
	for {
		n, err := conn.Read(buffer)
		if err == io.EOF {
			// Connection closed by peer
			break
		} else if err != nil {
			fmt.Println("Error reading the request: ", err.Error())
			os.Exit(1)
		}

		reqData.Write(buffer[:n])
		
		// Read the 'Request Line'
		rl, err := ReadUntilCRLF(&reqData)
		if err == nil && len(rl) > 1 {
			reqLine = rl
			break
		}
	}

	// Split the request line and get the 'Request Target'
	reqLineTokens := strings.Split(reqLine, " ")
	if len(reqLineTokens) != 3 {
		fmt.Println("Invalid request line")
		os.Exit(1)
	}

	reqTarget := reqLineTokens[1]
	var resp []byte
	if reqTarget != "/" {
		resp = []byte("HTTP/1.1 404 Not Found\r\n\r\n")
	} else {
		resp = []byte("HTTP/1.1 200 OK\r\n\r\n")
	}

	// Write the response back to client
	_, err = conn.Write(resp)
	if err != nil {
		fmt.Println("Error returning response: ", err.Error())
		os.Exit(1)
	}
}
