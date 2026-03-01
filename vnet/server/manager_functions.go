package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/quic-go/quic-go"

	"go-vnet/vnet/server/consts"
)

const (
	FuncNamePing              = "ping"
	FuncNameGetRouterData     = "get_router_data"
	FuncNameRegisterP2PPeer   = "register_p2p_peer"
	FuncNameRefreshP2PAddress = "refresh_p2p_address"
	FuncNameGetP2PPeerInfo    = "get_p2p_peer_info"
	FuncNameGetP2PPeerMapping = "get_p2p_relay_mapping"
	FuncNameCloseSession      = "close_session"
)

// getNATAddressFromConn 从连接中获取NAT地址
func getNATAddressFromConn(conn any) (ip string, port int, ok bool) {
	switch c := conn.(type) {
	case *quic.Conn:
		remoteAddr := c.RemoteAddr()
		if remoteAddr != nil {
			realRemoteAddr := remoteAddr.String()
			if host, portStr, err := net.SplitHostPort(realRemoteAddr); err == nil {
				return host, getPortFromString(portStr), true
			}
		}
	case net.Conn:
		remoteAddr := c.RemoteAddr()
		if remoteAddr != nil {
			realRemoteAddr := remoteAddr.String()
			if host, portStr, err := net.SplitHostPort(realRemoteAddr); err == nil {
				return host, getPortFromString(portStr), true
			}
		}
	}
	return "", 0, false
}

// getPortFromString 从字符串中提取端口号
func getPortFromString(portStr string) int {
	port, _ := net.LookupPort("tcp", portStr)
	return port
}

