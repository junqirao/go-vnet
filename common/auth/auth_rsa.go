package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// RSAConfig holds the configuration for RSA authentication
type RSAConfig struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// RSAAuthenticator implements the AuthorizedHandler interface using RSA encryption
type RSAAuthenticator struct {
	config *RSAConfig
}

// NewRSAAuthenticator creates a new RSA authenticator with the given configuration
func NewRSAAuthenticator(config *RSAConfig) (*RSAAuthenticator, error) {
	if config == nil || config.PrivateKey == nil || config.PublicKey == nil {
		return nil, errors.New("both private and public keys are required")
	}
	return &RSAAuthenticator{config: config}, nil
}

// GenerateRSAKeyPair generates a new RSA key pair with the specified bit size (e.g., 2048, 4096)
func GenerateRSAKeyPair(bitSize int) (*RSAConfig, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	return &RSAConfig{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// Make encrypts the payload using RSA-OAEP and returns the encrypted data
func (a *RSAAuthenticator) Make(ctx context.Context, payload ...map[string]any) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("no payload provided")
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload[0])
	if err != nil {
		return nil, err
	}

	// Encrypt the data using RSA-OAEP
	hash := sha256.New()
	ciphertext, err := rsa.EncryptOAEP(
		hash,
		rand.Reader,
		a.config.PublicKey,
		jsonData,
		nil, // label
	)
	if err != nil {
		return nil, err
	}

	// Encode the encrypted data as base64 for safe transport
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)

	return encoded, nil
}

// Handle decrypts the incoming data using RSA-OAEP and returns the decrypted payload
func (a *RSAAuthenticator) Handle(ctx context.Context, in []byte) (map[string]any, error) {
	if len(in) == 0 {
		return nil, errors.New("empty input data")
	}

	// Decode the base64-encoded ciphertext
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	n, err := base64.StdEncoding.Decode(decoded, in)
	if err != nil {
		return nil, err
	}
	decoded = decoded[:n]

	// Decrypt the data using RSA-OAEP
	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(
		hash,
		rand.Reader,
		a.config.PrivateKey,
		decoded,
		nil, // label
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON payload
	var payload map[string]any
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// EncodePublicKeyToString encodes the public key to a PEM string
func (a *RSAAuthenticator) EncodePublicKeyToString() (string, error) {
	if a.config.PublicKey == nil {
		return "", errors.New("public key not set")
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(a.config.PublicKey)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pubASN1), nil
}

// DecodePublicKeyFromString decodes a public key from a base64-encoded string
func DecodePublicKeyFromString(pubKeyStr string) (*rsa.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(pubKeyStr)
	if err != nil {
		return nil, err
	}

	pub, err := x509.ParsePKIXPublicKey(decoded)
	if err != nil {
		return nil, err
	}

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		return pub, nil
	default:
		return nil, errors.New("key type is not RSA")
	}
}
