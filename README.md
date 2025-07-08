# Conn Package

[![Go Reference](https://pkg.go.dev/badge/github.com/NodePassProject/conn.svg)](https://pkg.go.dev/github.com/NodePassProject/conn)
[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

A flexible and efficient network connection exchange system for Go applications.

## Features

- Thread-safe data exchange between TCP connections
- Bidirectional data transfer with proper connection state management
- Support for UDP to TCP data transfer
- Automatic connection handling and cleanup
- Configurable buffer sizes and timeouts
- Efficient error handling and resource management

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
    "io"

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

    // Exchange data between the two connections with a 5-second idle timeout
    bytesAtoB, bytesBtoA, err := conn.DataExchange(conn1, conn2, 5*time.Second)
    if err != nil && err != io.EOF {
        fmt.Printf("Data exchange error: %v\n", err)
    }

    fmt.Printf("Transferred %d bytes from server1 to server2\n", bytesAtoB)
    fmt.Printf("Transferred %d bytes from server2 to server1\n", bytesBtoA)
}
```

### UDP to TCP Data Transfer

```go
package main

import (
    "fmt"
    "net"
    "time"
    
    "github.com/NodePassProject/conn"
)

func main() {
    // Create a UDP connection
    udpConn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 5000})
    if err != nil {
        fmt.Printf("Failed to create UDP socket: %v\n", err)
        return
    }
    defer udpConn.Close()
    
    // Create a TCP connection
    tcpConn, err := net.Dial("tcp", "example.com:8080")
    if err != nil {
        fmt.Printf("Failed to connect to TCP server: %v\n", err)
        return
    }
    defer tcpConn.Close()
    
    // UDP client address (for responses)
    udpAddr, _ := net.ResolveUDPAddr("udp", "client.example.com:6000")
    
    // Initial data received from UDP client
    initialData := []byte("Hello from UDP client")
    
    // Transfer data between UDP and TCP with a 5-second timeout
    udpToTcp, tcpToUdp, err := conn.DataTransfer(
        udpConn,
        tcpConn,
        udpAddr,
        initialData,
        4096,  // buffer size
        5*time.Second,  // timeout
    )
    
    if err != nil {
        fmt.Printf("Data transfer error: %v\n", err)
        return
    }
    
    fmt.Printf("Transferred %d bytes from UDP to TCP\n", udpToTcp)
    fmt.Printf("Transferred %d bytes from TCP to UDP\n", tcpToUdp)
}
```

## License

Copyright (c) 2025, NodePassProject. Licensed under the BSD 3-Clause License.
See the [LICENSE](LICENSE) file for details.