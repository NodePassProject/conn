# Conn Package

[![Go Reference](https://pkg.go.dev/badge/github.com/NodePassProject/conn.svg)](https://pkg.go.dev/github.com/NodePassProject/conn)
[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

A simple and efficient TCP connection data exchange utility for Go applications.

## Features

- Thread-safe, bidirectional data exchange between two TCP connections
- Idle timeout support for automatic connection cleanup
- Efficient error handling and resource management
- `TimeoutReader` for per-read timeout control

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

### TimeoutReader

`TimeoutReader` is a wrapper for `net.Conn` that allows you to set a read timeout for each read operation. It is used internally by `DataExchange`, but can also be used directly if needed:

```go
tr := &conn.TimeoutReader{Conn: tcpConn, timeout: 5 * time.Second}
buf := make([]byte, 4096)
n, err := tr.Read(buf)
```

## License

Copyright (c) 2025, NodePassProject. Licensed under the BSD 3-Clause License.
See the [LICENSE](LICENSE) file for details.