package connection

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
)

const (
	PosDial  = "dial"
	PosRead  = "read"
	PosWrite = "write"
)

type (
	Connection struct {
		Adaptor
		id  string
		ctx context.Context
		rwc io.ReadWriteCloser
	}
	Error struct {
		Conn     *Connection
		Position string
		Err      error
	}
	Adaptor interface {
		OnDial(ctx context.Context) (rwc io.ReadWriteCloser, err error)
		OnError(e *Error)
		OnRead(p []byte) []byte
		OnWrite(p []byte) []byte
	}
)

func (e *Error) Error() string {
	return fmt.Errorf("connection %s error caused at %s : %w",
		e.Conn.id, e.Position, e.Err).Error()
}

func NewConnection(ctx context.Context, a Adaptor) *Connection {
	return &Connection{
		Adaptor: a,
		id:      uuid.New().String(),
		ctx:     ctx,
	}
}

func (c *Connection) Read(p []byte) (n int, err error) {
	if err = c.MustDial(); err != nil {
		return
	}
	n, err = c.rwc.Read(c.OnRead(p))
	if err != nil {
		c.OnError(&Error{
			Conn:     c,
			Position: PosRead,
			Err:      err,
		})
	}
	return
}

func (c *Connection) Write(p []byte) (n int, err error) {
	if err = c.MustDial(); err != nil {
		return
	}
	n, err = c.rwc.Write(c.OnWrite(p))
	if err != nil {
		c.OnError(&Error{
			Conn:     c,
			Position: PosWrite,
			Err:      err,
		})
	}
	return
}

func (c *Connection) Close() error {
	if c.rwc == nil {
		return nil
	}
	return c.rwc.Close()
}

func (c *Connection) MustDial() (err error) {
	if c.rwc == nil {
		c.rwc, err = c.Adaptor.OnDial(c.ctx)
	}
	if err != nil {
		c.OnError(&Error{
			Conn:     c,
			Position: PosDial,
			Err:      err,
		})
	}
	return
}
