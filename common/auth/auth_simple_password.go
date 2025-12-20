package auth

import (
	"context"
	"encoding/json"
	"fmt"
)

type (
	SimplePasswordAuthenticator struct {
		password string
		salt     string
	}
	simplePasswordAuthPacket struct {
		Payload   map[string]any `json:"payload"`
		Signature string         `json:"signature"`
	}
)

func NewSimplePasswordAuthenticator(password string, md5salt ...string) AuthorizedHandler {
	salt := ""
	if len(md5salt) > 0 && md5salt[0] != "" {
		salt = md5salt[0]
	}
	return &SimplePasswordAuthenticator{
		password: password,
		salt:     salt,
	}
}

func (s *SimplePasswordAuthenticator) Make(_ context.Context, payload ...map[string]any) (data []byte, err error) {
	pkg := simplePasswordAuthPacket{}
	if len(payload) > 0 {
		pkg.Payload = payload[0]
	}
	pkg.Signature = fmt.Sprintf("%x", []byte(fmt.Sprintf("%s%s%s", s.password, pkg.Payload, s.salt)))
	data, err = json.Marshal(pkg)
	return
}

func (s *SimplePasswordAuthenticator) Handle(_ context.Context, in []byte) (payload map[string]any, err error) {
	pkg := simplePasswordAuthPacket{}
	if err = json.Unmarshal(in, &pkg); err != nil {
		return
	}
	if pkg.Signature != fmt.Sprintf("%x", []byte(fmt.Sprintf("%s%s%s", s.password, pkg.Payload, s.salt))) {
		err = fmt.Errorf("signature not match")
		return
	}
	payload = pkg.Payload
	return
}
