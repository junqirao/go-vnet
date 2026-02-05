package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/gogf/gf/v2/util/grand"
)

func genTestKey() (pri, pub string, err error) {
	// generate rsa key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}

	// encode private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	pri = string(privateKeyPEM)

	// encode public key to PEM format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	pub = string(publicKeyPEM)
	return
}

func TestClientBuildRequest(t *testing.T) {
	pri, pub, err := genTestKey()
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("private key: \n%s\n", pri)
	t.Logf("public key: \n%s\n", pub)
	client, err := NewRequestClient(nil, pub)
	if err != nil {
		t.Fatal(err)
		return
	}
	req := map[string]any{
		"test": 123,
	}
	nonce := grand.S(16)
	secret := grand.S(16)
	request, err := client.buildRequest(req, nonce, secret)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("req:\n%s", request)

	header, pl, err := ParseRequest(request)
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("Header=%+v", header)
	t.Logf("payload=%+v", pl)
	block, _ := pem.Decode([]byte(pri))
	if block == nil {
		t.Fatal("invalid private key")
		return
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
		return
	}
	response, err := HandleRequest(context.Background(), header, privateKey, pl, func(ctx context.Context, req map[string]any) (resp map[string]any, err error) {
		t.Logf("[handler] req=%+v", req)
		return map[string]any{
			"ok": true,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	rsp := map[string]any{}
	err = client.decode(response, secret, nonce, &rsp)
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Logf("rsp=%+v", rsp)
}
