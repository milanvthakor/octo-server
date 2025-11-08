# Octo Server

A lightweight HTTP/1.1 server implementation in Go, built from scratch. This server handles multiple concurrent connections and supports various HTTP endpoints including file operations, content compression, and standard HTTP methods.

## Features

- **HTTP/1.1 Protocol**: Full support for HTTP/1.1 request/response handling
- **Concurrent Connections**: Handles multiple clients simultaneously using goroutines
- **File Operations**: GET and POST endpoints for file serving and storage
- **Content Compression**: Automatic gzip compression when supported by the client

## Supported Endpoints

- `GET /` - Root endpoint
- `GET /echo/<str>` - Echoes back the string with optional gzip compression
- `GET /user-agent` - Returns the User-Agent header from the request
- `GET /files/<filename>` - Retrieves and serves a file
- `POST /files/<filename>` - Saves request body content to a file

## Developer Setup

### Prerequisites

- Go 1.24 or later installed on your system

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd octo-server
```

### Running the Server

1. Build the application:
```bash
go build -o http-server ./app
```

2. Run the server:
```bash
./http-server
```

The server will start on port `4221` by default.

### Running with Options

**Specify a directory for file operations:**
```bash
./http-server -directory /path/to/files
```

**Specify a custom port:**
```bash
./http-server -port 8080
```

### Testing the Server

Once the server is running, you can test it using `curl`:

```bash
# Test root endpoint
curl http://localhost:4221/

# Test echo endpoint
curl http://localhost:4221/echo/hello

# Test echo with compression
curl -H "Accept-Encoding: gzip" --compressed http://localhost:4221/echo/hello

# Test user-agent endpoint
curl -H "User-Agent: MyApp/1.0" http://localhost:4221/user-agent

# Test file GET (requires -directory flag)
curl http://localhost:4221/files/example.txt

# Test file POST (requires -directory flag)
echo "Hello, World!" | curl -X POST -d @- http://localhost:4221/files/example.txt
```