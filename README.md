# Conn Package

[![Go Reference](https://pkg.go.dev/badge/github.com/NodePassProject/conn.svg)](https://pkg.go.dev/github.com/NodePassProject/conn)
[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

A simple and efficient TCP connection data exchange utility for Go applications.

## Features

- Thread-safe, bidirectional data exchange between two TCP connections
- Idle timeout support for automatic connection cleanup
- Efficient error handling and resource management
- `TimeoutReader` for per-read timeout control
- `StatConn` for connection statistics tracking (RX/TX bytes)
- `RateLimiter` for bandwidth control using token bucket algorithm
- Global rate limiting with separate read/write speed controls

## Installation

```bash
go get github.com/NodePassProject/conn
```

## Usage

### TCP to TCP Data Exchange

```go
package main

import (
    "fmt"
    "net"
    "time"
    "github.com/NodePassProject/conn"
)

func main() {
    // Example with two TCP connections
    conn1, err := net.Dial("tcp", "server1.example.com:8080")
    if err != nil {
        fmt.Printf("Failed to connect to server1: %v\n", err)
        return
    }
    defer conn1.Close()

    conn2, err := net.Dial("tcp", "server2.example.com:9090")
    if err != nil {
        fmt.Printf("Failed to connect to server2: %v\n", err)
        return
    }
    defer conn2.Close()

    // Optional: Create rate limiter (1MB/s read, 512KB/s write)
    rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)

    // Optional: Wrap connections with StatConn for statistics and rate limiting
    statConn1 := &conn.StatConn{Conn: conn1, Rate: rateLimiter}
    statConn2 := &conn.StatConn{Conn: conn2, Rate: rateLimiter}

    // Exchange data between the two connections with a 5-second idle timeout
    err = conn.DataExchange(statConn1, statConn2, 5*time.Second)
    if err != nil && err.Error() != "EOF" {
        fmt.Printf("Data exchange error: %v\n", err)
    }
}
```

### TimeoutReader

`TimeoutReader` is a wrapper for `net.Conn` that allows you to set a read timeout for each read operation. It is used internally by `DataExchange`, but can also be used directly if needed:

```go
import "github.com/NodePassProject/conn"

tr := &conn.TimeoutReader{Conn: tcpConn, Timeout: 5 * time.Second}
buf := make([]byte, 4096)
n, err := tr.Read(buf)
```

### StatConn

`StatConn` is a wrapper for `net.Conn` that tracks connection statistics (received and transmitted bytes) and supports optional rate limiting. It implements the `net.Conn` interface and can be used as a drop-in replacement:

```go
import (
    "sync/atomic"
    "github.com/NodePassProject/conn"
)

// Basic usage without rate limiting
var rxBytes, txBytes uint64
statConn := &conn.StatConn{
    Conn: tcpConn,
    RX:   &rxBytes,
    TX:   &txBytes,
}

// Usage with rate limiting (1MB/s read, 512KB/s write)
rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)
statConnWithLimit := &conn.StatConn{
    Conn: tcpConn,
    RX:   &rxBytes,
    TX:   &txBytes,
    Rate: rateLimiter, // Enable rate limiting
}

// Use statConn like a normal net.Conn
// The rxBytes and txBytes variables will be updated automatically
// Rate limiting is applied automatically if Rate is set
n, err := statConnWithLimit.Write(data)
fmt.Printf("Total bytes sent: %d\n", atomic.LoadUint64(&txBytes))
fmt.Printf("Total bytes received: %d\n", atomic.LoadUint64(&rxBytes))
```

### RateLimiter

`RateLimiter` implements a token bucket algorithm for bandwidth control. It supports separate rate limiting for read and write operations:

```go
import "github.com/NodePassProject/conn"

// Create a rate limiter with 1MB/s read and 512KB/s write limits
rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)

// Use with StatConn for automatic rate limiting
var rxBytes, txBytes uint64
statConn := &conn.StatConn{
    Conn: tcpConn,
    RX:   &rxBytes,
    TX:   &txBytes,
    Rate: rateLimiter, // Enable rate limiting
}

// All read/write operations will be automatically rate limited
data := make([]byte, 4096)
n, err := statConn.Read(data)  // Automatically applies read rate limit
n, err = statConn.Write(data)  // Automatically applies write rate limit
```

You can also use the rate limiter directly:

```go
rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)

// Manual rate limiting
dataSize := int64(len(data))
rateLimiter.WaitWrite(dataSize)  // Wait for write tokens
n, err := conn.Write(data)

rateLimiter.WaitRead(int64(n))   // Wait for read tokens (if needed)
```

**Rate Limiter Features:**
- Token bucket algorithm for smooth traffic shaping
- Separate read and write rate controls
- Thread-safe implementation using atomic operations
- Zero value means unlimited rate (set to 0 or negative values)
- Automatic token refill based on configured rates

## License

Copyright (c) 2025, NodePassProject. Licensed under the BSD 3-Clause License.
See the [LICENSE](LICENSE) file for details.