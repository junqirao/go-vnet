package auth

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
)

type rsaEncoder struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

type RSAEncoderOption func(*rsaEncoder)

// WithPrivateKey sets the RSA private key for the encoder
func WithPrivateKey(privateKeyPEM string) RSAEncoderOption {
	return func(r *rsaEncoder) {
		block, _ := pem.Decode([]byte(privateKeyPEM))
		if block != nil {
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				r.privateKey = key
				r.publicKey = &key.PublicKey
			}
		}
	}
}

// WithPublicKey sets the RSA public key for the encoder
func WithPublicKey(publicKeyPEM string) RSAEncoderOption {
	return func(r *rsaEncoder) {
		block, _ := pem.Decode([]byte(publicKeyPEM))
		if block != nil {
			if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
				if rsaKey, ok := key.(*rsa.PublicKey); ok {
					r.publicKey = rsaKey
				}
			}
		}
	}
}

func NewRsaEncoder(opts ...RSAEncoderOption) Encoder {
	encoder := &rsaEncoder{}

	// Generate a new key pair if no keys are provided
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err == nil {
		encoder.privateKey = privateKey
		encoder.publicKey = &privateKey.PublicKey
	}

	// Apply options
	for _, opt := range opts {
		opt(encoder)
	}

	return encoder
}

func (r rsaEncoder) Encode(ctx context.Context, payload map[string]any) (data []byte, err error) {
	if r.publicKey == nil {
		return nil, errors.New("RSA public key not available")
	}

	// Serialize payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// RSA encryption with OAEP padding for better security
	encrypted, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		r.publicKey,
		jsonData,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Return base64 encoded encrypted data for safe transport
	base64Encoded := base64.StdEncoding.EncodeToString(encrypted)
	return []byte(base64Encoded), nil
}

func (r rsaEncoder) Decode(ctx context.Context, in []byte) (payload map[string]any, err error) {
	if r.privateKey == nil {
		return nil, errors.New("RSA private key not available")
	}

	// Decode base64 encrypted data
	encrypted, err := base64.StdEncoding.DecodeString(string(in))
	if err != nil {
		return nil, err
	}

	// RSA decryption with OAEP padding
	decrypted, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		r.privateKey,
		encrypted,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize JSON to payload
	var result map[string]any
	err = json.Unmarshal(decrypted, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetPublicKeyPEM returns the public key in PEM format
func (r rsaEncoder) GetPublicKeyPEM() (string, error) {
	if r.publicKey == nil {
		return "", errors.New("public key not available")
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(r.publicKey)
	if err != nil {
		return "", err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(publicKeyPEM), nil
}

// GetPrivateKeyPEM returns the private key in PEM format
func (r rsaEncoder) GetPrivateKeyPEM() (string, error) {
	if r.privateKey == nil {
		return "", errors.New("private key not available")
	}

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(r.privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return string(privateKeyPEM), nil
}
