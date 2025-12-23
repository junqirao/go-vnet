package client

import (
	"context"
	"testing"

	"go-vnet/server/auth"
)

func TestClient_Run(t *testing.T) {
	c, err := NewClient(
		NewConfig(
			WithInsecureSkipVerify(true),
			WithNetworkId("test"),
			WithServer("127.0.0.1:9800"),
			WithAuth(auth.Config{
				Type:       auth.TypeSimplePassword,
				Password:   "",
				PrivateKey: "",
				PublicKey:  "",
			}),
		),
	)
	if err != nil {
		t.Fatal(err)
		return
	}
	err = c.Run(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
}
