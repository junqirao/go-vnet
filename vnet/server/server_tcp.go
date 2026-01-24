package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"

	"go-vnet/common/session"
)

type (
	tcpServer struct {
		*Server
		cfg           *TransportConfig
		listener      *net.TCPListener
		acceptErr     chan error
		acceptCh      chan net.Conn
		transportChs  sync.Map // ip : chan net.Conn
		transportConn sync.Map // ip : net.Conn
		workerPool    *ants.Pool
	}
)

func newTcpServer(s *Server) internalServer {
	wp, _ := ants.NewPool(runtime.NumCPU())
	return &tcpServer{
		Server:        s,
		acceptErr:     make(chan error),
		acceptCh:      make(chan net.Conn),
		transportConn: sync.Map{},
		workerPool:    wp,
	}
}

func (s *tcpServer) Setup(ctx context.Context, cfg *TransportConfig) (err error) {
	s.cfg = cfg
	// listen
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port))
	if err != nil {
		return
	}
	s.listener, err = net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}
	go s.acceptLoop(ctx)
	return
}

func (s *tcpServer) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-s.Server.sig:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				s.logger.Errorf(ctx, "%s accept connection error: %s", s.cfg.Name, err.Error())
				continue
			}
			err = s.workerPool.Submit(func() {
				err = s.dispatch(ctx, conn)
				if err != nil {
					s.logger.Errorf(ctx, "dispatch connection error: %s", err.Error())
					return
				}
				// drop err if full
				select {
				case s.acceptErr <- err:
				default:
				}
			})
			if err != nil {
				s.logger.Errorf(ctx, "drop connection, on submit error: %s", err.Error())
				_ = conn.Close()
			}
		}
	}
}

func (s *tcpServer) dispatch(ctx context.Context, conn net.Conn) (err error) {
	// timeout 3s for first packet
	var (
		buf   = make([]byte, 15)
		ch    = make(chan []byte)
		first []byte
	)

	go func() {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		ch <- buf[:n]
	}()

	timer := time.NewTimer(time.Second * 3)
	select {
	case <-timer.C:
		err = fmt.Errorf("read first pkg timeout. remote=%v", conn.RemoteAddr().String())
		return
	case first = <-ch:
	}

	if s.isControl(first) {
		s.logger.Infof(ctx, "dispatch control connection, remote=%s", conn.RemoteAddr().String())
		s.acceptCh <- conn
		return
	}

	src := string(first)
	s.logger.Infof(ctx, "dispatch transport connection, refer=%s", src)
	// make sure the connection is established
	if _, ok := s.sessions.Load(src); !ok {
		err = fmt.Errorf("refer session not found: refer=%s", src)
		return
	}
	s.transportConn.Store(src, conn)
	v, _ := s.transportChs.LoadOrStore(src, make(chan net.Conn))
	tch, ok := v.(chan net.Conn)
	if !ok {
		err = fmt.Errorf("internal error: invalid transport channel")
		return
	}
	tch <- conn
	return
}

func (s *tcpServer) isControl(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if i == 0 && b[i] != 1 {
			return false
		}
		if i > 0 && b[i] != 0 {
			return false
		}
	}
	return true

}

func (s *tcpServer) Accept(ctx context.Context) (ss *serverSession, err error) {
	select {
	case <-s.Server.sig:
		err = errors.New("server closed")
		return
	case err = <-s.acceptErr:
		return
	case conn := <-s.acceptCh:
		ss = &serverSession{
			conn:              conn,
			SendReceiveCloser: session.SendReceiverFromNetConn(conn),
		}
		return
	}
}

func (s *tcpServer) Close() error {
	return s.listener.Close()
}

func (s *tcpServer) AcceptTransport(session *serverSession) (rwc io.ReadWriteCloser, err error) {
	v, _ := s.transportChs.LoadOrStore(session.IP, make(chan net.Conn))
	ch, ok := v.(chan net.Conn)
	if !ok {
		// usually not gonna happen
		err = fmt.Errorf("internal error: invalid transport channel")
		return
	}
	select {
	case <-s.Server.sig:
		err = errors.New("server closed")
		return
	case rwc = <-ch:
		return
	}
}

func (s *tcpServer) GetDstTransportWriter(src *serverSession, dst *serverSession) (rwc io.ReadWriteCloser, err error) {
	v, ok := s.transportConn.Load(dst.IP)
	if !ok {
		err = fmt.Errorf("transport connection not found: refer=%s", dst.IP)
		return
	}
	conn, ok := v.(net.Conn)
	if !ok {
		err = fmt.Errorf("internal error: invalid transport connection")
		return
	}
	rwc = conn
	return
}