// updateP2PPeerAddress 更新P2P peer地址并广播
func updateP2PPeerAddress(ctx context.Context, session *Session, server *Server, peerInfo *peer.AddrInfo, isRefresh bool) (string, error) {
	// 从连接中获取NAT地址
	realRemoteIP, realPort, hasNATAddr := getNATAddressFromConn(session.conn)

	// 获取原始地址列表
	originalAddrs := make([]string, 0, len(peerInfo.Addrs))
	for _, addr := range peerInfo.Addrs {
		originalAddrs = append(originalAddrs, addr.String())
	}

	// 检查是否需要添加NAT地址
	if hasNATAddr && realPort > 0 && realRemoteIP != "" {
		// 获取服务器配置的所有P2P地址
		var newNATAddrs []multiaddr.Multiaddr
		if server.cfg.P2P != nil {
			for _, serverAddr := range server.cfg.P2P.Addresses {
				// 根据配置生成对应协议的地址
				switch serverAddr.Transport {
				case "tcp":
					newAddr := multiaddr.StringCast(fmt.Sprintf("/ip4/%s/tcp/%d", realRemoteIP, realPort))
					newNATAddrs = append(newNATAddrs, newAddr)
				case "udp":
					if serverAddr.Version == "quic-v1" {
						newAddr := multiaddr.StringCast(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", realRemoteIP, realPort))
						newNATAddrs = append(newNATAddrs, newAddr)
					} else {
						newAddr := multiaddr.StringCast(fmt.Sprintf("/ip4/%s/udp/%d", realRemoteIP, realPort))
						newNATAddrs = append(newNATAddrs, newAddr)
					}
				}
			}
		} else {
			// 如果没有配置，使用默认TCP地址
			newNATAddrs = append(newNATAddrs, multiaddr.StringCast(fmt.Sprintf("/ip4/%s/tcp/%d", realRemoteIP, realPort)))
		}

		// 检查并添加新地址
		addedCount := 0
		for _, newNATAddr := range newNATAddrs {
			// 检查地址是否已存在
			addrExists := false
			for _, addr := range peerInfo.Addrs {
				if addr.Equal(newNATAddr) {
					addrExists = true
					break
				}
			}

			if !addrExists {
				// 将新的NAT地址添加到列表前面
				peerInfo.Addrs = append([]multiaddr.Multiaddr{newNATAddr}, peerInfo.Addrs...)
				addedCount++
				g.Log().Debugf(ctx, "p2p peer %s: added NAT address %s", session.IP, newNATAddr.String())
			}
		}

		if addedCount > 0 {
			if isRefresh {
				g.Log().Infof(ctx, "p2p peer %s: refreshed %d NAT addresses for IP %s:%d",
					session.IP, addedCount, realRemoteIP, realPort)
			} else {
				g.Log().Infof(ctx, "p2p peer %s: added %d server-observed NAT addresses %s:%d (original: %v)",
					session.IP, addedCount, realRemoteIP, realPort, originalAddrs)
			}
		} else {
			if isRefresh {
				g.Log().Debugf(ctx, "p2p peer %s: all NAT addresses already exist, skipping refresh",
					session.IP)
			} else {
				g.Log().Debugf(ctx, "p2p peer %s: all NAT addresses already exist",
					session.IP)
			}
		}
	} else {
		if isRefresh {
			g.Log().Warningf(ctx, "p2p peer %s: cannot observe NAT address, skipping refresh", session.IP)
			return "", fmt.Errorf("cannot observe NAT address")
		}

		g.Log().Warningf(ctx, "p2p peer %s: no NAT address observed from connection (original: %v)",
			session.IP, originalAddrs)
	}

	// 序列化更新后的peerInfo
	newPeerBytes, err := json.Marshal(peerInfo)
	if err != nil {
		g.Log().Errorf(ctx, "failed to marshal peer info: %v", err)
		return "", fmt.Errorf("failed to marshal peer info: %v", err)
	}

	newPeerStr := string(newPeerBytes)

	// 更新存储
	session.storage.Store(sessionStorageKeyP2PPeer, newPeerStr)

	// 更新路由
	session.network.p2pRouter.Register(fmt.Sprintf("%s/32", session.IP), newPeerStr)

	server.BroadcastPeer(ctx, EventNameP2PPeerUpdate, session, newPeerStr)
	return newPeerStr, nil
}

var (
	funcPing = FuncCallInfo{
		Name: FuncNamePing,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			session.storage.Store(sessionStorageKeyLastPing, time.Now())
			return &FuncCallResponse{Code: 0, Data: fmt.Sprintf("%s,%s",
				session.network.Router().MD5(),
				session.network.p2pRouter.MD5WithValue())}, nil
		},
	}
	funcGetRouteData = FuncCallInfo{
		Name: FuncNameGetRouterData,
		Fn: func(_ context.Context, session *Session, _ *FuncCallRequest) (resp *FuncCallResponse, err error) {
			data := session.network.Router().Keys()
			bs, _ := json.Marshal(data)
			return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
		},
	}
	funcGetP2PInfo = FuncCallInfo{
		Name: FuncNameGetP2PPeerInfo,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}

			info := AddressInfo{
				Id:        server.p2pSignalingServer.ID(),
				Addresses: server.p2pSignalingServer.Addresses(),
			}
			bs, _ := json.Marshal(info)
			return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
		},
	}
	funcGetP2PRelayMapping = FuncCallInfo{
		Name: FuncNameGetP2PPeerMapping,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			res := map[string]string{}
			session.network.p2pRouter.Range(func(key string, value any) {
				res[key] = value.(string)
			})
			bs, _ := json.Marshal(res)
			return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
		},
	}
	funcRegisterP2PPeer = FuncCallInfo{
		Name: FuncNameRegisterP2PPeer,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			var p string
			v, ok := req.Args["peer"]
			if ok && v != "" {
				p = v.(string)

				// 解析客户端发送的 peerInfo
				var peerInfo peer.AddrInfo
				if err = json.Unmarshal([]byte(p), &peerInfo); err != nil {
					g.Log().Errorf(ctx, "failed to unmarshal peer info: %v", err)
					return &FuncCallResponse{Code: 1, Data: "invalid peer info"}, nil
				}

				// 更新P2P peer地址（首次注册）
				newPeerStr, err := updateP2PPeerAddress(ctx, session, server, &peerInfo, false)
				if err != nil {
					// 首次注册即使失败也记录，不返回错误
					g.Log().Warningf(ctx, "failed to update p2p peer address during registration: %v", err)
				}

				// 如果updateP2PPeerAddress返回空字符串，使用原始peerInfo
				if newPeerStr == "" {
					if peerBytes, err := json.Marshal(peerInfo); err == nil {
						newPeerStr = string(peerBytes)
					}
				}

				_, ok = session.storage.LoadOrStore(sessionStorageKeyP2PPeer, newPeerStr)
				if !ok {
					g.Log().Infof(ctx, "registered p2p peer from %s", session.IP)
				}
			}
			return &FuncCallResponse{Code: 0, Data: nil}, nil
		},
	}
	funcRefreshP2PAddress = FuncCallInfo{
		Name: FuncNameRefreshP2PAddress,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}

			// 从存储中获取当前的peerInfo
			v, ok := session.storage.Load(sessionStorageKeyP2PPeer)
			if !ok {
				g.Log().Warningf(ctx, "no p2p peer info found for %s, nothing to refresh", session.IP)
				return &FuncCallResponse{Code: 0, Data: nil}, nil
			}

			currentPeerStr := v.(string)

			// 解析当前的peerInfo
			var peerInfo peer.AddrInfo
			if err = json.Unmarshal([]byte(currentPeerStr), &peerInfo); err != nil {
				g.Log().Errorf(ctx, "failed to unmarshal current peer info: %v", err)
				return &FuncCallResponse{Code: 1, Data: "failed to unmarshal peer info"}, nil
			}

			// 更新P2P peer地址（刷新）
			_, err = updateP2PPeerAddress(ctx, session, server, &peerInfo, true)
			if err != nil {
				return &FuncCallResponse{Code: 0, Data: nil}, nil // 刷新失败不返回错误
			}

			return &FuncCallResponse{Code: 0, Data: nil}, nil
		},
	}
	funcCloseSession = FuncCallInfo{
		Name: FuncNameCloseSession,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			session.Stop()
			session.Release(errors.New("client active close"))
			return &FuncCallResponse{Code: 0, Data: nil}, nil
		},
	}
)
