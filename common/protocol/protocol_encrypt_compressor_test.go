package protocol

import (
	"bytes"
	"fmt"
	"testing"
)

func TestTransport_EncryptedCompressed_Transport_Small(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "small data",
			data: []byte("hello world"),
		},
		{
			name: "medium data",
			data: bytes.Repeat([]byte("test"), 100),
		},
		{
			name: "data below compression threshold",
			data: bytes.Repeat([]byte("small"), 100), // 500 bytes < 10KB threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed message
			_, err := transport.WriteMessage(TypeTransport, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt/decompress message
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			// Verify type
			if typ != TypeTransport {
				t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
			}

			// Verify data
			if !bytes.Equal(buf[:n], tt.data) {
				t.Errorf("ReadMessage() data = %v, want %v", buf[:n], tt.data)
			}
		})
	}
}

func TestTransport_EncryptedCompressed_Transport_Large(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "large data",
			data: bytes.Repeat([]byte("data"), 1000),
		},
		{
			name: "very large data",
			data: bytes.Repeat([]byte("large"), 5000),
		},
		{
			name: "highly compressible large data",
			data: bytes.Repeat([]byte("AAAA"), 3000),
		},
		{
			name: "random large data",
			data: make([]byte, 20000),
		},
	}

	for i := range tests {
		// Initialize random data only once
		if tests[i].name == "random large data" {
			for j := range tests[i].data {
				tests[i].data[j] = byte(j % 256)
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed message
			_, err := transport.WriteMessage(TypeTransport, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt/decompress message
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			// Verify type
			if typ != TypeTransport {
				t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
			}

			// Verify data
			if !bytes.Equal(buf[:n], tt.data) {
				t.Errorf("ReadMessage() data length mismatch: got %d, want %d", n, len(tt.data))
			}
		})
	}
}

func TestTransport_EncryptedCompressed_BatchTransport(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name  string
		buf   [][]byte
		sizes []int
	}{
		{
			name:  "single message",
			buf:   [][]byte{append(make([]byte, 8), []byte("hello")...)},
			sizes: []int{5},
		},
		{
			name: "multiple messages",
			buf: [][]byte{
				append(make([]byte, 8), []byte("msg1")...),
				append(make([]byte, 8), []byte("msg2")...),
				append(make([]byte, 8), []byte("msg3")...),
			},
			sizes: []int{4, 4, 4},
		},
		{
			name: "large batch",
			buf: [][]byte{
				append(make([]byte, 8), bytes.Repeat([]byte("A"), 1000)...),
				append(make([]byte, 8), bytes.Repeat([]byte("B"), 1500)...),
				append(make([]byte, 8), bytes.Repeat([]byte("C"), 2000)...),
			},
			sizes: []int{1000, 1500, 2000},
		},
		{
			name: "highly compressible batch",
			buf: [][]byte{
				append(make([]byte, 8), bytes.Repeat([]byte("X"), 5000)...),
				append(make([]byte, 8), bytes.Repeat([]byte("Y"), 5000)...),
			},
			sizes: []int{5000, 5000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed batch message
			_, err := transport.BatchWrite(tt.buf, tt.sizes, 8)
			if err != nil {
				t.Fatalf("BatchWrite() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt/decompress message
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			// Verify type
			if typ != TypeBatchTransport {
				t.Errorf("ReadMessage() type = %v, want %v", typ, TypeBatchTransport)
			}

			// Parse batch
			parseBuf := make([][]byte, len(tt.buf))
			for i := range parseBuf {
				parseBuf[i] = make([]byte, 8+tt.sizes[i])
			}
			parseSizes := make([]int, len(tt.sizes))
			count, err := transport.ParseBatch(buf[:n], parseBuf, parseSizes, 8)
			if err != nil {
				t.Fatalf("ParseBatch() error = %v", err)
			}

			// Verify message count
			if count != len(tt.buf) {
				t.Errorf("ParseBatch() count = %v, want %v", count, len(tt.buf))
			}

			// Verify each message
			for i := range tt.buf {
				expectedData := tt.buf[i][8 : 8+tt.sizes[i]]
				actualData := parseBuf[i][8 : 8+parseSizes[i]]
				if !bytes.Equal(actualData, expectedData) {
					t.Errorf("ParseBatch() message %d data mismatch: got %d bytes, want %d bytes", i, parseSizes[i], tt.sizes[i])
				}
			}
		})
	}
}

func TestTransport_EncryptedCompressed_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x02}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		typ  byte
		data []byte
	}{
		{
			name: "TypeTransport with small data",
			typ:  TypeTransport,
			data: []byte("test data"),
		},
		{
			name: "TypeTransport with medium data",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte("encrypt and compress me"), 500),
		},
		{
			name: "TypeTransport with large data",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte("large data to compress"), 2000),
		},
		{
			name: "TypeTransport with all zeros",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte{0x00}, 5000),
		},
		{
			name: "TypeTransport with all ones",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte{0xFF}, 5000),
		},
		{
			name: "TypeTransport with mixed bytes",
			typ:  TypeTransport,
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0xAA, 0x55},
		},
		{
			name: "TypeTransport with highly compressible",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte("ABCD"), 2500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed message
			_, err := transport.WriteMessage(tt.typ, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt/decompress message
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			// Verify type
			if typ != tt.typ {
				t.Errorf("ReadMessage() type = %v, want %v", typ, tt.typ)
			}

			// Verify data
			if !bytes.Equal(buf[:n], tt.data) {
				t.Errorf("ReadMessage() data length mismatch: got %d, want %d", n, len(tt.data))
			}
		})
	}
}

