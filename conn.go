// Package conn 提供两条TCP连接之间双向数据交换功能
package conn

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TimeoutReader 包装了 net.Conn，支持设置读取超时
type TimeoutReader struct {
	Conn    net.Conn
	Timeout time.Duration
}

// Read 实现了 io.Reader 接口，读取数据时会设置读取超时
func (tr *TimeoutReader) Read(b []byte) (int, error) {
	if tr.Timeout > 0 {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.Timeout))
	}
	return tr.Conn.Read(b)
}

// StatConn 是一个包装了 net.Conn 的结构体，用于统计读取和写入的字节数
type StatConn struct {
	Conn net.Conn
	RX   *uint64
	TX   *uint64
}

// Read 实现了 io.Reader 接口，读取数据时会统计读取字节数
func (sc *StatConn) Read(b []byte) (int, error) {
	n, err := sc.Conn.Read(b)
	atomic.AddUint64(sc.RX, uint64(n))
	return n, err
}

// Write 实现了 io.Writer 接口，写入数据时会统计写入字节数
func (sc *StatConn) Write(b []byte) (int, error) {
	n, err := sc.Conn.Write(b)
	atomic.AddUint64(sc.TX, uint64(n))
	return n, err
}

// Close 关闭连接
func (sc *StatConn) Close() error {
	return sc.Conn.Close()
}

// LocalAddr 返回本地地址
func (sc *StatConn) LocalAddr() net.Addr {
	return sc.Conn.LocalAddr()
}

// RemoteAddr 返回远程地址
func (sc *StatConn) RemoteAddr() net.Addr {
	return sc.Conn.RemoteAddr()
}

// SetDeadline 设置连接的读写超时
func (sc *StatConn) SetDeadline(t time.Time) error {
	return sc.Conn.SetDeadline(t)
}

// SetReadDeadline 设置连接的读取超时
func (sc *StatConn) SetReadDeadline(t time.Time) error {
	return sc.Conn.SetReadDeadline(t)
}

// SetWriteDeadline 设置连接的写入超时
func (sc *StatConn) SetWriteDeadline(t time.Time) error {
	return sc.Conn.SetWriteDeadline(t)
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
		reader := &TimeoutReader{Conn: src, Timeout: idleTimeout}
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
