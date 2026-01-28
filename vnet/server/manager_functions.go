package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"go-vnet/vnet/server/consts"
)

var (
	funcPing = FuncCallInfo{
		Name: FuncNamePing,
		Fn: func(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
			session.storage.Store("last_ping", time.Now())
			s := ctx.Value(consts.CtxKeyServer)
			server, ok := s.(*Server)
			if !ok {
				err = errors.New("internal type error of value 'server'")
				return
			}
			v, ok := req.Args["host_id"]
			if ok && v != "" {
				hostId := v.(string)
				host, ok := server.relayHosts.LoadOrStore(session.IP, &RelayHost{
					Id:            hostId,
					LastHeartbeat: time.Now(),
				})
				if ok {
					h := host.(*RelayHost)
					h.Id = hostId
					h.LastHeartbeat = time.Now()
				} else {
					server.logger.Infof(ctx, "registered p2p relay host id %s from %s", hostId, session.IP)
				}
			}
			return &FuncCallResponse{Code: 0, Data: session.network.Router().MD5()}, nil
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
)