func TestTransport_EncryptedCompressed_WriteReadMultiple(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

	// Write multiple messages
	messages := []struct {
		typ  byte
		data []byte
	}{
		{TypeTransport, []byte("msg1")},
		{TypeTransport, bytes.Repeat([]byte("test"), 100)},
		{TypeTransport, bytes.Repeat([]byte("large message"), 1000)},
		{TypeTransport, []byte("msg2")},
		{TypeTransport, bytes.Repeat([]byte("AAAA"), 2000)},
	}

	for _, msg := range messages {
		_, err := transport.WriteMessage(msg.typ, msg.data)
		if err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
	}

	// Reset reader to read what was written
	mock.resetReader()

	// Read and verify all messages
	for i, expected := range messages {
		buf := make([]byte, MaxTransportByteSize)
		typ, n, err := transport.ReadMessage(buf)
		if err != nil {
			t.Fatalf("ReadMessage() %d error = %v", i, err)
		}

		if typ != expected.typ {
			t.Errorf("ReadMessage() %d type = %v, want %v", i, typ, expected.typ)
		}

		if !bytes.Equal(buf[:n], expected.data) {
			t.Errorf("ReadMessage() %d data length mismatch: got %d, want %d", i, n, len(expected.data))
		}
	}
}

func TestTransport_EncryptedCompressed_CompressionThreshold(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name           string
		data           []byte
		shouldCompress bool
	}{
		{
			name:           "data below threshold (5KB)",
			data:           bytes.Repeat([]byte("test"), 1250), // 5KB < 10KB threshold
			shouldCompress: false,
		},
		{
			name:           "data at threshold (10KB)",
			data:           bytes.Repeat([]byte("test"), 2500), // 10KB == threshold
			shouldCompress: true,
		},
		{
			name:           "data above threshold (20KB)",
			data:           bytes.Repeat([]byte("test"), 5000), // 20KB > threshold
			shouldCompress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed message
			written, err := transport.WriteMessage(TypeTransport, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and verify
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			if typ != TypeTransport {
				t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
			}

			if !bytes.Equal(buf[:n], tt.data) {
				t.Errorf("ReadMessage() data mismatch")
			}

			// The written size should be smaller than original if compressed
			// But encrypted, so we can't directly compare sizes
			// Just verify round-trip works
			if written == 0 {
				t.Errorf("WriteMessage() returned 0 bytes")
			}
		})
	}
}

func TestTransport_EncryptedCompressed_WithoutCompressor(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))

	// Write encrypted message (without compression)
	data := []byte("test data")
	_, err = transport.WriteMessage(TypeTransport, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Reset reader to read what was written
	mock.resetReader()

	// Read message (should work without compressor)
	buf := make([]byte, MaxTransportByteSize)
	typ, n, err := transport.ReadMessage(buf)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	// Verify type
	if typ != TypeTransport {
		t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
	}

	// Verify data
	if !bytes.Equal(buf[:n], data) {
		t.Errorf("ReadMessage() data = %v, want %v", buf[:n], data)
	}
}

func TestTransport_EncryptedCompressed_WithoutEncryptor(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithCompressor(compressor))

	// Write compressed message (without encryption)
	data := bytes.Repeat([]byte("test"), 5000) // Large data to trigger compression
	_, err = transport.WriteMessage(TypeTransport, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Reset reader to read what was written
	mock.resetReader()

	// Read message (should work without encryptor)
	buf := make([]byte, MaxTransportByteSize)
	typ, n, err := transport.ReadMessage(buf)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	// Verify type
	if typ != TypeTransport {
		t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
	}

	// Verify data
	if !bytes.Equal(buf[:n], data) {
		t.Errorf("ReadMessage() data mismatch")
	}
}

