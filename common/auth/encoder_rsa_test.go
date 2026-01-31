package auth

import (
	"context"
	"testing"
)

func TestNewRsaEncoder(t *testing.T) {
	encoder := NewRsaEncoder()

	if encoder == nil {
		t.Fatal("NewRsaEncoder returned nil")
	}

	// Type assertion to verify it's the correct type
	rsaEncoder, ok := encoder.(*rsaEncoder)
	if !ok {
		t.Fatal("NewRsaEncoder did not return *rsaEncoder")
	}

	if rsaEncoder.privateKey == nil {
		t.Error("Private key should be generated automatically")
	}

	if rsaEncoder.publicKey == nil {
		t.Error("Public key should be generated automatically")
	}
}

func TestRsaEncoder_EncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "Simple payload",
			payload: map[string]any{
				"username": "john",
				"role":     "admin",
			},
		},
		{
			name: "Complex payload",
			payload: map[string]any{
				"user_id":  12345,
				"active":   true,
				"score":    99.5,
				"tags":     []string{"go", "auth", "rsa"},
				"metadata": map[string]any{"ip": "192.168.1.1", "port": 8080},
				"null_val": nil,
			},
		},
		{
			name:    "Empty payload",
			payload: map[string]any{},
		},
		{
			name: "Unicode characters",
			payload: map[string]any{
				"name":  "张三",
				"emoji": "🚀🔐",
				"text":  "Hello 世界",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			encoder := NewRsaEncoder()

			// Test Encode
			encoded, err := encoder.Encode(ctx, tt.payload)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			if len(encoded) == 0 {
				t.Error("Encoded data should not be empty")
			}

			// Test Decode
			decoded, err := encoder.Decode(ctx, encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify decoded payload matches original
			if !equalPayloads(tt.payload, decoded) {
				t.Errorf("Decoded payload does not match original.\nExpected: %+v\nGot: %+v", tt.payload, decoded)
			}
		})
	}
}

func TestRsaEncoder_WithKeys(t *testing.T) {
	ctx := context.Background()

	// Create encoder with auto-generated keys
	encoder1 := NewRsaEncoder()

	// Get keys from first encoder
	publicKeyPEM, err := encoder1.(*rsaEncoder).GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}

	privateKeyPEM, err := encoder1.(*rsaEncoder).GetPrivateKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get private key: %v", err)
	}

	// Create second encoder with the same keys
	encoder2 := NewRsaEncoder(
		WithPublicKey(publicKeyPEM),
		WithPrivateKey(privateKeyPEM),
	)

	payload := map[string]any{
		"data": "test with custom keys",
		"id":   42,
	}

	// Encode with second encoder
	encoded, err := encoder2.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode with second encoder
	decoded, err := encoder2.Decode(ctx, encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !equalPayloads(payload, decoded) {
		t.Error("Round-trip with custom keys failed")
	}
}

func TestRsaEncoder_PublicOnlyEncoding(t *testing.T) {
	ctx := context.Background()

	// Create encoder and get public key
	encoder1 := NewRsaEncoder()
	publicKeyPEM, err := encoder1.(*rsaEncoder).GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}

	// Create encoder with only public key
	publicEncoder := NewRsaEncoder(WithPublicKey(publicKeyPEM))

	payload := map[string]any{"message": "can only encrypt"}

	// Should be able to encode
	encoded, err := publicEncoder.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode with public key only should work: %v", err)
	}

	// Should not be able to decode without private key
	_, err = publicEncoder.Decode(ctx, encoded)
	if err == nil {
		t.Error("Decode with public key only should fail")
	}
}

func TestRsaEncoder_WrongPrivateKey(t *testing.T) {
	ctx := context.Background()

	// Create first encoder and encode data
	encoder1 := NewRsaEncoder()
	payload := map[string]any{"secret": "data"}
	encoded, err := encoder1.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Create second encoder with different keys
	encoder2 := NewRsaEncoder()

	// Try to decode with wrong private key
	_, err = encoder2.Decode(ctx, encoded)
	if err == nil {
		t.Error("Decode with wrong private key should fail")
	}
}

