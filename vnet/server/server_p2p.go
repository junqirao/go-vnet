package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"

	"go-vnet/common/logger"
)

type (
	SignalingServerAddress struct {
		IP        string `json:"ip"`
		Transport string `json:"transport"`
		Port      int    `json:"port"`
		Version   string `json:"version"` // only for quic
	}
	p2pSignalingServer struct {
		ctx       context.Context
		config    *P2PConfig
		ref       *Server
		logger    logger.Logger
		host      host.Host
		addresses []string
		hostId    string
	}
)

func (c SignalingServerAddress) MultiAddr(network ...string) multiaddr.Multiaddr {
	n := "ip4"
	if network != nil {
		n = network[0]
	}
	str := fmt.Sprintf("/%s/%s/%s/%d", n, c.IP, c.Transport, c.Port)
	if c.Version != "" {
		str += "/" + c.Version
	}
	return multiaddr.StringCast(str)
}

func newP2PSignalingServer(config *P2PConfig, ref *Server) *p2pSignalingServer {
	return &p2pSignalingServer{
		config: config,
		ref:    ref,
		logger: ref.logger,
	}
}

func (s *p2pSignalingServer) Run(ctx context.Context) (err error) {
	var addresses []multiaddr.Multiaddr
	for _, address := range s.config.Addresses {
		addresses = append(addresses, address.MultiAddr())
	}
	if len(addresses) == 0 {
		err = fmt.Errorf("no address for p2p server")
		return
	}

	s.ctx = ctx
	s.host, err = libp2p.New(
		libp2p.ListenAddrs(addresses...),
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		err = fmt.Errorf("failed to create host: %v", err)
		return
	}

	s.addresses = make([]string, 0)
	for _, addr := range s.host.Addrs() {
		s.addresses = append(s.addresses, addr.String())
	}
	s.hostId = s.host.ID().String()

	s.logger.Infof(ctx, "p2p signaling server started: %s", s.ID())
	s.logger.Infof(ctx, "p2p signaling server addresses: \n%v",
		strings.Join(s.Addresses(), "\n"))
	return
}

func (s *p2pSignalingServer) Close() (err error) {
	s.logger.Infof(s.ctx, "p2p signaling server stopped")
	return s.host.Close()
}

func (s *p2pSignalingServer) ID() string {
	return s.hostId
}

func (s *p2pSignalingServer) Addresses() []string {
	return s.addresses
}
