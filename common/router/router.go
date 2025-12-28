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
		Restore(ctx context.Context, data []byte) (err error)
		Delete(ctx context.Context, addr string) (err error)
		Hash() string
		SetLogger(logger logger.Logger)
		Len() int
		Print() string
	}
	router struct {
		table  *RouteTable
		logger logger.Logger
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
		table:  NewRouteTable(root),
		logger: logger.NopLogger,
	}
}

func (r *router) Register(ctx context.Context, addr string, conn any) (err error) {
	r.logger.Infof(ctx, "register route: addr=%s", addr)
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

func (r *router) Restore(ctx context.Context, data []byte) (err error) {
	if len(data) == 0 {
		return nil
	}
	newRoot, err := UnMarshalTriNode(data)
	if err != nil {
		return err
	}
	// 同步路由表：合并data中的路由，删除目标中不存在于data的路由
	r.syncTrieNodes(ctx, r.table.root, newRoot)
	return nil
}

// syncTrieNodes 同步两个TrieNode：
// 1. 添加src中存在但dest中不存在的路由
// 2. 删除dest中存在但src中不存在的路由
// 3. 保留dest中已有的路由（不覆盖target）
func (r *router) syncTrieNodes(ctx context.Context, dest, src *TrieNode) {
	if dest == nil {
		return
	}

	// 处理当前节点
	if dest.isLeaf {
		if src == nil || !src.isLeaf {
			// dest是叶子节点但src不是或src不存在，删除dest中的路由
			r.logger.Infof(ctx, "Route deleted: target=%v", dest.target)
			dest.isLeaf = false
			dest.target = nil
		}
		// 如果dest和src都是叶子节点，保留dest的target（不覆盖）
	} else if src != nil && src.isLeaf && !dest.isLeaf {
		// src是叶子节点但dest不是，添加路由
		dest.isLeaf = src.isLeaf
		dest.target = src.target
		r.logger.Infof(ctx, "Route merged: target=%v", src.target)
	}

	// 递归处理子节点
	// 处理zero分支
	if src != nil && src.zero != nil {
		// src有zero分支，递归同步
		if dest.zero == nil {
			dest.zero = &TrieNode{}
		}
		r.syncTrieNodes(ctx, dest.zero, src.zero)
	} else if dest.zero != nil {
		// src没有zero分支但dest有，递归删除dest.zero中的路由
		r.cleanTrieNodes(ctx, dest.zero)
	}

	// 处理one分支
	if src != nil && src.one != nil {
		// src有one分支，递归同步
		if dest.one == nil {
			dest.one = &TrieNode{}
		}
		r.syncTrieNodes(ctx, dest.one, src.one)
	} else if dest.one != nil {
		// src没有one分支但dest有，递归删除dest.one中的路由
		r.cleanTrieNodes(ctx, dest.one)
	}
}

// cleanTrieNodes 递归删除TrieNode中的所有路由（清除叶子节点标记和target）
func (r *router) cleanTrieNodes(ctx context.Context, node *TrieNode) {
	if node == nil {
		return
	}
	if node.isLeaf {
		r.logger.Infof(ctx, "Route deleted: target=%v", node.target)
		node.isLeaf = false
		node.target = nil
	}
	r.cleanTrieNodes(ctx, node.zero)
	r.cleanTrieNodes(ctx, node.one)
}

func (r *router) Delete(ctx context.Context, addr string) (err error) {
	return r.table.DeleteRoute(ctx, addr)
}

func (r *router) Hash() string {
	return r.table.Hash()
}

func (r *router) SetLogger(logger logger.Logger) {
	r.logger = logger
	r.table.logger = logger
}

func (r *router) Len() int {
	return r.table.Len()
}

func (r *router) Print() string {
	return r.table.Print()
}
