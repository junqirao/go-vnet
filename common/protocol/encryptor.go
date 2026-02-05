package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// Chacha20Poly1305NonceSize is the size of nonce for chacha20-poly1305
	Chacha20Poly1305NonceSize = 24
	// Chacha20Poly1305KeySize is the size of key for chacha20-poly1305
	Chacha20Poly1305KeySize = 32
	// Chacha20Poly1305Overhead is the overhead added by chacha20-poly1305 (authentication tag)
	Chacha20Poly1305Overhead = 16
)

var (
	// TestNonce is a fixed nonce for testing purposes only
	// WARNING: In production, never reuse the same nonce with the same key
	TestNonce = [Chacha20Poly1305NonceSize]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
)

var (
	// ErrInvalidCiphertext is returned when decryption fails due to authentication
	ErrInvalidCiphertext = errors.New("invalid ciphertext: authentication failed")
)

// Chacha20Poly1305Encryptor implements Encryptor interface using chacha20-poly1305 AEAD
type Chacha20Poly1305Encryptor struct {
	aead  any
	nonce [Chacha20Poly1305NonceSize]byte
}

// NewChacha20Poly1305Encryptor creates a new chacha20-poly1305 encryptor with given key
// The key must be exactly 32 bytes
func NewChacha20Poly1305Encryptor(key []byte) (*Chacha20Poly1305Encryptor, error) {
	if len(key) != Chacha20Poly1305KeySize {
		return nil, errors.New("key must be exactly 32 bytes")
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	return &Chacha20Poly1305Encryptor{
		aead:  aead,
		nonce: TestNonce, // Use fixed nonce for now
	}, nil
}

// seal is a helper method to call Seal on the internal AEAD
func (e *Chacha20Poly1305Encryptor) seal(dst, nonce, plaintext []byte) []byte {
	aead := e.aead.(interface {
		Seal(dst, nonce, plaintext, additionalData []byte) []byte
	})
	return aead.Seal(dst, nonce, plaintext, nil)
}

// open is a helper method to call Open on the internal AEAD
func (e *Chacha20Poly1305Encryptor) open(dst, nonce, ciphertext []byte) ([]byte, error) {
	aead := e.aead.(interface {
		Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
	})
	return aead.Open(dst, nonce, ciphertext, nil)
}

// overhead returns the overhead of the AEAD
func (e *Chacha20Poly1305Encryptor) overhead() int {
	aead := e.aead.(interface {
		Overhead() int
	})
	return aead.Overhead()
}

// Encrypt encrypts data using chacha20-poly1305
// buf can be used to avoid allocation
// Returns ciphertext with nonce prepended: [nonce 24 bytes][ciphertext]
func (e *Chacha20Poly1305Encryptor) Encrypt(data []byte, buf []byte) ([]byte, error) {
	// Prepend nonce to ciphertext
	totalLen := Chacha20Poly1305NonceSize + len(data) + e.overhead()

	// Use provided buf or allocate new slice
	var result []byte
	if cap(buf) >= totalLen {
		result = buf[:totalLen]
	} else {
		result = make([]byte, totalLen)
	}

	// Copy nonce to result
	copy(result[:Chacha20Poly1305NonceSize], e.nonce[:])

	// Encrypt using AEAD
	e.seal(result[Chacha20Poly1305NonceSize:Chacha20Poly1305NonceSize], e.nonce[:], data)

	return result, nil
}

// Decrypt decrypts data using chacha20-poly1305
// Expects data format: [nonce 24 bytes][ciphertext]
// buf can be used to avoid allocation
func (e *Chacha20Poly1305Encryptor) Decrypt(data []byte, buf []byte) ([]byte, error) {
	if len(data) < Chacha20Poly1305NonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce
	nonce := data[:Chacha20Poly1305NonceSize]
	ciphertext := data[Chacha20Poly1305NonceSize:]

	// Decrypt using AEAD
	// Use empty slice with capacity to allow AEAD to allocate in provided buffer
	var dst []byte
	if len(buf) > 0 {
		// Create an empty slice with the same capacity to avoid appending to existing data
		dst = buf[:0:cap(buf)]
	}
	plaintext, err := e.open(dst, nonce, ciphertext)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	// Ensure plaintext is not nil
	if plaintext == nil {
		return []byte{}, nil
	}

	return plaintext, nil
}

// SetNonce sets a new nonce for the encryptor
// WARNING: Never reuse the same nonce with the same key
func (e *Chacha20Poly1305Encryptor) SetNonce(nonce []byte) error {
	if len(nonce) != Chacha20Poly1305NonceSize {
		return errors.New("nonce must be exactly 24 bytes")
	}
	copy(e.nonce[:], nonce)
	return nil
}

// GenerateRandomNonce generates a random nonce for production use
func GenerateRandomNonce() ([Chacha20Poly1305NonceSize]byte, error) {
	var nonce [Chacha20Poly1305NonceSize]byte
	_, err := rand.Read(nonce[:])
	return nonce, err
}

// GenerateRandomKey generates a random 32-byte key for chacha20-poly1305
func GenerateRandomKey() ([Chacha20Poly1305KeySize]byte, error) {
	var key [Chacha20Poly1305KeySize]byte
	_, err := rand.Read(key[:])
	return key, err
}

// BuildNonce builds a nonce from a 64-bit counter
// Useful for generating nonces in a deterministic way
func BuildNonce(counter uint64) [Chacha20Poly1305NonceSize]byte {
	var nonce [Chacha20Poly1305NonceSize]byte
	binary.BigEndian.PutUint64(nonce[:8], counter)
	// Remaining 16 bytes can be set to zero or used for additional context
	return nonce
}
