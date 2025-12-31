package server

import (
	"context"
	"fmt"

	"go-vnet/server/network"

	"go-vnet/common/device"
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
	Session struct {
		SendReceiveCloser
		Conn             any              `json:"-"`
		Type             SessionType      `json:"type"`
		Id               string           `json:"id"`
		Network          *network.Network `json:"network"`
		DispatchedDevice *device.Device   `json:"dispatched_device"`
		IP               string           `json:"ip"`
	}
	SessionType = Type
	Error       struct {
		msg   string
		code  int
		cause error
	}
)

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
