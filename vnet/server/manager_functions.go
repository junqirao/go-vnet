package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/vnet/server/consts"
)

const (
	FuncNamePing              = "ping"
	FuncNameGetRouterData     = "get_router_data"
	FuncNameRegisterP2PPeer   = "register_p2p_peer"
	FuncNameGetP2PPeerInfo    = "get_p2p_peer_info"
	FuncNameGetP2PPeerMapping = "get_p2p_relay_mapping"
)

var (
	funcPing = FuncCallInfo{
		Name: FuncNamePing,
		Fn: func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			session.storage.Store(sessionStorageKeyLastPing, time.Now())
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			return &FuncCallResponse{Code: 0, Data: fmt.Sprintf("%s,%d",
				session.network.Router().MD5(),
				server.peerMappingVersion.Load())}, nil
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
	funcGetP2PRelayInfo = FuncCallInfo{
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
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			res := map[string]string{}
			server.sessions.Range(func(_, value any) bool {
				session, ok := value.(*Session)
				if !ok {
					return true
				}
				v, ok := session.storage.Load(sessionStorageKeyP2PPeer)
				if !ok {
					return true
				}
				hostId, ok := v.(string)
				if !ok || hostId == "" {
					return true
				}
				res[session.IP] = hostId
				return true
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
			v, ok := req.Args["peer"]
			if ok && v != "" {
				peer := v.(string)
				_, ok := session.storage.LoadOrStore(sessionStorageKeyP2PPeer, peer)
				if !ok {
					g.Log().Infof(ctx, "registered p2p peer from %s: %s", session.IP, peer)
					server.peerMappingVersion.Add(1)
				}
			}
			return &FuncCallResponse{Code: 0, Data: nil}, nil
		},
	}
)
