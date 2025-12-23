package auth

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewSimplePasswordEncoder(t *testing.T) {
	password := "test-password"
	encoder := NewSimplePasswordEncoder(password)

	if encoder == nil {
		t.Fatal("NewSimplePasswordEncoder returned nil")
	}

	// Type assertion to verify it's the correct type
	simpleEncoder, ok := encoder.(*simplePasswordEncoder)
	if !ok {
		t.Fatal("NewSimplePasswordEncoder did not return *simplePasswordEncoder")
	}

	if simpleEncoder.password != password {
		t.Errorf("Expected password %s, got %s", password, simpleEncoder.password)
	}
}

func TestSimplePasswordEncoder_EncodeDecode(t *testing.T) {
	tests := []struct {
		name     string
		password string
		payload  map[string]any
	}{
		{
			name:     "Simple payload",
			password: "test123",
			payload: map[string]any{
				"username": "john",
				"role":     "admin",
			},
		},
		{
			name:     "Complex payload with various types",
			password: "complex-p@ssw0rd!",
			payload: map[string]any{
				"user_id":    12345,
				"active":     true,
				"score":      99.5,
				"tags":       []string{"go", "auth", "encoder"},
				"metadata":   map[string]any{"ip": "192.168.1.1", "port": 8080},
				"null_value": nil,
			},
		},
		{
			name:     "Empty payload",
			password: "empty",
			payload:  map[string]any{},
		},
		{
			name:     "Unicode characters",
			password: "密码🔐",
			payload: map[string]any{
				"name":  "张三",
				"emoji": "🚀🔥",
				"text":  "Hello 世界",
			},
		},
		{
			name:     "Long password",
			password: "this-is-a-very-long-password-with-special-chars-!@#$%^&*()_+-=[]{}|;:,.<>?",
			payload: map[string]any{
				"message": "Security test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			encoder := NewSimplePasswordEncoder(tt.password)

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

func TestSimplePasswordEncoder_DecodeWithWrongPassword(t *testing.T) {
	ctx := context.Background()
	originalPassword := "correct-password"
	wrongPassword := "wrong-password"

	payload := map[string]any{
		"data": "sensitive information",
	}

	// Encode with correct password
	encoder1 := NewSimplePasswordEncoder(originalPassword)
	encoded, err := encoder1.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Try to decode with wrong password
	encoder2 := NewSimplePasswordEncoder(wrongPassword)
	decoded, err := encoder2.Decode(ctx, encoded)

	// Should not panic, but should return an error or invalid data
	if err == nil {
		t.Error("Expected error when decoding with wrong password, but got nil")
	}

	if decoded != nil && equalPayloads(payload, decoded) {
		t.Error("Should not be able to decode correctly with wrong password")
	}
}

func TestSimplePasswordEncoder_EmptyPassword(t *testing.T) {
	ctx := context.Background()
	payload := map[string]any{"test": "value"}

	encoder := NewSimplePasswordEncoder("")

	// Should work even with empty password
	encoded, err := encoder.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed with empty password: %v", err)
	}

	decoded, err := encoder.Decode(ctx, encoded)
	if err != nil {
		t.Fatalf("Decode failed with empty password: %v", err)
	}

	if !equalPayloads(payload, decoded) {
		t.Error("Round-trip with empty password failed")
	}
}

func TestSimplePasswordEncoder_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := map[string]any{"test": "value"}
	encoder := NewSimplePasswordEncoder("test")

	// Encode should still work as the implementation doesn't check context
	encoded, err := encoder.Encode(ctx, payload)
	if err != nil {
		t.Errorf("Encode failed with cancelled context: %v", err)
	}

	// Decode should also work
	decoded, err := encoder.Decode(ctx, encoded)
	if err != nil {
		t.Errorf("Decode failed with cancelled context: %v", err)
	}

	if decoded != nil && !equalPayloads(payload, decoded) {
		t.Error("Round-trip with cancelled context failed")
	}
}

func TestSimplePasswordEncoder_EmptyInput(t *testing.T) {
	ctx := context.Background()
	encoder := NewSimplePasswordEncoder("test")

	// Test decoding empty input
	decoded, err := encoder.Decode(ctx, []byte{})
	if err == nil {
		t.Error("Expected error when decoding empty input, but got nil")
	}

	if decoded != nil {
		t.Error("Expected nil result when decoding empty input")
	}
}

func TestSimplePasswordEncoder_NilInput(t *testing.T) {
	ctx := context.Background()
	encoder := NewSimplePasswordEncoder("test")

	// Test decoding nil input
	decoded, err := encoder.Decode(ctx, nil)
	if err == nil {
		t.Error("Expected error when decoding nil input, but got nil")
	}

	if decoded != nil {
		t.Error("Expected nil result when decoding nil input")
	}
}

func TestSimplePasswordEncoder_LargePayload(t *testing.T) {
	ctx := context.Background()

	// Create a large payload
	largePayload := make(map[string]any)
	largePayload["data"] = make([]string, 1000)
	for i := 0; i < 1000; i++ {
		largePayload["data"].([]string)[i] = "This is a large string for testing purposes"
	}

	encoder := NewSimplePasswordEncoder("large-payload-test")

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

func TestSimplePasswordEncoder_SamePasswordDifferentInstances(t *testing.T) {
	ctx := context.Background()
	password := "test-password"
	payload := map[string]any{"data": "test value"}

	// Create two encoders with the same password
	encoder1 := NewSimplePasswordEncoder(password)
	encoder2 := NewSimplePasswordEncoder(password)

	// Encode with first encoder
	encoded, err := encoder1.Encode(ctx, payload)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode with second encoder
	decoded, err := encoder2.Decode(ctx, encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !equalPayloads(payload, decoded) {
		t.Error("Different instances with same password should work together")
	}
}

// Helper function to compare two payloads
func equalPayloads(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}

		if !deepEqual(aVal, bVal) {
			return false
		}
	}

	return true
}

// Helper function for deep comparison of values
func deepEqual(a, b any) bool {
	// Use JSON marshaling for reliable comparison
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)

	// If both marshal successfully, compare JSON strings
	if err1 == nil && err2 == nil {
		return string(aJSON) == string(bJSON)
	}

	// Fallback to simple equality for cases where marshaling fails
	if err1 != nil {
		return a == b
	}

	return false
}
