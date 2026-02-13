package protocol

import (
	"bytes"
	"testing"
)

// mockReadWriteCloser implements io.ReadWriteCloser for testing
type mockReadWriteCloser struct {
	reader *bytes.Reader
	writer *bytes.Buffer
}

func newMockReadWriteCloser() *mockReadWriteCloser {
	return &mockReadWriteCloser{
		reader: bytes.NewReader([]byte{}),
		writer: &bytes.Buffer{},
	}
}

func (m *mockReadWriteCloser) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

func (m *mockReadWriteCloser) Close() error {
	return nil
}

// resetReader resets the reader to read from writer's buffer
func (m *mockReadWriteCloser) resetReader() {
	m.reader = bytes.NewReader(m.writer.Bytes())
}

func TestTransport_Encrypted_Transport(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
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
			name: "large data",
			data: bytes.Repeat([]byte("data"), 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor))

			// Write encrypted message
			_, err := transport.WriteMessage(TypeTransport, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt message
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

func TestTransport_Encrypted_BatchTransport(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
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
			name: "large messages",
			buf: [][]byte{
				append(make([]byte, 8), bytes.Repeat([]byte("A"), 100)...),
				append(make([]byte, 8), bytes.Repeat([]byte("B"), 200)...),
			},
			sizes: []int{100, 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor))

			// Write encrypted batch message
			_, err := transport.BatchWrite(tt.buf, tt.sizes, 8)
			if err != nil {
				t.Fatalf("BatchWrite() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt message
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
					t.Errorf("ParseBatch() message %d data = %v, want %v", i, actualData, expectedData)
				}
			}
		})
	}
}

func TestTransport_Encrypted_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x02}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
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
			name: "TypeTransport with large data",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte("encrypt me"), 500),
		},
		{
			name: "TypeTransport with all zeros",
			typ:  TypeTransport,
			data: bytes.Repeat([]byte{0x00}, 100),
		},
		{
			name: "TypeTransport with mixed bytes",
			typ:  TypeTransport,
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0xAA, 0x55},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriteCloser()
			transport := NewTransport(mock, WithEncryptor(encryptor))

			// Write encrypted message
			_, err := transport.WriteMessage(tt.typ, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Reset reader to read what was written
			mock.resetReader()

			// Read and decrypt message
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
				t.Errorf("ReadMessage() data = %v, want %v", buf[:n], tt.data)
			}
		})
	}
}

func TestTransport_Encrypted_WithoutEncryptor(t *testing.T) {
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock)

	// Write unencrypted message
	data := []byte("test data")
	_, err := transport.WriteMessage(TypeTransport, data)
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
		t.Errorf("ReadMessage() data = %v, want %v", buf[:n], data)
	}
}

func TestTransport_Encrypted_MissingEncryptor(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))

	// Write encrypted message
	data := []byte("test data")
	_, err = transport.WriteMessage(TypeTransport, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Create new transport without encryptor
	mock.resetReader()
	transport2 := NewTransport(mock)

	// Try to read encrypted message without encryptor
	buf := make([]byte, MaxTransportByteSize)
	_, _, err = transport2.ReadMessage(buf)
	if err != ErrMissingEncryptor {
		t.Errorf("ReadMessage() error = %v, want %v", err, ErrMissingEncryptor)
	}
}

func TestTransport_Encrypted_WriteRead(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))

	// Write multiple messages
	messages := []struct {
		typ  byte
		data []byte
	}{
		{TypeTransport, []byte("msg1")},
		{TypeTransport, bytes.Repeat([]byte("test"), 100)},
		{TypeTransport, []byte("last message")},
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
			t.Errorf("ReadMessage() %d data = %v, want %v", i, buf[:n], expected.data)
		}
	}
}

func TestTransport_Encrypted_CustomType(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, err := NewChacha20Poly1305Encryptor(key)
	if err != nil {
		t.Fatalf("NewChacha20Poly1305Encryptor() error = %v", err)
	}

	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))

	// Write custom type message (should not be encrypted)
	customType := byte(0x20)
	data := []byte("custom type message")
	_, err = transport.WriteMessage(customType, data)
	if err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}

	// Reset reader to read what was written
	mock.resetReader()

	// Read message (custom types are not encrypted)
	buf := make([]byte, MaxTransportByteSize)
	typ, n, err := transport.ReadMessage(buf)
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	// Verify type (should be custom type, not encrypted)
	if typ != customType {
		t.Errorf("ReadMessage() type = %v, want %v", typ, customType)
	}

	// Verify data
	if !bytes.Equal(buf[:n], data) {
		t.Errorf("ReadMessage() data = %v, want %v", buf[:n], data)
	}
}

// Benchmark tests for encrypted transport

func BenchmarkTransport_Encrypted_Write_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))
	data := []byte("hello world")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writer.Reset()
		_, _ = transport.WriteMessage(TypeTransport, data)
	}
}

func BenchmarkTransport_Encrypted_Write_Medium(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))
	data := bytes.Repeat([]byte("test"), 100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writer.Reset()
		_, _ = transport.WriteMessage(TypeTransport, data)
	}
}

func BenchmarkTransport_Encrypted_Read_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	mock := newMockReadWriteCloser()
	transport := NewTransport(mock, WithEncryptor(encryptor))
	data := []byte("hello world")

	// Pre-write encrypted message
	_, _ = transport.WriteMessage(TypeTransport, data)
	encryptedData := mock.writer.Bytes()
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.reader = bytes.NewReader(encryptedData)
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_Encrypted_RoundTrip_Small(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	data := []byte("hello world")
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor))
		_, _ = transport.WriteMessage(TypeTransport, data)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}

func BenchmarkTransport_Encrypted_RoundTrip_Medium(b *testing.B) {
	key := bytes.Repeat([]byte{0x01}, Chacha20Poly1305KeySize)
	encryptor, _ := NewChacha20Poly1305Encryptor(key)
	data := bytes.Repeat([]byte("test"), 100)
	buf := make([]byte, MaxTransportByteSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriteCloser()
		transport := NewTransport(mock, WithEncryptor(encryptor))
		_, _ = transport.WriteMessage(TypeTransport, data)
		mock.resetReader()
		_, _, _ = transport.ReadMessage(buf)
	}
}
