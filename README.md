# Conn Package

[![Go Reference](https://pkg.go.dev/badge/github.com/NodePassProject/conn.svg)](https://pkg.go.dev/github.com/NodePassProject/conn)
[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

A comprehensive Go package for network connection management with advanced features including statistics tracking, rate limiting, and protocol-specific operations.

## Features

- Thread-safe, bidirectional data exchange between network connections
- Idle timeout support for automatic connection cleanup
- Customizable buffer size for optimized performance in data exchange
- Efficient error handling and resource management with connection validation
- `TimeoutReader` for per-read timeout control
- `StatConn` for connection statistics tracking (RX/TX bytes) with protocol-specific methods
- `RateLimiter` for bandwidth control using token bucket algorithm
- Global rate limiting with separate read/write speed controls
- TCP-specific methods (KeepAlive, NoDelay, Linger, CloseRead/Write)
- UDP-specific methods (ReadFromUDP, WriteToUDP, Buffer control)
- Connection type detection and safe type conversion

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
    var rx1, tx1, rx2, tx2 uint64
    statConn1 := conn.NewStatConn(conn1, &rx1, &tx1, rateLimiter)
    statConn2 := conn.NewStatConn(conn2, &rx2, &tx2, rateLimiter)

    // Configure TCP-specific options if needed
    if statConn1.IsTCP() {
        statConn1.SetKeepAlive(true)
        statConn1.SetKeepAlivePeriod(30 * time.Second)
        statConn1.SetNoDelay(true) // Disable Nagle algorithm for low latency
    }

    // Create custom buffer for better performance (optional, nil uses default)
    buffer := make([]byte, 64*1024) // 64KB buffer
    
    // Exchange data between the two connections with a 5-second idle timeout and custom buffer
    err = conn.DataExchange(statConn1, statConn2, 5*time.Second, buffer)
    if err != nil && err.Error() != "EOF" {
        fmt.Printf("Data exchange error: %v\n", err)
    }
    
    // Print statistics
    fmt.Printf("Conn1 - RX: %d bytes, TX: %d bytes\n", statConn1.GetRX(), statConn1.GetTX())
    fmt.Printf("Conn2 - RX: %d bytes, TX: %d bytes\n", statConn2.GetRX(), statConn2.GetTX())
}
```

### DataExchange Function

The `DataExchange` function supports customizable buffer sizes for optimized performance:

```go
func DataExchange(conn1, conn2 net.Conn, idleTimeout time.Duration, buffer []byte) error
```

**Parameters:**
- `conn1`, `conn2`: The two connections to exchange data between
- `idleTimeout`: Maximum idle time before timeout (use `0` for no timeout)
- `buffer`: Custom buffer for data copying (pass `nil` to use default buffer)

**Buffer Configuration:**
- **Default buffer (`nil`)**: Uses Go's internal default buffer size (typically 32KB)
- **Custom buffer**: Allows optimization for specific use cases:
  - Larger buffers (64KB+) for high-throughput connections
  - Smaller buffers for memory-constrained environments
  - The same buffer is shared between both directions for memory efficiency

**Example buffer configurations:**

```go
// Use default buffer
err := conn.DataExchange(conn1, conn2, 30*time.Second, nil)

// High-throughput scenario
largeBuffer := make([]byte, 128*1024) // 128KB
err := conn.DataExchange(conn1, conn2, 30*time.Second, largeBuffer)

// Memory-constrained scenario  
smallBuffer := make([]byte, 8*1024) // 8KB
err := conn.DataExchange(conn1, conn2, 30*time.Second, smallBuffer)

// No timeout with custom buffer
err := conn.DataExchange(conn1, conn2, 0, make([]byte, 32*1024))
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

`StatConn` is a wrapper for `net.Conn` that tracks connection statistics (received and transmitted bytes) and supports optional rate limiting. It implements the `net.Conn` interface and can be used as a drop-in replacement. It also provides protocol-specific methods for TCP and UDP connections.

```go
import (
    "sync/atomic"
    "github.com/NodePassProject/conn"
)

// Basic usage without rate limiting
var rxBytes, txBytes uint64
statConn := conn.NewStatConn(tcpConn, &rxBytes, &txBytes, nil)

// Usage with rate limiting (1MB/s read, 512KB/s write)
rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)
statConnWithLimit := conn.NewStatConn(tcpConn, &rxBytes, &txBytes, rateLimiter)

// Use statConn like a normal net.Conn
// The rxBytes and txBytes variables will be updated automatically
// Rate limiting is applied automatically if Rate is set
n, err := statConnWithLimit.Write(data)
fmt.Printf("Total bytes sent: %d\n", statConn.GetTX())
fmt.Printf("Total bytes received: %d\n", statConn.GetRX())
```

#### TCP-Specific Methods

When the underlying connection is a TCP connection, you can use these specialized methods:

```go
// Check if it's a TCP connection
if statConn.IsTCP() {
    // Configure TCP-specific options
    err := statConn.SetKeepAlive(true)
    err = statConn.SetKeepAlivePeriod(30 * time.Second)
    err = statConn.SetNoDelay(true)  // Disable Nagle algorithm
    err = statConn.SetLinger(10)     // Set linger timeout
    
    // Graceful shutdown
    err = statConn.CloseWrite()  // Close write end
    err = statConn.CloseRead()   // Close read end
}

// Safe type conversion
if tcpConn, ok := statConn.AsTCPConn(); ok {
    // Direct access to *net.TCPConn if needed
    _ = tcpConn
}
```

#### UDP-Specific Methods

When the underlying connection is a UDP connection, you can use these specialized methods:

```go
// Check if it's a UDP connection
if statConn.IsUDP() {
    // UDP-specific read/write with address information
    buffer := make([]byte, 1024)
    n, addr, err := statConn.ReadFromUDP(buffer)
    
    // Send to specific address
    n, err = statConn.WriteToUDP(data, remoteAddr)
    
    // Configure UDP buffer sizes
    err = statConn.SetReadBuffer(65536)
    err = statConn.SetWriteBuffer(65536)
    
    // Advanced UDP operations with out-of-band data
    oob := make([]byte, 256)
    n, oobn, flags, addr, err := statConn.ReadMsgUDP(buffer, oob)
    n, oobn, err = statConn.WriteMsgUDP(data, oob, remoteAddr)
}

// Safe type conversion
if udpConn, ok := statConn.AsUDPConn(); ok {
    // Direct access to *net.UDPConn if needed
    _ = udpConn
}
```

#### Connection Type Detection

```go
// Check connection type
fmt.Printf("Network type: %s\n", statConn.NetworkType()) // "tcp", "udp", or "unknown"

// Type-specific checks
if statConn.IsTCP() {
    fmt.Println("This is a TCP connection")
}
if statConn.IsUDP() {
    fmt.Println("This is a UDP connection")
}
```

**StatConn Features:**
- Automatic statistics tracking for all read/write operations
- Optional rate limiting integration
- Protocol-specific method access with type safety
- Safe type conversion methods
- Connection type detection utilities
- All UDP methods include automatic statistics and rate limiting

### RateLimiter

`RateLimiter` implements a token bucket algorithm for bandwidth control. It supports separate rate limiting for read and write operations:

```go
import "github.com/NodePassProject/conn"

// Create a rate limiter with 1MB/s read and 512KB/s write limits
rateLimiter := conn.NewRateLimiter(1024*1024, 512*1024)

// Use with StatConn for automatic rate limiting
var rxBytes, txBytes uint64
statConn := conn.NewStatConn(tcpConn, &rxBytes, &txBytes, rateLimiter)

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

// Dynamic rate adjustment
rateLimiter.SetRate(2*1024*1024, 1024*1024) // Change to 2MB/s read, 1MB/s write

// Reset rate limiter state
rateLimiter.Reset() // Clear all tokens and restart
```

**Rate Limiter Features:**
- Token bucket algorithm for smooth traffic shaping
- Separate read and write rate controls
- Thread-safe implementation using atomic operations
- Zero value means unlimited rate (set to 0 or negative values)
- Automatic token refill based on configured rates
- Dynamic rate adjustment with `SetRate()` method
- State reset capability with `Reset()` method

## Complete Examples

### TCP Proxy with Statistics and Rate Limiting

```go
package main

import (
    "fmt"
    "log"
    "net"
    "time"
    "github.com/NodePassProject/conn"
)

func handleConnection(clientConn net.Conn) {
    defer clientConn.Close()
    
    // Connect to target server
    serverConn, err := net.Dial("tcp", "target-server.com:80")
    if err != nil {
        log.Printf("Failed to connect to server: %v", err)
        return
    }
    defer serverConn.Close()
    
    // Create rate limiter (10MB/s read, 5MB/s write)
    rateLimiter := conn.NewRateLimiter(10*1024*1024, 5*1024*1024)
    
    // Wrap connections with StatConn
    var clientRX, clientTX, serverRX, serverTX uint64
    statClient := conn.NewStatConn(clientConn, &clientRX, &clientTX, rateLimiter)
    statServer := conn.NewStatConn(serverConn, &serverRX, &serverTX, rateLimiter)
    
    // Configure TCP options for better performance
    if statClient.IsTCP() {
        statClient.SetKeepAlive(true)
        statClient.SetKeepAlivePeriod(30 * time.Second)
        statClient.SetNoDelay(true)
    }
    if statServer.IsTCP() {
        statServer.SetKeepAlive(true)
        statServer.SetKeepAlivePeriod(30 * time.Second)
        statServer.SetNoDelay(true)
    }
    
    // Start data exchange with 60-second idle timeout and custom buffer
    start := time.Now()
    buffer := make([]byte, 32*1024) // 32KB buffer for better performance
    err = conn.DataExchange(statClient, statServer, 60*time.Second, buffer)
    duration := time.Since(start)
    
    // Log statistics
    totalBytes := statClient.GetTotal() + statServer.GetTotal()
    avgSpeed := float64(totalBytes) / duration.Seconds() / 1024 / 1024 // MB/s
    
    log.Printf("Connection closed - Duration: %v, Total: %d bytes, Avg Speed: %.2f MB/s",
        duration, totalBytes, avgSpeed)
    
    if err != nil && err.Error() != "EOF" {
        log.Printf("Data exchange error: %v", err)
    }
}

func main() {
    listener, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }
    defer listener.Close()
    
    log.Println("TCP proxy listening on :8080")
    
    for {
        clientConn, err := listener.Accept()
        if err != nil {
            log.Printf("Failed to accept connection: %v", err)
            continue
        }
        
        go handleConnection(clientConn)
    }
}
```

### UDP Echo Server with Buffer Management

```go
package main

import (
    "fmt"
    "log"
    "net"
    "github.com/NodePassProject/conn"
)

func main() {
    // Listen on UDP port
    udpAddr, err := net.ResolveUDPAddr("udp", ":8081")
    if err != nil {
        log.Fatalf("Failed to resolve UDP address: %v", err)
    }
    
    udpConn, err := net.ListenUDP("udp", udpAddr)
    if err != nil {
        log.Fatalf("Failed to listen on UDP: %v", err)
    }
    defer udpConn.Close()
    
    // Create rate limiter for UDP (1MB/s each direction)
    rateLimiter := conn.NewRateLimiter(1024*1024, 1024*1024)
    
    // Wrap with StatConn
    var rxBytes, txBytes uint64
    statConn := conn.NewStatConn(udpConn, &rxBytes, &txBytes, rateLimiter)
    
    // Configure UDP buffer sizes for better performance
    if statConn.IsUDP() {
        statConn.SetReadBuffer(65536)
        statConn.SetWriteBuffer(65536)
    }
    
    log.Println("UDP echo server listening on :8081")
    
    buffer := make([]byte, 1024)
    for {
        // Read from UDP with automatic statistics and rate limiting
        n, clientAddr, err := statConn.ReadFromUDP(buffer)
        if err != nil {
            log.Printf("Failed to read from UDP: %v", err)
            continue
        }
        
        message := string(buffer[:n])
        log.Printf("Received from %v: %s", clientAddr, message)
        
        // Echo back to client with automatic statistics and rate limiting
        response := fmt.Sprintf("Echo: %s", message)
        _, err = statConn.WriteToUDP([]byte(response), clientAddr)
        if err != nil {
            log.Printf("Failed to write to UDP: %v", err)
            continue
        }
        
        // Print statistics periodically
        if statConn.GetRX()%10240 == 0 { // Every ~10KB
            log.Printf("Stats - RX: %d bytes, TX: %d bytes, Total: %d bytes",
                statConn.GetRX(), statConn.GetTX(), statConn.GetTotal())
        }
    }
}
```

## License

Copyright (c) 2025, NodePassProject. Licensed under the BSD 3-Clause License.
See the [LICENSE](LICENSE) file for details.