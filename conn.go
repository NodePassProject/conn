package conn

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNotTCPConn = errors.New("not a TCP connection")
	ErrNotUDPConn = errors.New("not a UDP connection")
)

type RateLimiter struct {
	readRate, writeRate     int64
	readTokens, writeTokens int64
	lastUpdate              int64
	condition               *sync.Cond
}

func NewRateLimiter(readBytesPerSecond, writeBytesPerSecond int64) *RateLimiter {
	if readBytesPerSecond <= 0 && writeBytesPerSecond <= 0 {
		return nil
	}

	if readBytesPerSecond <= 0 {
		readBytesPerSecond = 1 << 40
	}
	if writeBytesPerSecond <= 0 {
		writeBytesPerSecond = 1 << 40
	}

	rl := &RateLimiter{
		readRate:  readBytesPerSecond,
		writeRate: writeBytesPerSecond,
		condition: sync.NewCond(&sync.Mutex{}),
	}

	atomic.StoreInt64(&rl.readTokens, readBytesPerSecond)
	atomic.StoreInt64(&rl.writeTokens, writeBytesPerSecond)
	atomic.StoreInt64(&rl.lastUpdate, time.Now().UnixNano())

	return rl
}

func (rl *RateLimiter) WaitRead(bytes int64) {
	if rl == nil || bytes <= 0 {
		return
	}
	rl.waitTokens(bytes, &rl.readTokens)
}

func (rl *RateLimiter) WaitWrite(bytes int64) {
	if rl == nil || bytes <= 0 {
		return
	}
	rl.waitTokens(bytes, &rl.writeTokens)
}

func (rl *RateLimiter) SetRate(readBytesPerSecond, writeBytesPerSecond int64) {
	if rl == nil {
		return
	}

	if readBytesPerSecond <= 0 {
		readBytesPerSecond = 1 << 40
	}
	if writeBytesPerSecond <= 0 {
		writeBytesPerSecond = 1 << 40
	}

	rl.condition.L.Lock()
	defer rl.condition.L.Unlock()

	atomic.StoreInt64(&rl.readRate, readBytesPerSecond)
	atomic.StoreInt64(&rl.writeRate, writeBytesPerSecond)

	atomic.StoreInt64(&rl.readTokens, readBytesPerSecond)
	atomic.StoreInt64(&rl.writeTokens, writeBytesPerSecond)

	rl.condition.Broadcast()
}

func (rl *RateLimiter) Reset() {
	if rl == nil {
		return
	}

	rl.condition.L.Lock()
	defer rl.condition.L.Unlock()

	readRate := atomic.LoadInt64(&rl.readRate)
	writeRate := atomic.LoadInt64(&rl.writeRate)

	atomic.StoreInt64(&rl.readTokens, readRate)
	atomic.StoreInt64(&rl.writeTokens, writeRate)
	atomic.StoreInt64(&rl.lastUpdate, time.Now().UnixNano())

	rl.condition.Broadcast()
}

func (rl *RateLimiter) waitTokens(bytes int64, tokens *int64) {
	if rl == nil || bytes <= 0 {
		return
	}

	rl.condition.L.Lock()
	defer rl.condition.L.Unlock()
	for {
		rl.refillTokens()

		if curr := atomic.LoadInt64(tokens); curr >= bytes &&
			atomic.CompareAndSwapInt64(tokens, curr, curr-bytes) {
			return
		}
		rl.condition.Wait()
	}
}

func (rl *RateLimiter) refillTokens() {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&rl.lastUpdate)

	if elapsed := now - last; elapsed > 0 && atomic.CompareAndSwapInt64(&rl.lastUpdate, last, now) {
		elapsedSeconds := float64(elapsed) / float64(time.Second)
		readAdd := int64(float64(rl.readRate) * elapsedSeconds)
		writeAdd := int64(float64(rl.writeRate) * elapsedSeconds)

		rl.addTokens(&rl.readTokens, readAdd, rl.readRate)
		rl.addTokens(&rl.writeTokens, writeAdd, rl.writeRate)

		rl.condition.Broadcast()
	}
}

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

type StatConn struct {
	Conn net.Conn
	RX   *uint64
	TX   *uint64
	Rate *RateLimiter
}

func NewStatConn(conn net.Conn, rx, tx *uint64, rate *RateLimiter) *StatConn {
	return &StatConn{
		Conn: conn,
		RX:   rx,
		TX:   tx,
		Rate: rate,
	}
}

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

func (sc *StatConn) Close() error {
	return sc.Conn.Close()
}

func (sc *StatConn) LocalAddr() net.Addr {
	return sc.Conn.LocalAddr()
}

func (sc *StatConn) RemoteAddr() net.Addr {
	return sc.Conn.RemoteAddr()
}

func (sc *StatConn) SetDeadline(t time.Time) error {
	return sc.Conn.SetDeadline(t)
}

func (sc *StatConn) SetReadDeadline(t time.Time) error {
	return sc.Conn.SetReadDeadline(t)
}

func (sc *StatConn) SetWriteDeadline(t time.Time) error {
	return sc.Conn.SetWriteDeadline(t)
}

func (sc *StatConn) GetConn() net.Conn {
	return sc.Conn
}

func (sc *StatConn) GetRate() *RateLimiter {
	return sc.Rate
}

func (sc *StatConn) GetRX() uint64 {
	return atomic.LoadUint64(sc.RX)
}

func (sc *StatConn) GetTX() uint64 {
	return atomic.LoadUint64(sc.TX)
}

func (sc *StatConn) GetTotal() uint64 {
	return sc.GetRX() + sc.GetTX()
}

