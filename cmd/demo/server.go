package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

type Server struct {
	listener *quic.Listener
	connA    *quic.Conn
	connB    *quic.Conn
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run() {
	ctx := context.Background()
	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	listener, err := quic.ListenAddr(":8080", tls, config.DefaultQuicConfig)
	if err != nil {
		panic(err)
	}
	err = s.Listen(ctx, listener)
	if err != nil {
		panic(err)
		return
	}
	err = s.Forward(ctx)
	if err != nil {
		panic(err)
		return
	}
}

// Listen 监听连接，直到connA和connB都不为nil
func (s *Server) Listen(ctx context.Context, listener *quic.Listener) error {
	s.listener = listener
	log.Println("Server listening, waiting for connA and connB...")

	for s.connA == nil || s.connB == nil {
		conn, err := listener.Accept(ctx)
		if err != nil {
			log.Printf("Accept error: %v", err)
			return err
		}

		if s.connA == nil {
			s.connA = conn
			log.Println("connA connected")
		} else {
			s.connB = conn
			log.Println("connB connected")
		}
	}

	log.Println("Both connA and connB connected")
	return nil
}

// Forward 在两个连接之间转发流量
func (s *Server) Forward(ctx context.Context) error {
	log.Println("Starting traffic forwarding...")

	// 启动两个goroutine分别处理A->B和B->A的流量转发
	errCh := make(chan error, 2)

	go func() {
		errCh <- s.forwardBidirectional(ctx, s.connA, s.connB, "a->b")
	}()

	go func() {
		errCh <- s.forwardBidirectional(ctx, s.connB, s.connA, "b->a")
	}()

	// 等待任意一个方向的转发出错
	return <-errCh
}

// forwardBidirectional 接受stream并将流量从一个连接转发到另一个连接
func (s *Server) forwardBidirectional(ctx context.Context, source, dest *quic.Conn, name string) error {
	fmt.Println("start forwardBidirectional ", name)
	for {
		stream, err := source.AcceptStream(ctx)
		if err != nil {
			log.Printf("AcceptStream error: %v", err)
			return err
		}

		go s.forwardStream(stream, dest, name)
	}
}

// forwardStream 转发单个stream的数据
func (s *Server) forwardStream(stream *quic.Stream, dest *quic.Conn, name string) {
	defer stream.Close()
	fmt.Println("forwardStream ", name)

	destStream, err := dest.OpenStreamSync(stream.Context())
	if err != nil {
		log.Printf("OpenStreamSync error: %v", err)
		return
	}
	defer destStream.Close()

	n, err := proxy(name, destStream, stream)
	if err != nil {
		log.Printf("%s io.Copy error: %v", name, err)
		return
	}
	log.Printf("%s io.Copy %d bytes", name, n)
}

func proxy(name string, dst io.Writer, src io.Reader) (written int64, err error) {
	fmt.Println("start proxy", name)
	var (
		buf = make([]byte, 1400) // 优化: 增大缓冲区到1.4MB,提高吞吐量
		nr  int
		er  error
	)

	for {
		nr, er = src.Read(buf)
		if nr > 0 {
			// fmt.Printf("%s : %v\n", name, buf[:nr])
			nw, ew := dst.Write(buf[0:nr])
			if ew != nil {
				return written, ew
			}
			if nw > 0 {
				written += int64(nw)
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