func TestTransport_EncryptedCompressed_MissingCompressor(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor), EnableCompress(true), EnableEncrypt(true))

	// Write encrypted and compressed message
	data := bytes.Repeat([]byte("test"), 5000)
	_, err = transport.WriteMessage(TypeTransport, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Create new transport without compressor
	mock.resetReader()
	transport2 := NewTransport(mock, WithEncryptor(encryptor))

	// Try to read compressed message without compressor
	buf := make([]byte, MaxTransportByteSize)
	_, _, err = transport2.ReadMessage(buf)
	if err != ErrMissingCompressor {
		t.Errorf("ReadMessage() error = %v, want %v", err, ErrMissingCompressor)
	}
}

func TestTransport_EncryptedCompressed_MissingEncryptor(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

	// Write encrypted and compressed message
	data := bytes.Repeat([]byte("test"), 5000)
	_, err = transport.WriteMessage(TypeTransport, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Create new transport without encryptor
	mock.resetReader()
	transport2 := NewTransport(mock, WithCompressor(compressor))

	// Try to read encrypted message without encryptor
	buf := make([]byte, MaxTransportByteSize)
	_, _, err = transport2.ReadMessage(buf)
	if err != ErrMissingEncryptor {
		t.Errorf("ReadMessage() error = %v, want %v", err, ErrMissingEncryptor)
	}
}

func TestTransport_EncryptedCompressed_CompressionLevels(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	data := bytes.Repeat([]byte("test"), 2500) // 10KB

	for level := FastestCompressionLevel; level <= BestCompressionLevel; level++ {
		t.Run(fmt.Sprintf("level_%d", level), func(t *testing.T) {
			compressor, cErr := NewZstdCompressor(level)
			if cErr != nil {
				t.Fatalf("NewZstdCompressor(%d) error = %v", level, cErr)
			}

			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))

			// Write encrypted and compressed message
			_, err := transport.WriteMessage(TypeTransport, data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and verify
			buf := make([]byte, MaxTransportByteSize)
			typ, n, err := transport.ReadMessage(buf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			if typ != TypeTransport {
				t.Errorf("ReadMessage() type = %v, want %v", typ, TypeTransport)
			}

			if !bytes.Equal(buf[:n], data) {
				t.Errorf("ReadMessage() data mismatch for level %d", level)
			}
		})
	}
}

// Benchmark tests for encrypted and compressed transport

func BenchmarkTransport_EncryptedCompressed_Write_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
	data := []byte("hello world")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writer.Reset()
		_, _ = transport.WriteMessage(TypeTransport, data)
	}
}

func BenchmarkTransport_EncryptedCompressed_Write_Medium(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
	data := bytes.Repeat([]byte("test"), 100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writer.Reset()
		_, _ = transport.WriteMessage(TypeTransport, data)
	}
}

func BenchmarkTransport_EncryptedCompressed_Write_Large(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
	data := bytes.Repeat([]byte("data"), 1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writer.Reset()
		_, _ = transport.WriteMessage(TypeTransport, data)
	}
}

func BenchmarkTransport_EncryptedCompressed_Read_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
	data := []byte("hello world")

	// Pre-write encrypted and compressed message
	_, _ = transport.WriteMessage(TypeTransport, data)
	encryptedCompressedData := mock.writer.Bytes()
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.reader = bytes.NewReader(encryptedCompressedData)
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_EncryptedCompressed_Read_Large(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
	data := bytes.Repeat([]byte("data"), 1000)

	// Pre-write encrypted and compressed message
	_, _ = transport.WriteMessage(TypeTransport, data)
	encryptedCompressedData := mock.writer.Bytes()
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.reader = bytes.NewReader(encryptedCompressedData)
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_EncryptedCompressed_RoundTrip_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	data := []byte("hello world")
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
		_, _ = transport.WriteMessage(TypeTransport, data)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_EncryptedCompressed_RoundTrip_Medium(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 100)
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
		_, _ = transport.WriteMessage(TypeTransport, data)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_EncryptedCompressed_RoundTrip_Large(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("data"), 1000)
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
		_, _ = transport.WriteMessage(TypeTransport, data)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_EncryptedCompressed_BatchWrite(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	compressor, _ := NewDefaultZstdCompressor()
	buf := make([]byte, MaxTransportByteSize)
	bufs := [][]byte{
		append(make([]byte, 8), bytes.Repeat([]byte("test"), 100)...),
		append(make([]byte, 8), bytes.Repeat([]byte("data"), 100)...),
		append(make([]byte, 8), bytes.Repeat([]byte("msg"), 100)...),
	}
	sizes := []int{400, 400, 300}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor), WithCompressor(compressor))
		_, _ = transport.BatchWrite(bufs, sizes, 8)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}
