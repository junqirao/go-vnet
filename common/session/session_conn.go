package session

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
)

const (
	msgTypeData byte = iota
	msgTypeError
)

type (
	connSendReceiver struct {
		conn net.Conn
	}
)

func SendReceiverFromNetConn(conn net.Conn) SendReceiveCloser {
	return &connSendReceiver{
		conn: conn,
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

		_ = c.send(msgTypeError, errData)
	}
	_ = c.conn.Close()
}

func (c *connSendReceiver) send(typ uint8, data []byte) (err error) {
	buf := make([]byte, len(data)+5)
	buf[0] = typ
	binary.BigEndian.PutUint32(buf[1:], uint32(len(data)))
	copy(buf[5:], data)
	_, err = c.conn.Write(buf)
	return
}

func (c *connSendReceiver) Send(data []byte) (err error) {
	return c.send(msgTypeData, data)
}

func (c *connSendReceiver) Receive(_ context.Context) (data []byte, err error) {
	// 读取5字节长度前缀
	header := make([]byte, 5)
	if _, err = io.ReadFull(c.conn, header); err != nil {
		return
	}

	dataLen := binary.BigEndian.Uint32(header[1:5])
	if dataLen == 0 {
		return
	}

	// 读取实际数据
	data = make([]byte, dataLen)
	if _, err = io.ReadFull(c.conn, data); err != nil {
		return
	}

	// 如果是错误消息，转换为 error
	if header[0] == msgTypeError {
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
