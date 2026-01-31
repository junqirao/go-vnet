package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
)

type (
	simplePasswordEncoder struct {
		password string
	}
)

func NewSimplePasswordEncoder(password string) Encoder {
	return &simplePasswordEncoder{password: password}
}

func (s simplePasswordEncoder) Encode(ctx context.Context, payload map[string]any) (data []byte, err error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// 生成密码的哈希作为加密密钥
	keyHash := sha256.Sum256([]byte(s.password))

	encrypted := make([]byte, len(jsonData))
	for i, b := range jsonData {
		encrypted[i] = b ^ keyHash[i%len(keyHash)]
	}

	return encrypted, nil
}

func (s simplePasswordEncoder) Decode(ctx context.Context, in []byte) (payload map[string]any, err error) {
	result := make(map[string]any)
	return result, s.DecodeTo(ctx, in, &result)
}

func (s simplePasswordEncoder) DecodeTo(_ context.Context, in []byte, ptr any) (err error) {
	// 生成密码的哈希作为解密密钥
	keyHash := sha256.Sum256([]byte(s.password))

	decrypted := make([]byte, len(in))
	for i, b := range in {
		decrypted[i] = b ^ keyHash[i%len(keyHash)]
	}

	err = json.Unmarshal(decrypted, &ptr)
	return
}
