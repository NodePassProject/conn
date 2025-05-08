// Package conn 提供UDP和TCP连接之间的数据交换功能
package conn

import (
	"net"
	"time"
)

// DataTransfer 在UDP和TCP连接之间交换数据
func DataTransfer(
	udpConn *net.UDPConn,
	tcpConn net.Conn,
	udpAddr *net.UDPAddr,
	initialData []byte,
	bufferSize int,
	timeout time.Duration,
) (udpToTcp uint64, tcpToUdp uint64, err error) {
	buffer := make([]byte, bufferSize)

	// 通过判断udpAddr是否为空来确定方向
	if udpAddr != nil {
		// 将UDP数据发送到TCP
		n, err := tcpConn.Write(initialData)
		if err != nil {
			return 0, 0, err
		}
		udpToTcp = uint64(n)

		// 从TCP读取响应
		if err := tcpConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return udpToTcp, 0, err
		}

		n, err = tcpConn.Read(buffer)
		if err != nil {
			return udpToTcp, 0, err
		}

		// 将响应写回UDP
		n, err = udpConn.WriteToUDP(buffer[:n], udpAddr)
		if err != nil {
			return udpToTcp, 0, err
		}
		tcpToUdp = uint64(n)

	} else {
		// 从TCP读取请求数据
		n, err := tcpConn.Read(buffer)
		if err != nil {
			return 0, 0, err
		}
		tcpToUdp = uint64(n)

		// 将数据发送到UDP
		_, err = udpConn.Write(buffer[:n])
		if err != nil {
			return 0, tcpToUdp, err
		}

		// 设置UDP读取超时
		if err := udpConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return 0, tcpToUdp, err
		}

		// 从UDP读取响应
		n, err = udpConn.Read(buffer)
		if err != nil {
			return 0, tcpToUdp, err
		}

		// 将响应发送回TCP
		n, err = tcpConn.Write(buffer[:n])
		if err != nil {
			return 0, tcpToUdp, err
		}
		udpToTcp = uint64(n)
	}

	return udpToTcp, tcpToUdp, nil
}