func (sc *StatConn) Reset() {
	atomic.StoreUint64(sc.RX, 0)
	atomic.StoreUint64(sc.TX, 0)
}

func (sc *StatConn) AsTCPConn() (*net.TCPConn, bool) {
	if tcpConn, ok := sc.Conn.(*net.TCPConn); ok {
		return tcpConn, true
	}
	return nil, false
}

func (sc *StatConn) AsUDPConn() (*net.UDPConn, bool) {
	if udpConn, ok := sc.Conn.(*net.UDPConn); ok {
		return udpConn, true
	}
	return nil, false
}

func (sc *StatConn) SetKeepAlive(keepalive bool) error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.SetKeepAlive(keepalive)
	}
	return &net.OpError{Op: "set", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) SetKeepAlivePeriod(d time.Duration) error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.SetKeepAlivePeriod(d)
	}
	return &net.OpError{Op: "set", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) SetNoDelay(noDelay bool) error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.SetNoDelay(noDelay)
	}
	return &net.OpError{Op: "set", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) SetLinger(sec int) error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.SetLinger(sec)
	}
	return &net.OpError{Op: "set", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) CloseRead() error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.CloseRead()
	}
	return &net.OpError{Op: "close", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) CloseWrite() error {
	if tcpConn, ok := sc.AsTCPConn(); ok {
		return tcpConn.CloseWrite()
	}
	return &net.OpError{Op: "close", Net: "tcp", Err: ErrNotTCPConn}
}

func (sc *StatConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	if udpConn, ok := sc.AsUDPConn(); ok {
		n, addr, err := udpConn.ReadFromUDP(b)
		if n > 0 {
			atomic.AddUint64(sc.RX, uint64(n))
			if sc.Rate != nil {
				sc.Rate.WaitRead(int64(n))
			}
		}
		return n, addr, err
	}
	return 0, nil, &net.OpError{Op: "read", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	if udpConn, ok := sc.AsUDPConn(); ok {
		if sc.Rate != nil {
			sc.Rate.WaitWrite(int64(len(b)))
		}
		n, err := udpConn.WriteToUDP(b, addr)
		if n > 0 {
			atomic.AddUint64(sc.TX, uint64(n))
		}
		return n, err
	}
	return 0, &net.OpError{Op: "write", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) ReadMsgUDP(b, oob []byte) (n, oobn, flags int, addr *net.UDPAddr, err error) {
	if udpConn, ok := sc.AsUDPConn(); ok {
		n, oobn, flags, addr, err = udpConn.ReadMsgUDP(b, oob)
		if n > 0 {
			atomic.AddUint64(sc.RX, uint64(n))
			if sc.Rate != nil {
				sc.Rate.WaitRead(int64(n))
			}
		}
		return n, oobn, flags, addr, err
	}
	return 0, 0, 0, nil, &net.OpError{Op: "read", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) WriteMsgUDP(b, oob []byte, addr *net.UDPAddr) (n, oobn int, err error) {
	if udpConn, ok := sc.AsUDPConn(); ok {
		if sc.Rate != nil {
			sc.Rate.WaitWrite(int64(len(b)))
		}
		n, oobn, err = udpConn.WriteMsgUDP(b, oob, addr)
		if n > 0 {
			atomic.AddUint64(sc.TX, uint64(n))
		}
		return n, oobn, err
	}
	return 0, 0, &net.OpError{Op: "write", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) SetReadBuffer(bytes int) error {
	if udpConn, ok := sc.AsUDPConn(); ok {
		return udpConn.SetReadBuffer(bytes)
	}
	return &net.OpError{Op: "set", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) SetWriteBuffer(bytes int) error {
	if udpConn, ok := sc.AsUDPConn(); ok {
		return udpConn.SetWriteBuffer(bytes)
	}
	return &net.OpError{Op: "set", Net: "udp", Err: ErrNotUDPConn}
}

func (sc *StatConn) IsTCP() bool {
	_, ok := sc.AsTCPConn()
	return ok
}

func (sc *StatConn) IsUDP() bool {
	_, ok := sc.AsUDPConn()
	return ok
}

func (sc *StatConn) NetworkType() string {
	if sc.IsTCP() {
		return "tcp"
	} else if sc.IsUDP() {
		return "udp"
	}
	return "unknown"
}

type TimeoutReader struct {
	Conn    net.Conn
	Timeout time.Duration
}

func (tr *TimeoutReader) Read(b []byte) (int, error) {
	if tr.Timeout > 0 {
		tr.Conn.SetReadDeadline(time.Now().Add(tr.Timeout))
	}
	return tr.Conn.Read(b)
}

func DataExchange(conn1, conn2 net.Conn, idleTimeout time.Duration, buffer1, buffer2 []byte) error {
	if conn1 == nil || conn2 == nil {
		return io.ErrUnexpectedEOF
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	copyData := func(src, dst net.Conn, buffer []byte) {
		defer wg.Done()

		reader := &TimeoutReader{Conn: src, Timeout: idleTimeout}
		_, err := io.CopyBuffer(dst, reader, buffer)
		errChan <- err

		if idleTimeout == 0 {
			if tcpConn, ok := dst.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			} else {
				dst.Close()
			}
		}
	}

	wg.Add(2)
	go copyData(conn1, conn2, buffer1)
	go copyData(conn2, conn1, buffer2)
	wg.Wait()
	close(errChan)

	if idleTimeout > 0 {
		conn1.SetReadDeadline(time.Time{})
		conn2.SetReadDeadline(time.Time{})
	}

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return io.EOF
}
