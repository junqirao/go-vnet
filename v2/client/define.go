package client

import (
	"io"

	"go-vnet/common/device"
)

type (
	JoinNetworkResponse struct {
		Device device.Config `json:"device"`
	}
	Session struct {
		SendReceiver
		io.Closer
		err              error
		DispatchedDevice device.Config `json:"dispatched_device"`
	}
)

func (s *Session) Error() error {
	return s.err
}

func (s *Session) SetError(err error) {
	s.err = err
}

func (s *Session) Close() error {
	if s.Closer == nil {
		return nil
	}
	return s.Closer.Close()
}
