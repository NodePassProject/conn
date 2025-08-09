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

// RateLimiter 全局令牌桶读写限速器
type RateLimiter struct {
	readRate, writeRate     int64 // 每秒读取/写入字节数
	readTokens, writeTokens int64 // 当前令牌数
	lastUpdate              int64 // 上次更新时间
}

// NewRateLimiter 创建新的全局令牌桶读写限速器
func NewRateLimiter(readBytesPerSecond, writeBytesPerSecond int64) *RateLimiter {
	if readBytesPerSecond <= 0 && writeBytesPerSecond <= 0 {
		return nil
	}

	// 如果某个速率为0，设置为无限制
	if readBytesPerSecond <= 0 {
		readBytesPerSecond = 1 << 40 // 1TB/s
	}
	if writeBytesPerSecond <= 0 {
		writeBytesPerSecond = 1 << 40 // 1TB/s
	}

	rl := &RateLimiter{
		readRate:  readBytesPerSecond,
		writeRate: writeBytesPerSecond,
	}

	// 使用原子操作初始化
	atomic.StoreInt64(&rl.readTokens, readBytesPerSecond)
	atomic.StoreInt64(&rl.writeTokens, writeBytesPerSecond)
	atomic.StoreInt64(&rl.lastUpdate, time.Now().UnixNano())

	return rl
}

// WaitRead 等待读取令牌
func (rl *RateLimiter) WaitRead(bytes int64) {
	if rl == nil || bytes <= 0 {
		return
	}
	rl.waitTokens(bytes, &rl.readTokens)
}

// WaitWrite 等待写入令牌
func (rl *RateLimiter) WaitWrite(bytes int64) {
	if rl == nil || bytes <= 0 {
		return
	}
	rl.waitTokens(bytes, &rl.writeTokens)
}

// waitTokens 等待令牌
func (rl *RateLimiter) waitTokens(bytes int64, tokens *int64) {
	if rl == nil || bytes <= 0 {
		return
	}

	for {
		rl.refillTokens()

		// 原子获取并消耗令牌
		if curr := atomic.LoadInt64(tokens); curr >= bytes &&
			atomic.CompareAndSwapInt64(tokens, curr, curr-bytes) {
			return
		}

		time.Sleep(time.Millisecond)
	}
}

// refillTokens 更新令牌
func (rl *RateLimiter) refillTokens() {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&rl.lastUpdate)

	if elapsed := now - last; elapsed > 0 && atomic.CompareAndSwapInt64(&rl.lastUpdate, last, now) {
		// 更新读写令牌
		elapsedSeconds := float64(elapsed) / float64(time.Second)
		readAdd := int64(float64(rl.readRate) * elapsedSeconds)
		writeAdd := int64(float64(rl.writeRate) * elapsedSeconds)

		rl.addTokens(&rl.readTokens, readAdd, rl.readRate)
		rl.addTokens(&rl.writeTokens, writeAdd, rl.writeRate)
	}
}

// addTokens 添加令牌
func (rl *RateLimiter) addTokens(tokens *int64, add, max int64) {
	if add <= 0 {
		return
	}
	for {
		curr := atomic.LoadInt64(tokens)
		newVal := min(curr+add, max)
		if atomic.CompareAndSwapInt64(tokens, curr, newVal) {
			break
		}
	}
}

// StatConn 是一个包装了 net.Conn 的结构体，用于统计并限制读取和写入的字节数
type StatConn struct {
	Conn net.Conn
	RX   *uint64
	TX   *uint64
	Rate *RateLimiter
}

// Read 实现了 io.Reader 接口，读取数据时会统计读取字节数并进行限速
func (sc *StatConn) Read(b []byte) (int, error) {
	n, err := sc.Conn.Read(b)
	if n > 0 {
		atomic.AddUint64(sc.RX, uint64(n))
		if sc.Rate != nil {
			sc.Rate.WaitRead(int64(n))
		}
	}
	return n, err
}

// Write 实现了 io.Writer 接口，写入数据时会统计写入字节数并进行限速
func (sc *StatConn) Write(b []byte) (int, error) {
	if sc.Rate != nil {
		sc.Rate.WaitWrite(int64(len(b)))
	}
	n, err := sc.Conn.Write(b)
	if n > 0 {
		atomic.AddUint64(sc.TX, uint64(n))
	}
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
