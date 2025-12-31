package client

import (
	"go-vnet/common/device"
)

type (
	JoinNetworkResponse struct {
		Device device.Config `json:"device"`
	}
	Session struct {
		SendReceiveCloser
		err              error
		Type             SessionType   `json:"type"`
		NetworkId        string        `json:"network_id"`
		DispatchedDevice device.Config `json:"dispatched_device"`
		Conn             any           `json:"-"`
	}
	SessionType = Type
)

func (s *Session) Error() error {
	return s.err
}

func (s *Session) SetError(err error) {
	s.err = err
}

func (s *Session) Close() error {
	if s.SendReceiveCloser == nil {
		return nil
	}
	return s.SendReceiveCloser.Close()
}
