package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

type (
	QuicClient struct {
		*Client
		// quic
		tlsConfig  *tls.Config
		quicConfig *quic.Config
		session    *QuicSession
	}
	QuicSession struct {
		*Session
		conn *quic.Conn
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

func SendReceiverFromQuicConn(conn *quic.Conn) SendReceiver {
	return &quicSendReceiver{conn}
}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}

func NewQuicClient(c *Client) *QuicClient {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](c.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, ConfigKeyQuicConfig, &quic.Config{
		KeepAlivePeriod: time.Second * 3,
		EnableDatagrams: true,
	})
	if c.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	qc := &QuicClient{
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
	}

	qc.Client = c

	return qc
}

func (c *QuicClient) Dial(ctx context.Context) (session *Session, err error) {
	c.ctx = ctx
	addr := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	conn, err := quic.DialAddr(ctx, addr, c.tlsConfig, c.quicConfig)
	if err != nil {
		return
	}

	// get payload and overwrite network id
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	// do auth
	resp, err := c.auth.Auth(ctx, payload,
		func(ctx context.Context, in []byte) (out []byte, err error) {
			if err = conn.SendDatagram(in); err != nil {
				return
			}
			return conn.ReceiveDatagram(ctx)
		},
	)
	if err != nil {
		return
	}

	joined := JoinNetworkResponse{}
	bs, _ := json.Marshal(resp)
	_ = json.Unmarshal(bs, &joined)

	c.session = &QuicSession{
		Session: &Session{
			SendReceiver:     SendReceiverFromQuicConn(conn),
			DispatchedDevice: joined.Device,
		},
		conn: conn,
	}
	session = c.session.Session

	c.manager = NewManager(session)
	return
}

func (c *QuicClient) SendToServer(dst string, buf []byte, n int) (err error) {
	return
}
