// Package conn 提供两条TCP连接之间双向数据交换功能
package conn

import (
	"io"
	"net"
	"sync"
)

// DataExchange 在两个网络连接之间交换数据
func DataExchange(conn1, conn2 net.Conn) (int64, int64, error) {
	// 连接有效性检查
	if conn1 == nil || conn2 == nil {
		return 0, 0, io.ErrUnexpectedEOF
	}

	var (
		sum1, sum2 int64
		wg         sync.WaitGroup
		errChan    = make(chan error, 2)
	)

	// copyData 在两个连接之间复制数据并管理连接状态
	copyData := func(dst, src net.Conn, sum *int64) {
		defer wg.Done()

		// 执行数据复制
		n, err := io.Copy(dst, src)
		*sum = n
		errChan <- err

		// 根据错误类型处理连接
		if err == nil || err == io.EOF {
			// 正常关闭情况，如果目标支持半关闭，则关闭写入端
			if c, ok := dst.(interface{ CloseWrite() error }); ok {
				c.CloseWrite()
			}
		} else {
			// 发生错误时关闭源连接
			src.Close()
		}
	}

	// 启动双向数据传输
	wg.Add(2)
	go copyData(conn1, conn2, &sum1)
	go copyData(conn2, conn1, &sum2)

	// 等待所有传输完成
	wg.Wait()
	close(errChan)

	// 确保两个连接都关闭
	conn1.Close()
	conn2.Close()

	// 检查是否有错误发生
	for err := range errChan {
		if err != nil {
			return sum1, sum2, err
		}
	}

	return sum1, sum2, io.EOF
}
