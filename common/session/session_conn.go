package session

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
)

const (
	msgTypeData byte = iota
	msgTypeError
)

type (
	connSendReceiver struct {
		mu sync.RWMutex
		net.Conn
	}
)

func SendReceiverFromNetConn(conn net.Conn) SendReceiveCloser {
	return &connSendReceiver{
		mu:   sync.RWMutex{},
		Conn: conn,
	}
}

func (c *connSendReceiver) CloseWithError(err error) {
	if err != nil {
		// 构建错误消息
		msg := err.Error()
		code := 500

		// 如果是 session.Error 类型，提取原始信息
		var sErr *Error
		if errors.As(err, &sErr) {
			msg = sErr.msg
			code = sErr.Code()
		}

		errData, _ := json.Marshal(map[string]interface{}{
			"msg":  msg,
			"code": code,
		})

		// 发送错误类型消息
		c.mu.Lock()
		defer c.mu.Unlock()

		// 写入消息类型
		if _, werr := c.Conn.Write([]byte{msgTypeError}); werr != nil {
			_ = c.Close()
			return
		}

		// 写入长度前缀
		lengthBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(errData)))
		if _, werr := c.Conn.Write(lengthBuf); werr != nil {
			_ = c.Close()
			return
		}

		// 写入错误数据
		_, _ = c.Conn.Write(errData)
	}
	_ = c.Close()

}

func (c *connSendReceiver) Send(data []byte) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 写入消息类型
	if _, err = c.Conn.Write([]byte{msgTypeData}); err != nil {
		return
	}

	// 写入4字节长度前缀
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))

	if _, err = c.Conn.Write(lengthBuf); err != nil {
		return
	}

	// 写入实际数据
	_, err = c.Conn.Write(data)
	return
}

func (c *connSendReceiver) Receive(_ context.Context) (data []byte, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 读取消息类型
	msgTypeBuf := make([]byte, 1)
	if _, err = io.ReadFull(c.Conn, msgTypeBuf); err != nil {
		return
	}

	// 读取4字节长度前缀
	lengthBuf := make([]byte, 4)
	if _, err = io.ReadFull(c.Conn, lengthBuf); err != nil {
		return
	}

	dataLen := binary.BigEndian.Uint32(lengthBuf)
	if dataLen == 0 {
		// 读取一个字节来解除发送端的阻塞（即使发送0字节也会阻塞）
		// 发送端会发送0字节，我们需要读取这0字节来解除阻塞
		_, err = io.ReadFull(c.Conn, make([]byte, 1)[:0])
		return
	}

	// 读取实际数据
	data = make([]byte, dataLen)
	if _, err = io.ReadFull(c.Conn, data); err != nil {
		return
	}

	// 如果是错误消息，转换为 error
	if msgTypeBuf[0] == msgTypeError {
		var errData struct {
			Msg  string `json:"msg"`
			Code int    `json:"code"`
		}
		if json.Unmarshal(data, &errData) == nil {
			err = NewError(errData.Msg, errData.Code)
		}
		data = nil // 错误消息不返回数据
	}

	return
}
