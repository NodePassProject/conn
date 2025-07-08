// Package conn 提供两条TCP连接之间双向数据交换功能
package conn

import (
	"io"
	"net"
	"sync"
	"time"
)

// timeoutReader 包装了 net.Conn，支持设置读取超时
type timeoutReader struct {
	net.Conn
	timeout time.Duration
}

// Read 实现了 io.Reader 接口，读取数据时会设置读取超时
func (tr *timeoutReader) Read(b []byte) (int, error) {
	if tr.timeout > 0 {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.timeout))
	}
	return tr.Conn.Read(b)
}

// DataExchange 实现两个 net.Conn 之间的双向数据交换，支持空闲超时
func DataExchange(conn1, conn2 net.Conn, idleTimeout time.Duration) (int64, int64, error) {
	// 连接有效性检查
	if conn1 == nil || conn2 == nil {
		return 0, 0, io.ErrUnexpectedEOF
	}

	var (
		sum1, sum2 int64
		wg         sync.WaitGroup
		errChan    = make(chan error, 2)
	)

	// 定义一个函数用于双向数据传输
	copyData := func(src, dst net.Conn, sum *int64) {
		defer wg.Done()
		reader := &timeoutReader{Conn: src, timeout: idleTimeout}
		n, err := io.Copy(dst, reader)
		*sum = n
		errChan <- err
	}

	// 启动双向数据传输
	wg.Add(2)
	go copyData(conn1, conn2, &sum1)
	go copyData(conn2, conn1, &sum2)

	wg.Wait()
	close(errChan)

	// 清空超时设置
	if idleTimeout > 0 {
		conn1.SetReadDeadline(time.Time{})
		conn2.SetReadDeadline(time.Time{})
	}

	for err := range errChan {
		if err != nil {
			return sum1, sum2, err
		}
	}

	return sum1, sum2, io.EOF
}
