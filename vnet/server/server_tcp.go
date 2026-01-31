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
		txChs         sync.Map // ip : chan net.Conn
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
		txChs:         sync.Map{},
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
					// drop err if full
					select {
					case s.acceptErr <- err:
					default:
					}
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
		buf   = make([]byte, 33)
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
		s.logger.Infof(ctx, "dispatch connection, packet=%v", first)
	}

	defer func() {
		// send back ack or close
		if err != nil {
			s.logger.Errorf(ctx, "dispatch connection error: %s", err.Error())
			_ = conn.Close()
		}
		_, err = conn.Write([]byte{1})
	}()

	switch first[0] {
	case 1:
		s.logger.Infof(ctx, "dispatch control connection, remote=%s", conn.RemoteAddr().String())
		s.acceptCh <- conn
	case 4:
		src := net.IPv4(first[1], first[2], first[3], first[4]).String()
		dst := net.IPv4(first[5], first[6], first[7], first[8]).String()
		// make sure the connection is established
		v, ok := s.sessions.Load(dst)
		if !ok {
			err = fmt.Errorf("refer session not found: dst=%s", dst)
			return
		}

		s.logger.Infof(ctx, "dispatch transport connection, dst=%s", dst)
		s.transportConn.Store(dst, conn)

		sess := v.(*Session)
		if src == dst {
			// rx only
			sess.storage.Store("rx", conn)
			s.logger.Infof(ctx, "dispatch rx connection, src=%s,session=%s", src, sess.SessionId)
		} else {
			// tx
			v, _ = s.txChs.LoadOrStore(src, make(chan net.Conn))
			tch, ok := v.(chan net.Conn)
			if !ok {
				err = fmt.Errorf("internal error: invalid transport channel")
				return
			}
			tch <- conn
		}
	default:
		err = fmt.Errorf("unsupported dispatch packet type: %d", first[0])
	}
	return
}

func (s *tcpServer) Accept(ctx context.Context) (ss *Session, err error) {
	select {
	case <-s.Server.sig:
		err = errors.New("server closed")
		return
	case err = <-s.acceptErr:
		return
	case conn := <-s.acceptCh:
		ss = newServerSession(session.SendReceiverFromNetConn(conn), conn)
		return
	}
}

func (s *tcpServer) Close() error {
	return s.listener.Close()
}

func (s *tcpServer) AcceptTransport(session *Session) (rwc io.ReadWriteCloser, err error) {
	v, _ := s.txChs.LoadOrStore(session.IP, make(chan net.Conn))
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
	case err = <-s.acceptErr:
		return
	case <-session.sig:
		err = errors.New("session closed")
		return
	case rwc = <-ch:
		return
	}
}

func (s *tcpServer) GetDstTransportWriter(_ *Session, dst *Session) (rwc io.ReadWriteCloser, err error) {
	v, ok := dst.storage.Load("rx")
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
