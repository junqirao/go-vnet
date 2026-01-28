package server

import (
	"context"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"

	"go-vnet/common/logger"
)

type (
	P2PRelayServer struct {
		ref    *Server
		logger logger.Logger
		config []*RelayConfig
		host   host.Host
		relay  *relay.Relay
	}
	RelayInfo struct {
		Id        string   `json:"id"`
		Addresses []string `json:"addresses"`
	}
	RelayHost struct {
		Id            string
		LastHeartbeat time.Time
	}
)

func newP2PRelayServer(config []*RelayConfig, ref *Server) *P2PRelayServer {
	return &P2PRelayServer{
		config: config,
		ref:    ref,
		logger: ref.logger,
	}
}

func (r *P2PRelayServer) Run(ctx context.Context) (err error) {
	if len(r.config) == 0 {
		return nil
	}
	var addr []multiaddr.Multiaddr
	for _, c := range r.config {
		addr = append(addr, c.MultiAddr())
	}
	r.host, err = libp2p.New(libp2p.ListenAddrs(addr...))
	if err != nil {
		return
	}
	r.relay, err = relay.New(r.host)
	if err != nil {
		return
	}
	r.logger.Infof(ctx, "p2p relay server started: %s", r.ID())
	r.logger.Infof(ctx, "p2p relay server addresses: \n%v",
		strings.Join(r.Addresses(), "\n"))
	return
}

func (r *P2PRelayServer) ID() string {
	if r.host == nil {
		return ""
	}
	return r.host.ID().String()
}

func (r *P2PRelayServer) Addresses() []string {
	if r.host == nil {
		return []string{}
	}
	res := make([]string, 0)
	for _, addr := range r.host.Addrs() {
		res = append(res, addr.String())
	}
	return res
}

func (r *P2PRelayServer) CLose() (err error) {
	if r.host != nil {
		_ = r.host.Close()
		r.host = nil
	}
	if r.relay != nil {
		_ = r.relay.Close()
		r.relay = nil
	}
	return
}
