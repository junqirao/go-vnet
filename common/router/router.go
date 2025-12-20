package router

import (
	"context"
	"fmt"
	"io"

	"go-vnet/common/logger"
)

type (
	Router interface {
		Register(addr string, conn any) (err error)
		Route(ctx context.Context, r io.ReadWriteCloser) (dst io.Writer, err error)
		RouteString(dst string) (v any, ok bool)
		Dump() []byte
	}
	router struct {
		table    *RouteTable
		fallback io.Writer
	}
)

func NewRouter(data ...[]byte) Router {
	var root *TrieNode
	if len(data) > 0 && len(data[0]) > 0 {
		if r, err := UnMarshalTriNode(data[0]); err == nil {
			root = r
		} else {
			logger.DefaultLogger.Errorf(context.Background(),
				"failed to unmarshal route table data; %s", err.Error())
		}
	}
	return &router{
		table: NewRouteTable(root),
	}
}

func (r *router) Register(addr string, conn any) (err error) {
	return r.table.AddRoute(addr, conn)
}

func (r *router) Route(ctx context.Context, src io.ReadWriteCloser) (dst io.Writer, err error) {
	buf := make([]byte, 15)
	n, err := src.Read(buf)
	if err != nil {
		return
	}
	fmt.Printf("recv route pkg : %s\n", string(buf[:n]))
	res, ok := r.table.Lookup(string(buf[:n]))
	fmt.Printf("route: res=%+v,ok=%v\n", res, ok)
	if ok {
		if d, ok := res.(io.Writer); ok {
			dst = d
		}
	}
	var writeBack byte = 0
	if ok {
		writeBack = 1
	}
	_, err = src.Write([]byte{writeBack})
	return
}

func (r *router) RouteString(dst string) (v any, ok bool) {
	return r.table.Lookup(dst)
}

func (r *router) Dump() []byte {
	return r.table.MarshalRouteTable()
}
