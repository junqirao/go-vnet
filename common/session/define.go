package session

import (
	"context"
	"errors"
	"net"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"

	"go-vnet/common/device"
	"go-vnet/server/network"
)

const (
	TypeQuic Type = "quic"
)

type (
	Session struct {
		SendReceiveCloser `json:"-"`
		Conn              any            `json:"-"`
		Id                string         `json:"id"`
		Type              Type           `json:"type"`
		IP                string         `json:"ip"`
		DispatchedDevice  *device.Device `json:"dispatched_device"`
	}
	ServerSession struct {
		Session `json:"session"`
		Network *network.Network `json:"network"`
	}
	ClientSession struct {
		Session
		NetworkId string `json:"network_id"`
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

func (t Type) String() string {
	return string(t)
}

func NewSession(typ Type, conn any, dispatchedDevice *device.Device, sr SendReceiveCloser) Session {
	ip, _, _ := net.ParseCIDR(dispatchedDevice.CIDR)
	return Session{
		SendReceiveCloser: sr,
		Type:              typ,
		Id:                uuid.NewString(),
		IP:                ip.To4().String(),
		Conn:              conn,
		DispatchedDevice:  dispatchedDevice,
	}
}

func NewServerSession(s Session, nwk *network.Network) *ServerSession {
	return &ServerSession{
		Session: s,
		Network: nwk,
	}
}

func NewClientSession(s Session, networkId string) *ClientSession {
	return &ClientSession{
		Session:   s,
		NetworkId: networkId,
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
