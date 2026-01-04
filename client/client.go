package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/songgao/water/waterutil"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/device"
	"go-vnet/common/logger"
	"go-vnet/common/router"
	"go-vnet/common/session"
	"go-vnet/server"
)

type (
	internalClient interface {
		// Dial to server and create manager with established connection
		Dial(ctx context.Context) (sr session.SendReceiveCloser, conn any, err error)
		// SendToServer send flow to server by dst ip address
		SendToServer(dst string, buf []byte, n int) (err error)
		// ReadFromServerAndWriteToDevice read flow from server and write to device
		ReadFromServerAndWriteToDevice()
	}
	Client struct {
		internal      internalClient
		ctx           context.Context
		sig           chan struct{}
		cfg           *Config
		logger        logger.Logger
		auth          *auth.Client
		manager       *Manager
		router        router.Router
		bufPool       sync.Pool
		dev           device.IDevice
		src           string
		session       *session.ClientSession
		serverSession serverSession
	}
)

func NewClient(cfg *Config) *Client {
	c := &Client{
		cfg:    cfg,
		sig:    make(chan struct{}),
		logger: config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		auth:   auth.NewClient(cfg.Auth),
		router: router.NewRouter(),
	}
	switch cfg.Type {
	case TypeQuic:
		c.internal = newQuicClient(c)
	default:
		panic(fmt.Sprintf("invalid client type: %s", cfg.Type))
	}
	return c
}

func (c *Client) Run(ctx context.Context) (err error) {
	c.ctx = ctx

	// 1. connect to server
	c.logger.Infof(ctx, "connect to server %s:%d", c.cfg.Address, c.cfg.Port)
	sess, err := c.dial(ctx)
	if err != nil {
		return
	}

	// 2. setup device
	if c.dev != nil {
		_ = c.dev.Close()
	}
	if c.cfg.DeviceType != "" {
		sess.DispatchedDevice.Type = device.Type(c.cfg.DeviceType)
	}
	c.dev = device.NewTunDevice(sess.DispatchedDevice.Config)
	if err = c.dev.Setup(); err != nil {
		err = fmt.Errorf("failed to setup device: %w", err)
		sess.CloseWithError(err)
		return
	}
	ip, _, _ := net.ParseCIDR(sess.DispatchedDevice.CIDR)
	c.src = ip.To4().String()
	c.logger.Infof(ctx, "dispatched device ip=%s,mtu=%d", c.src,
		sess.DispatchedDevice.MTU)

	// 3. sync router loop
	go c.syncRouterLoop()

	// 4. handle rx flow
	go c.handleRX()

	// 5. block and handle tx flow
	return c.handleTX()
}

func (c *Client) dial(ctx context.Context) (sess *session.ClientSession, err error) {
	sr, conn, err := c.internal.Dial(ctx)
	if err != nil {
		return
	}

	// get payload and overwrite network id
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	// do auth
	resp, err := c.auth.Auth(ctx, payload,
		func(ctx context.Context, in []byte) (out []byte, err error) {
			if err = sr.Send(in); err != nil {
				return
			}
			return sr.Receive(ctx)
		},
	)
	if err != nil {
		return
	}

	joined := JoinNetworkResponse{}
	bs, _ := json.Marshal(resp)
	_ = json.Unmarshal(bs, &joined)

	sess = session.NewClientSession(
		session.NewSession(session.Type(c.cfg.Type), conn, joined.Session.DispatchedDevice, sr),
		joined.Session.Network.ID,
	)

	c.serverSession = joined.Session
	c.session = sess
	c.manager = NewManager(sess)
	c.logger.Infof(ctx, "session established: id=%s,network_id=%s", sess.Id, sess.NetworkId)
	return
}

