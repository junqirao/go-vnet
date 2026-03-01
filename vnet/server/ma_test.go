package server

import (
	"testing"
)

func TestReplaceAddress(t *testing.T) {
	ma := "/ip4/127.0.0.1/udp/1234/quic-v1"
	t.Log(replaceAddress(ma, "192.168.1.1", "8080"))
}
