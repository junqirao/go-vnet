package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go-vnet/vnet/server/consts"
)

const (
	FuncNamePing               = "ping"
	FuncNameGetRouterData      = "get_router_data"
	FuncNameGetP2PRelayInfo    = "get_p2p_relay_info"
	FuncNameGetP2PRelayMapping = "get_p2p_relay_mapping"
)

var (
	funcPing = FuncCallInfo{
		Name: FuncNamePing,
		Fn: func(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			session.storage.Store(sessionStorageKeyLastPing, time.Now())
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			v, ok := req.Args["host_id"]
			if ok && v != "" {
				hostId := v.(string)
				_, ok := session.storage.LoadOrStore(sessionStorageKeyRelayHostId, hostId)
				if !ok {
					server.logger.Infof(ctx, "registered p2p relay host id %s from %s", hostId, session.IP)
					server.relayVersion.Add(1)
				}
			}
			return &FuncCallResponse{Code: 0, Data: fmt.Sprintf("%s,%d",
				session.network.Router().MD5(),
				server.relayVersion.Load())}, nil
		},
	}
	funcGetRouteData = FuncCallInfo{
		Name: FuncNameGetRouterData,
		Fn: func(_ context.Context, session *serverSession, _ *FuncCallRequest) (resp *FuncCallResponse, err error) {
			data := session.network.Router().Keys()
			bs, _ := json.Marshal(data)
			return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
		},
	}
	funcGetP2PRelayInfo = FuncCallInfo{
		Name: FuncNameGetP2PRelayInfo,
		Fn: func(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}

			info := RelayInfo{
				Id:        server.relay.ID(),
				Addresses: server.relay.Addresses(),
			}
			bs, _ := json.Marshal(info)
			return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
		},
	}
	funcGetP2PRelayMapping = FuncCallInfo{
		Name: FuncNameGetP2PRelayMapping,
		Fn: func(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			res := map[string]string{}
			server.sessions.Range(func(_, value any) bool {
				session, ok := value.(*serverSession)
				if !ok {
					return true
				}
				v, ok := session.storage.Load(sessionStorageKeyRelayHostId)
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
)
