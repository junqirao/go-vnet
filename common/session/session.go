package session

import (
	"context"
	"fmt"
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
		Ctx              context.Context `json:"-"`
		IP               string          `json:"ip"`
		Type             string          `json:"type"`
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
		Id               string `json:"id"`                 // device id
		Sid              int    `json:"sid"`                // device sub id
		Name             string `json:"name"`               // name
		CIDR             string `json:"cidr"`               // cidr
		MTU              int    `json:"mtu"`                // mtu
		Key              string `json:"key,omitempty"`      // network encrypt key
		DataTrafficQuota *Quota `json:"data_traffic_quota"` // data traffic quota
		DataTrafficUsed  int64  `json:"data_traffic_used"`  // data traffic used
		BandwidthQuota   *Quota `json:"bandwidth_quota"`    // bandwidth quota
	}
	Quota struct {
		Id     int    `json:"id"`
		Unit   string `json:"unit"`
		Value  int64  `json:"value"`
		Period string `json:"period"`
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

func (d *Device) Clone() *Device {
	return &Device{
		Id:               d.Id,
		Sid:              d.Sid,
		Name:             d.Name,
		CIDR:             d.CIDR,
		MTU:              d.MTU,
		Key:              d.Key,
		DataTrafficQuota: d.DataTrafficQuota,
		DataTrafficUsed:  d.DataTrafficUsed,
		BandwidthQuota:   d.BandwidthQuota,
	}
}
