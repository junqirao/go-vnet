package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/quic-go/quic-go"
)

type (
	quicP2PServer struct {
		*Server
		cfg      *TransportConfig
		location peer.AddrInfo
		listener transport.Listener
	}
)

func newQuicP2PServer(s *Server) internalServer {
	return &quicP2PServer{
		Server: s,
	}
}

func (s *quicP2PServer) Setup(ctx context.Context, cfg *TransportConfig) (err error) {
	s.cfg = cfg

	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", cfg.Address, cfg.Port))
	if err != nil {
		return err
	}
	priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		return err
	}
	peerID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return err
	}

	reuse, err := quicreuse.NewConnManager(quic.StatelessResetKey{}, quic.TokenGeneratorKey{})
	if err != nil {
		return err
	}
	t, err := libp2pquic.NewTransport(priv, reuse, nil, nil, nil)
	if err != nil {
		return err
	}

	s.listener, err = t.Listen(addr)
	if err != nil {
		return
	}

	s.location = peer.AddrInfo{ID: peerID, Addrs: []ma.Multiaddr{s.listener.Multiaddr()}}
	s.logger.Infof(ctx, "quic p2p server listen addr: %s, peer id: %s, multiaddr: %s",
		addr, peerID, s.listener.Multiaddr())
	return
}

func (s *quicP2PServer) Accept(ctx context.Context) (ss *serverSession, err error) {
	c, err := s.listener.Accept()
	if err != nil {
		return
	}
	// no need session.SendReceiveCloser
	ss = newServerSession(nil, c)
	return
}

func (s *quicP2PServer) Close() error {
	return s.listener.Close()
}

func (s *quicP2PServer) AcceptTransport(session *serverSession) (rwc io.ReadWriteCloser, err error) {
	conn := session.conn.(transport.CapableConn)
	rwc, err = conn.AcceptStream()
	return
}

func (s *quicP2PServer) GetDstTransportWriter(src *serverSession, dst *serverSession) (rwc io.ReadWriteCloser, err error) {
	// no need
	return
}
