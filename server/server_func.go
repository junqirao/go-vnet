package server

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"go-vnet/server/transport"
)

type (
	FuncCallRequest struct {
		FuncName string         `json:"func_name"`
		Args     map[string]any `json:"args"`
	}
	FuncCallResponse struct {
		Code    int    `json:"code"`
		Data    any    `json:"data"`
		Message string `json:"message"`
	}
)

const (
	FuncNamePing          = "ping"
	FuncNameGetRouterData = "get_router_data"
)

func (s *Server) processFuncCallLoop(ctx context.Context) {
	for {
		select {
		case ev := <-s.funcCallEventChan:
			var req FuncCallRequest
			err := json.Unmarshal(ev.Payload, &req)
			if err != nil {
				s.logger.Errorf(ctx, "unmarshal func call request error: network=%s,from=%s, err=%s",
					ev.Info.Network.ID, ev.Info.Src, err.Error())
				continue
			}
			resp, err := s.handleFuncCall(ctx, ev.Info, &req)
			if err != nil {
				s.logger.Errorf(ctx, "handle func call error: network=%s,from=%s, err=%s",
					ev.Info.Network.ID, ev.Info.Src, err.Error())
				continue
			}
			respBytes, _ := json.Marshal(resp)
			_, _ = ev.Info.RWC.Write(respBytes)
		case <-s.sig:
			s.logger.Infof(ctx, "server closed")
			for _, server := range s.transportServers {
				_ = server.Close()
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) handleFuncCall(ctx context.Context, ci transport.ConnectionInfo, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
	switch req.FuncName {
	case FuncNamePing:
		return &FuncCallResponse{Code: 0, Data: ci.Network.Router().Hash()}, nil
	case FuncNameGetRouterData:
		data := ci.Network.Router().Dump()
		return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(data)}, nil
	}
	return
}