func (c *Client) syncRouter(ctx context.Context) (err error) {
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute ping to server: %s", err.Error())
		return
	}
	// c.logger.Infof(ctx, "ping latency: %.2fms", resp.Cost)

	// update router if hash changed
	current := c.router.Hash()
	if resp.Data == current {
		return
	}

	// get router data from server
	c.logger.Infof(c.ctx, "router hash changed, current: %s, server: %s", current, resp.Data)
	resp, err = c.manager.CallFunc(ctx, server.FuncNameGetRouterData)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(c.ctx, "failed to decode router data from server: %s", err.Error())
			return
		}
		if err = c.router.Restore(c.ctx, bs); err != nil {
			c.logger.Errorf(c.ctx, "failed to restore router from server: %s", err.Error())
			return
		}

		c.logger.Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), c.router.Len())
	}
	return
}

func (c *Client) syncRouterLoop() {
	c.logger.Infof(c.ctx, "sync router loop started.")
	for {
		select {
		case <-c.sig:
			c.logger.Infof(c.ctx, "manager connection closed.")
			return
		case <-c.ctx.Done():
			c.logger.Infof(c.ctx, "manager connection closed.")
			return
		default:
		}

		// sync router
		if err := c.syncRouter(context.Background()); err != nil {
			c.logger.Errorf(c.ctx, "failed to sync router: %s", err.Error())
		}

		// sleep interval
		time.Sleep(time.Second * 5)
	}
}

func (c *Client) handleTX() error {
	cfg := c.dev.GetConfig()
	c.bufPool = sync.Pool{New: func() any {
		return make([]byte, cfg.MTU)
	}}

	// 使用信号量控制并发 goroutine 数量，避免内存爆炸
	sem := make(chan struct{}, 1000) // 允许 1000 个并发发送任务
	errChan := make(chan error, 1)

	for {
		select {
		case <-c.sig:
			return nil
		case err := <-errChan:
			return err
		default:
		}
		buf := c.bufPool.Get().([]byte)
		n, err := c.dev.Read(buf)
		if err != nil {
			c.bufPool.Put(buf)
			err = fmt.Errorf("failed to read device: %w", err)
			return err
		}
		if n == 0 {
			c.bufPool.Put(buf)
			continue
		}

		dst := waterutil.IPv4Destination(buf[:n]).String()

		// drop current loopback packet
		if dst == "127.0.0.1" || dst == c.src {
			c.bufPool.Put(buf)
			continue
		}

		// 异步发送，提高并发度和吞吐量
		select {
		case sem <- struct{}{}:
			go func(bufCopy []byte, dstCopy string, nCopy int) {
				defer func() {
					c.bufPool.Put(bufCopy)
					<-sem // 释放信号量
				}()
				if err := c.sendToServer(dstCopy, bufCopy, nCopy); err != nil {
					select {
					case errChan <- err:
					default:
					}
				}
			}(buf, dst, n)
		default:
			// 并发数已满，同步发送
			if err := c.sendToServer(dst, buf, n); err != nil {
				return err
			}
		}
	}
}

func (c *Client) sendToServer(dst string, buf []byte, n int) (err error) {
	defer func() {
		c.bufPool.Put(buf)
	}()
	_, ok := c.router.Route(dst)
	if !ok {
		return
	}
	return c.internal.SendToServer(dst, buf, n)
}

func (c *Client) handleRX() {
	c.internal.ReadFromServerAndWriteToDevice()
}

// writeDevice
func (c *Client) writeDevice(src io.ReadWriteCloser) (written int64, err error) {
	defer func() {
		_ = src.Close()
		c.logger.Infof(c.ctx, "handle rx stopped: %d bytes written, err=%v", written, err)
	}()

	var (
		buf = make([]byte, c.dev.GetConfig().MTU*1000) // 增大缓冲区到 1.4MB，提高吞吐量
		nr  int
		er  error
	)

	for {
		select {
		case <-c.ctx.Done():
			return written, c.ctx.Err()
		case <-c.sig:
			return
		default:
		}
		nr, er = src.Read(buf)
		if nr > 0 {
			// 检查第一个字节，判断是否是有效的 IP 数据包
			// IPv4: 0x45, IPv6: 0x60
			if nr > 0 && (buf[0] != 0x45 && buf[0] != 0x60) {
				// 不写入无效的数据包
				continue
			}

			wn, err := c.dev.Write(buf[0:nr])
			if err != nil {
				return written, err
			}
			if wn > 0 {
				written += int64(wn)
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
