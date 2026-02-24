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
	FuncNameGetP2PPeerInfo    = "get_p2p_peer_info"
	FuncNameGetP2PPeerMapping = "get_p2p_relay_mapping"
	FuncNameCloseSession      = "close_session"
)

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

				// 从服务端连接中获取真实的远端地址和端口
				var realRemoteIP string
				var realPort int

				switch conn := session.conn.(type) {
				case *quic.Conn:
					// QUIC 连接
					remoteAddr := conn.RemoteAddr()
					if remoteAddr != nil {
						realRemoteAddr := remoteAddr.String()
						if host, portStr, err := net.SplitHostPort(realRemoteAddr); err == nil {
							realRemoteIP = host
							realPort, _ = net.LookupPort("tcp", portStr)
						}
					}
				case net.Conn:
					// TCP 连接
					remoteAddr := conn.RemoteAddr()
					if remoteAddr != nil {
						realRemoteAddr := remoteAddr.String()
						if host, portStr, err := net.SplitHostPort(realRemoteAddr); err == nil {
							realRemoteIP = host
							realPort, _ = net.LookupPort("tcp", portStr)
						}
					}
				}

				// 解析客户端发送的 peerInfo，替换为服务端看到的真实地址
				var peerInfo peer.AddrInfo
				if err = json.Unmarshal([]byte(p), &peerInfo); err == nil {
					// 如果获取到了真实IP和端口，使用服务端看到的真实地址
					if realPort > 0 && realRemoteIP != "" {
						peerInfo.Addrs = append(peerInfo.Addrs, multiaddr.StringCast(fmt.Sprintf("/ip4/%s/tcp/%d", realRemoteIP, realPort)))
						if newPeerBytes, err := json.Marshal(peerInfo); err == nil {
							p = string(newPeerBytes)
						}
						g.Log().Infof(ctx, "p2p peer address replaced: virtual=%s, real=%s:%d",
							session.IP, realRemoteIP, realPort)
					}
				}

				_, ok = session.storage.LoadOrStore(sessionStorageKeyP2PPeer, p)
				if !ok {
					g.Log().Infof(ctx, "registered p2p peer from %s: %s", session.IP, p)
					server.BroadcastPeer(ctx, EventNameP2PPeerUpdate, session, p)
					session.network.p2pRouter.Register(fmt.Sprintf("%s/32", session.IP), p)
				}
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
