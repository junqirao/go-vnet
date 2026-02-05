package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/crypto/gaes"
	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/util/grand"
)

type (
	payload struct {
		Request map[string]any `json:"request"`
		Secret  string         `json:"secret"`
	}
	Header struct {
		Timestamp   int64  `json:"timestamp"`
		Nonce       string `json:"nonce"`
		SubDeviceId uint64 `json:"sub_device_id"`
		Key         string `json:"key"`
	}
	RequestClient struct {
		pub *rsa.PublicKey
	}
	RequestHandler func(ctx context.Context, req map[string]any) (resp map[string]any, err error)
)

func NewHeader(subDeviceId uint64, key string) Header {
	return Header{
		Timestamp:   time.Now().Unix(),
		Nonce:       grand.S(16),
		SubDeviceId: subDeviceId,
		Key:         key,
	}
}

func NewRequestClient(publicKeyPEM string) (c *RequestClient, err error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		err = errors.New("invalid public key pem")
		return
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return
	}
	c = &RequestClient{
		pub: key.(*rsa.PublicKey),
	}
	return
}

func (c *RequestClient) Do(ctx context.Context, sr SendReceiveCloser, header Header, req map[string]any, decodeTo any) (err error) {
	secret := grand.S(16)
	request, err := c.buildRequest(req, header, secret)
	if err != nil {
		return
	}

	if err = sr.Send(request); err != nil {
		return err
	}
	receive, err := sr.Receive(ctx)
	if err != nil {
		return
	}

	return c.decode(receive, secret, header.Nonce, &decodeTo)
}

func (c *RequestClient) decode(receive []byte, secret, nonce string, ptr any) (err error) {
	// split
	parts := strings.Split(string(receive), ".")
	if len(parts) != 3 {
		err = errors.New("invalid data format")
		return
	}

	// checksum
	if gmd5.MustEncryptString(fmt.Sprintf("%s.%s", parts[0], parts[1])) != parts[2] {
		err = errors.New("failed to checksum")
		return
	}

	encrypted := gbase64.MustDecodeString(parts[1])

	// decrypt
	raw, err := gaes.DecryptCBC(encrypted, []byte(secret), []byte(nonce))
	if err != nil {
		return
	}
	err = json.Unmarshal(raw, ptr)
	return
}

func (c *RequestClient) buildRequest(req map[string]any, h Header, secret string) (data []byte, err error) {
	pl := payload{
		Request: req,
		Secret:  secret,
	}

	builder := strings.Builder{}
	builder.WriteString(toBase64(h))
	builder.WriteString(".")

	encrypted, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		c.pub,
		[]byte(toBase64(pl)),
		nil,
	)
	builder.WriteString(base64.StdEncoding.EncodeToString(encrypted))
	checksum := gmd5.MustEncryptString(builder.String())
	builder.WriteString(".")
	builder.WriteString(checksum)
	data = []byte(builder.String())
	return
}

func toBase64(m any) (data string) {
	bs, _ := json.Marshal(m)
	data = base64.RawStdEncoding.EncodeToString(bs)
	return
}

func HandleRequest(ctx context.Context, header *Header, pri *rsa.PrivateKey, pl string, handler RequestHandler) (data []byte, err error) {
	p, err := decrypt(pri, pl)
	if err != nil {
		return
	}

	resp, err := handler(ctx, p.Request)
	if err != nil {
		return
	}

	// refresh timestamp
	header.Timestamp = time.Now().Unix()
	data, err = encode(header, p.Secret, resp)
	return
}

func ParseRequest(receive []byte) (h *Header, payload string, err error) {
	// split
	parts := strings.Split(string(receive), ".")
	if len(parts) != 3 {
		err = errors.New("invalid data format")
		return
	}

	// checksum
	if gmd5.MustEncryptString(fmt.Sprintf("%s.%s", parts[0], parts[1])) != parts[2] {
		err = errors.New("failed to checksum")
		return
	}

	// base64ToPtr
	h = &Header{}
	if err = base64ToPtr(parts[0], h); err != nil {
		err = errors.New("failed to decode Header")
		return
	}
	payload = parts[1]
	return
}

func decrypt(pri *rsa.PrivateKey, encryptedPayload string) (p *payload, err error) {
	// decrypt
	p = &payload{}
	decrypted, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		pri,
		gbase64.MustDecodeString(encryptedPayload),
		nil,
	)
	if err = base64ToPtr(string(decrypted), p); err != nil {
		err = errors.New("failed to decode payload")
		return
	}
	return
}

func base64ToPtr(data string, ptr any) (err error) {
	bs, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return
	}
	err = json.Unmarshal(bs, &ptr)
	return
}

func encode(h *Header, secret string, resp any) (res []byte, err error) {
	bs, err := json.Marshal(resp)
	if err != nil {
		return
	}
	encrypted, err := gaes.EncryptCBC(bs, []byte(secret), []byte(h.Nonce))
	if err != nil {
		return
	}

	builder := strings.Builder{}
	builder.WriteString(toBase64(h))
	builder.WriteString(".")
	builder.WriteString(gbase64.EncodeToString(encrypted))
	checksum := gmd5.MustEncryptString(builder.String())
	builder.WriteString(".")
	builder.WriteString(checksum)
	res = []byte(builder.String())
	return
}
