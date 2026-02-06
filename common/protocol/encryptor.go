package protocol

import (
	"crypto/rand"
	"errors"
	"fmt"

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
	// ErrInvalidCiphertext is returned when decryption fails due to authentication
	ErrInvalidCiphertext = errors.New("invalid ciphertext: authentication failed")
)

// Chacha20Poly1305Encryptor implements Encryptor interface using chacha20-poly1305 AEAD
type Chacha20Poly1305Encryptor struct {
	aead any
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
		aead: aead,
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
// buf can be used to avoid allocation, must have capacity >= Chacha20Poly1305NonceSize + len(data) + Chacha20Poly1305Overhead
// Writes ciphertext with random nonce prepended: [nonce 24 bytes][ciphertext]
// Returns number of bytes written to buf
func (e *Chacha20Poly1305Encryptor) Encrypt(data []byte, buf []byte) (int, error) {
	// Prepend nonce to ciphertext
	totalLen := Chacha20Poly1305NonceSize + len(data) + e.overhead()

	// Check buffer capacity
	if cap(buf) < totalLen {
		return 0, fmt.Errorf("buffer capacity too small %d:%d+%d+%d", cap(buf), Chacha20Poly1305NonceSize, len(data), e.overhead())
	}

	// Ensure buffer has correct length
	result := buf[:totalLen]

	// Generate random nonce directly in result buffer to avoid extra allocation
	_, err := rand.Read(result[:Chacha20Poly1305NonceSize])
	if err != nil {
		return 0, err
	}

	// Encrypt using AEAD
	e.seal(result[Chacha20Poly1305NonceSize:Chacha20Poly1305NonceSize], result[:Chacha20Poly1305NonceSize], data)

	return totalLen, nil
}

// Decrypt decrypts data using chacha20-poly1305
// Expects data format: [nonce 24 bytes][ciphertext]
// buf can be used to avoid allocation, must have capacity >= len(data) - Chacha20Poly1305NonceSize - Chacha20Poly1305Overhead
// Returns number of bytes written to buf
func (e *Chacha20Poly1305Encryptor) Decrypt(data []byte, buf []byte) (int, error) {
	if len(data) < Chacha20Poly1305NonceSize {
		return 0, ErrInvalidCiphertext
	}

	// Extract nonce and ciphertext
	nonce := data[:Chacha20Poly1305NonceSize]
	ciphertext := data[Chacha20Poly1305NonceSize:]

	// Decrypt using AEAD directly without pre-sizing the result slice
	// This allows the AEAD implementation to handle the memory allocation efficiently
	plaintext, err := e.open(buf[:0], nonce, ciphertext)
	if err != nil {
		return 0, ErrInvalidCiphertext
	}

	// If AEAD allocated a new slice, copy it back to the provided buffer
	if cap(buf) >= len(plaintext) {
		if len(plaintext) > 0 {
			// Check if plaintext uses a different underlying array
			if len(buf) == 0 || &plaintext[0] != &buf[0] {
				// Copy to buffer if it's a different underlying array
				copy(buf, plaintext)
			}
		}
		return len(plaintext), nil
	}

	// Buffer is too small, return error
	return 0, errors.New("buffer capacity too small")
}
