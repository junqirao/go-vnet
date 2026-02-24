package server

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"

	"go-vnet/common/router"
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
		host      host.Host
		addresses []string              // 对外宣布的公网地址
		announce  []multiaddr.Multiaddr // 配置的公网地址
		hostId    string
		router    *router.Router
		connMgr   *connmgr.BasicConnMgr
	}
	AddressInfo struct {
		Id        string   `json:"id"`
		Addresses []string `json:"addresses"`
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
		router: router.NewRouter(),
	}
}

func (s *p2pSignalingServer) Run(ctx context.Context) (err error) {
	if len(s.config.Addresses) == 0 {
		err = fmt.Errorf("no address for p2p server")
		return
	}

	// 准备对外宣布的地址列表（配置的公网地址）
	s.announce = make([]multiaddr.Multiaddr, 0, len(s.config.Addresses))
	for _, address := range s.config.Addresses {
		s.announce = append(s.announce, address.MultiAddr())
	}

	// 准备监听地址 - 监听所有接口，使用配置中的端口
	listenAdders := make([]multiaddr.Multiaddr, 0, len(s.config.Addresses))
	for _, address := range s.config.Addresses {
		// 监听 0.0.0.0（所有接口）而不是公网 IP
		listenAddr := fmt.Sprintf("/%s/0.0.0.0/%s/%d",
			map[bool]string{true: "ip6", false: "ip4"}[address.IP == "::"],
			address.Transport,
			address.Port)
		if address.Version != "" {
			listenAddr += "/" + address.Version
		}
		listenAdders = append(listenAdders, multiaddr.StringCast(listenAddr))
	}

	s.ctx = ctx
	// 配置连接管理器：管理连接的生命周期和 keepalive
	// 保持活跃连接：30秒无活动后触发 keepalive
	// 超时时间：5分钟无活动后关闭连接
	s.connMgr, err = connmgr.NewConnManager(
		30,  // 低水位：连接数低于此值时触发 prune
		200, // 高水位：连接数高于此值时拒绝新连接
		connmgr.WithGracePeriod(time.Minute*2),
		connmgr.WithSilencePeriod(time.Second*10),
	)
	if err != nil {
		err = fmt.Errorf("failed to create connection manager: %v", err)
		return
	}

	s.host, err = libp2p.New(
		libp2p.ListenAddrs(listenAdders...),
		libp2p.AddrsFactory(func(adders []multiaddr.Multiaddr) []multiaddr.Multiaddr {
			// 返回对外宣布的公网地址，而不是本地监听地址
			return s.announce
		}),
		libp2p.ConnectionManager(s.connMgr), // 连接管理器
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		err = fmt.Errorf("failed to create host: %v", err)
		return
	}

	// 将配置的公网地址转换为字符串形式，供客户端使用
	s.addresses = make([]string, 0, len(s.announce))
	for _, addr := range s.announce {
		s.addresses = append(s.addresses, addr.String())
	}
	s.hostId = s.host.ID().String()

	g.Log().Infof(ctx, "p2p signaling server started: %s", s.ID())
	g.Log().Infof(ctx, "p2p signaling server addresses: \n%v",
		s.addresses)
	return
}

func (s *p2pSignalingServer) Close() (err error) {
	if s.connMgr != nil {
		_ = s.connMgr.Close()
	}
	if s.host != nil {
		_ = s.host.Close()
	}
	g.Log().Infof(s.ctx, "p2p signaling server stopped")
	return
}

func (s *p2pSignalingServer) ID() string {
	return s.hostId
}

func (s *p2pSignalingServer) Addresses() []string {
	return s.addresses
}
