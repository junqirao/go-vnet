package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/quic-go/quic-go"
)

const (
	TypeQuic Type = "quic"
)

type (
	SendReceiveCloser interface {
		Closer
		Send(data []byte) (err error)
		Receive(ctx context.Context) (data []byte, err error)
	}
	Closer interface {
		CloseWithError(err error)
	}
	Type    string
	Session struct {
		Ctx              context.Context `json:"-"`
		IP               string          `json:"ip"`
		Type             Type            `json:"type"`
		SessionId        string          `json:"session_id"`
		NetworkId        string          `json:"network_id"`
		NetworkInfo      map[string]any  `json:"network_info"`
		DispatchedDevice Device          `json:"dispatched_device"`
		Meta             map[string]any  `json:"meta"`
	}
	ConnectionInfo struct {
		FromIp   string `json:"from_ip"`
		FromPort int    `json:"from_port"`
	}
	Error struct {
		msg   string
		code  int
		cause error
	}
	Device struct {
		Id   string `json:"id"`
		Name string `json:"name"`
		CIDR string `json:"cidr"`
		MTU  int    `json:"mtu"`
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

func (t Type) String() string {
	return string(t)
}

func NewError(msg string, code int) *Error {
	return &Error{
		msg:  msg,
		code: code,
	}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d]%s: %s", e.code, e.msg, e.cause.Error())
	}
	return fmt.Sprintf("[%d]%s", e.code, e.msg)
}

func (e *Error) Code() int {
	return e.code
}

func (e *Error) WithCause(cause error) *Error {
	ee := e.Clone()
	ee.cause = cause
	return ee
}

func (e *Error) Clone() *Error {
	return &Error{
		msg:   e.msg,
		code:  e.code,
		cause: e.cause,
	}
}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}

func (q quicSendReceiver) CloseWithError(err error) {
	desc := "connection closed"
	code := 0
	if err != nil {
		desc = err.Error()
		var ee *Error
		if errors.As(err, &ee) {
			code = ee.Code()
		}
	}
	_ = q.Conn.CloseWithError(quic.ApplicationErrorCode(code), desc)
}

func SendReceiverFromQuicConn(conn *quic.Conn) SendReceiveCloser {
	return &quicSendReceiver{conn}
}
