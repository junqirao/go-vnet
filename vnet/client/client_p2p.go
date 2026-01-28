package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"

	"go-vnet/vnet/server"
)

func (c *Client) setupP2P(ctx context.Context) (err error) {
	c.relayInfo, err = c.getRelayInfo(ctx)
	if err != nil {
		return
	}

	// todo select multi addr
	if len(c.relayInfo.Addresses) == 0 {
		return
	}

	if err = c.registerRelay(ctx, c.relayInfo.Addresses[0]); err != nil {
		return
	}
	return
}

func (c *Client) getRelayInfo(ctx context.Context) (info *server.RelayInfo, err error) {
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetP2PRelayInfo)
	if err != nil {
		return
	}
	info = new(server.RelayInfo)
	data, err := base64.StdEncoding.DecodeString(resp.Data.(string))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &info)
	return
}

func (c *Client) registerRelay(ctx context.Context, addr string) (err error) {
	if c.relayInfo == nil || c.relayInfo.Id == "" {
		return
	}
	host, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.EnableRelay(),
	)
	if err != nil {
		return
	}
	c.host = host

	relayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", addr, c.relayInfo.Id))
	if err != nil {
		return
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(relayAddr)
	if err != nil {
		return
	}

	if err = host.Connect(ctx, *addrInfo); err != nil {
		return
	}

	_, err = client.Reserve(ctx, host, *addrInfo)
	if err != nil {
		return
	}

	host.SetStreamHandler("/transport", func(stream network.Stream) {
		c.logger.Infof(ctx, "accept p2p stream from %s", stream.Conn().RemotePeer())
		c.hub.HandleRx(stream)
	})

	c.hostId = host.ID().String()
	c.logger.Infof(ctx, "register p2p relay success: %s", c.hostId)
	return
}
