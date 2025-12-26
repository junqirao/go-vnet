package router

import (
	"context"
	"io"

	"go-vnet/common/logger"
)

type (
	Router interface {
		Register(ctx context.Context, addr string, conn any) (err error)
		Route(ctx context.Context, r io.ReadWriteCloser) (dst io.Writer, err error)
		RouteString(dst string) (v any, ok bool)
		Dump() []byte
		Restore(data []byte) (err error)
		Delete(ctx context.Context, addr string) (err error)
		Hash() string
	}
	router struct {
		table *RouteTable
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

func (r *router) Register(ctx context.Context, addr string, conn any) (err error) {
	return r.table.AddRoute(ctx, addr, conn)
}

func (r *router) Route(_ context.Context, src io.ReadWriteCloser) (dst io.Writer, err error) {
	buf := make([]byte, 15)
	n, err := src.Read(buf)
	if err != nil {
		return
	}
	res, ok := r.table.Lookup(string(buf[:n]))
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

func (r *router) Restore(data []byte) (err error) {
	if len(data) == 0 {
		return nil
	}
	newRoot, err := UnMarshalTriNode(data)
	if err != nil {
		return err
	}
	// 合并路由表，不覆盖已有的路由
	mergeTrieNodes(r.table.root, newRoot)
	return nil
}

// mergeTrieNodes 合并两个TrieNode，不覆盖已存在的叶子节点
func mergeTrieNodes(dest, src *TrieNode) {
	if src == nil {
		return
	}

	// 如果源节点是叶子节点且目标节点不是叶子节点，则复制
	if src.isLeaf && !dest.isLeaf {
		dest.isLeaf = src.isLeaf
		dest.target = src.target
	}

	// 递归合并子节点
	if src.zero != nil {
		if dest.zero == nil {
			dest.zero = &TrieNode{}
		}
		mergeTrieNodes(dest.zero, src.zero)
	}
	if src.one != nil {
		if dest.one == nil {
			dest.one = &TrieNode{}
		}
		mergeTrieNodes(dest.one, src.one)
	}
}

func (r *router) Delete(ctx context.Context, addr string) (err error) {
	return r.table.DeleteRoute(ctx, addr)
}

func (r *router) Hash() string {
	return r.table.Hash()
}
