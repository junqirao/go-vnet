package session

import (
	"context"
	"errors"

	"github.com/quic-go/quic-go"
)

type (
	quicSendReceiver struct {
		*quic.Conn
	}
)

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
