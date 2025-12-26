package transport

import (
	"context"
	"testing"
	"time"

	"github.com/quic-go/quic-go"

	tt "go-vnet/common/tls"
)

func TestServer_Serve(t *testing.T) {
	tlsConfig := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tlsConfig.InsecureSkipVerify = true
	quicConfig := &quic.Config{
		EnableDatagrams: true,
		KeepAlivePeriod: time.Second * 3,
	}
	ts := NewTransportServer(nil, NewTransportServerConfig(WithQuicConfig(quicConfig), WithTLSConfig(tlsConfig)))
	if err := ts.Serve(context.Background()); err != nil {
		t.Fatal(err)
		return
	}
}