func TestRsaEncoder_InvalidKeyPEM(t *testing.T) {
	invalidPEM := "-----BEGIN INVALID KEY-----\ninvalid data\n-----END INVALID KEY-----"

	// Should not panic with invalid key
	encoder := NewRsaEncoder(WithPublicKey(invalidPEM), WithPrivateKey(invalidPEM))

	// Should still work with auto-generated keys
	ctx := context.Background()
	payload := map[string]any{"test": "fallback to auto-generated keys"}

	encoded, err := encoder.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := encoder.Decode(ctx, encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !equalPayloads(payload, decoded) {
		t.Error("Round-trip with invalid PEM keys failed")
	}
}

func TestRsaEncoder_EmptyInput(t *testing.T) {
	ctx := context.Background()
	encoder := NewRsaEncoder()

	// Test decoding empty input
	decoded, err := encoder.Decode(ctx, []byte{})
	if err == nil {
		t.Error("Expected error when decoding empty input, but got nil")
	}

	if decoded != nil {
		t.Error("Expected nil result when decoding empty input")
	}
}

func TestRsaEncoder_NilInput(t *testing.T) {
	ctx := context.Background()
	encoder := NewRsaEncoder()

	// Test decoding nil input
	decoded, err := encoder.Decode(ctx, nil)
	if err == nil {
		t.Error("Expected error when decoding nil input, but got nil")
	}

	if decoded != nil {
		t.Error("Expected nil result when decoding nil input")
	}
}

func TestRsaEncoder_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := map[string]any{"test": "value"}
	encoder := NewRsaEncoder()

	// Should still work as implementation doesn't check context
	encoded, err := encoder.Encode(ctx, payload)
	if err != nil {
		t.Errorf("Encode failed with cancelled context: %v", err)
	}

	decoded, err := encoder.Decode(ctx, encoded)
	if err != nil {
		t.Errorf("Decode failed with cancelled context: %v", err)
	}

	if decoded != nil && !equalPayloads(payload, decoded) {
		t.Error("Round-trip with cancelled context failed")
	}
}

func TestRsaEncoder_GetKeysPEM(t *testing.T) {
	encoder := NewRsaEncoder()

	// Test public key PEM
	publicKeyPEM, err := encoder.(*rsaEncoder).GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get public key PEM: %v", err)
	}

	if publicKeyPEM == "" {
		t.Error("Public key PEM should not be empty")
	}

	if !contains(publicKeyPEM, "-----BEGIN PUBLIC KEY-----") {
		t.Error("Public key PEM should contain proper header")
	}

	if !contains(publicKeyPEM, "-----END PUBLIC KEY-----") {
		t.Error("Public key PEM should contain proper footer")
	}

	// Test private key PEM
	privateKeyPEM, err := encoder.(*rsaEncoder).GetPrivateKeyPEM()
	if err != nil {
		t.Fatalf("Failed to get private key PEM: %v", err)
	}

	if privateKeyPEM == "" {
		t.Error("Private key PEM should not be empty")
	}

	if !contains(privateKeyPEM, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Error("Private key PEM should contain proper header")
	}

	if !contains(privateKeyPEM, "-----END RSA PRIVATE KEY-----") {
		t.Error("Private key PEM should contain proper footer")
	}
}

func TestRsaEncoder_LargePayload(t *testing.T) {
	ctx := context.Background()

	// Create a payload that's within RSA limit (RSA-2048 can encrypt about 245 bytes with OAEP)
	largePayload := map[string]any{
		"data": "This is a test string sized for RSA encryption.",
		"metadata": map[string]any{
			"source": "test",
			"type":   "validation",
		},
	}

	encoder := NewRsaEncoder()

	// Test encoding large payload
	encoded, err := encoder.Encode(ctx, largePayload)
	if err != nil {
		t.Fatalf("Encode failed for large payload: %v", err)
	}

	// Test decoding large payload
	decoded, err := encoder.Decode(ctx, encoded)
	if err != nil {
		t.Fatalf("Decode failed for large payload: %v", err)
	}

	if !equalPayloads(largePayload, decoded) {
		t.Error("Round-trip for large payload failed")
	}
}

func TestRsaEncoder_CrossCompatibility(t *testing.T) {
	ctx := context.Background()

	// Test that data encoded by one instance cannot be decoded by another with different keys
	encoder1 := NewRsaEncoder()
	encoder2 := NewRsaEncoder()

	payload := map[string]any{
		"message": "cross-compatibility test",
		"from":    "encoder1",
	}

	// Encode with encoder1
	encoded, err := encoder1.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Try to decode with encoder2 (different keys) - should fail
	_, err = encoder2.Decode(ctx, encoded)
	if err == nil {
		t.Error("Decoding with different keys should fail")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
