package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// mockReadWriter implements io.ReadWriter for testing
type mockReadWriter struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func newMockReadWriter() *mockReadWriter {
	return &mockReadWriter{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	return m.readBuf.Read(p)
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	return m.writeBuf.Write(p)
}

func buildMessage(typ byte, data []byte) []byte {
	// Build message with VNET magic
	length := len(data)
	msg := make([]byte, 7+length)
	msg[0] = 0x56
	msg[1] = 0x4E
	msg[2] = 0x45
	msg[3] = 0x54
	msg[4] = typ
	binary.BigEndian.PutUint16(msg[5:7], uint16(length))
	copy(msg[7:], data)
	return msg
}

func buildRawMessage(data []byte) []byte {
	return buildMessage(0, data)
}

func TestNewTransport(t *testing.T) {
	mock := newMockReadWriter()
	transport := NewTransport(mock)
	if transport == nil {
		t.Fatal("NewTransport returned nil")
	}
}

func TestTransport_Read(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name: "simple message",
			data: []byte("hello"),
		},
		{
			name: "large message",
			data: bytes.Repeat([]byte("test"), 1000),
		},
		{
			name: "empty data",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			msg := buildRawMessage(tt.data)
			mock.readBuf.Write(msg)

			transport := NewTransport(mock)
			out := make([]byte, len(tt.data))
			n, err := transport.Read(out)

			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expectedLen := len(tt.data)
				if n != expectedLen {
					t.Errorf("Read() n = %v, want %v", n, expectedLen)
				}
				// Output should be: data only (skip magic, type and length bytes)
				if !bytes.Equal(out[:n], tt.data) {
					t.Errorf("Read() output = %v, want %v", out[:n], tt.data)
				}
			}
		})
	}
}

func TestTransport_Read_InvalidMagic(t *testing.T) {
	mock := newMockReadWriter()
	// Create a message with invalid magic
	msg := []byte{0x00, 0x02, 0x03, 0x04, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}
	mock.readBuf.Write(msg)

	transport := NewTransport(mock)
	out := make([]byte, 10)
	_, err := transport.Read(out)

	if err == nil {
		t.Error("Read() should return error for invalid magic number")
	}
}

func TestTransport_Read_TooLarge(t *testing.T) {
	mock := newMockReadWriter()
	// Create a message with invalid length
	msg := []byte{0x56, 0x4E, 0x45, 0x54, 0x02, 0xFF, 0xFF} // length = 65535 > 65530
	mock.readBuf.Write(msg)

	transport := NewTransport(mock)
	out := make([]byte, 10)
	_, err := transport.Read(out)

	if err == nil {
		t.Error("Read() should return error for too large message")
	}
}

func TestTransport_Write(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name: "simple message",
			data: []byte("hello"),
		},
		{
			name: "large message",
			data: bytes.Repeat([]byte("test"), 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			transport := NewTransport(mock)

			n, err := transport.Write(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expectedMsg := buildRawMessage(tt.data)
				if n != len(expectedMsg) {
					t.Errorf("Write() n = %v, want %v", n, len(expectedMsg))
				}
				if !bytes.Equal(mock.writeBuf.Bytes(), expectedMsg) {
					t.Errorf("Write() output = %v, want %v", mock.writeBuf.Bytes(), expectedMsg)
				}
			}
		})
	}
}

func TestTransport_Write_TooLarge(t *testing.T) {
	mock := newMockReadWriter()
	transport := NewTransport(mock)

	// Data too large (> 65531 bytes)
	data := bytes.Repeat([]byte("a"), 65532)
	_, err := transport.Write(data)

	if err == nil {
		t.Error("Write() should return error for too large message")
	}
}

func TestTransport_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "small message",
			data: []byte("test data"),
		},
		{
			name: "medium message",
			data: bytes.Repeat([]byte("hello"), 100),
		},
		{
			name: "large message",
			data: bytes.Repeat([]byte("world"), 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			transport := NewTransport(mock)

			// Write raw data (will be wrapped in protocol)
			_, err := transport.Write(tt.data)
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			// Move written data to read buffer
			mock.readBuf = mock.writeBuf

			// Read message back (will be unwrapped)
			out := make([]byte, len(tt.data))
			n, err := transport.Read(out)
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}

			// Verify we get back the same data (without header)
			if n != len(tt.data) {
				t.Errorf("RoundTrip length failed: got %v, want %v", n, len(tt.data))
			}
			if !bytes.Equal(out[:n], tt.data) {
				t.Errorf("RoundTrip data failed: got %v, want %v", out[:n], tt.data)
			}
		})
	}
}

func TestTransport_PrintEncodedData(t *testing.T) {
	mock := newMockReadWriter()
	transport := NewTransport(mock).(*Transport)

	// Test data
	data := []byte("hello world")

	t.Logf("Original data: %v", string(data))
	t.Logf("Original data bytes: %v", data)

	// Write data (will be encoded with protocol)
	n, err := transport.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Get the encoded data from write buffer
	encoded := mock.writeBuf.Bytes()
	t.Logf("Encoded data length: %d bytes", n)
	t.Logf("Encoded data bytes: %v", encoded)

	// Parse and print each field
	magic := encoded[0:4]
	typ := encoded[4]
	length := binary.BigEndian.Uint16(encoded[5:7])
	payload := encoded[7:]

	t.Logf("Magic bytes: %v (%c%c%c%c)", magic, magic[0], magic[1], magic[2], magic[3])
	t.Logf("Type byte: 0x%02X", typ)
	t.Logf("Length: %d", length)
	t.Logf("Payload data: %v", string(payload))
	t.Logf("Payload bytes: %v", payload)

	// Verify encoding
	if magic[0] != 0x56 || magic[1] != 0x4E ||
		magic[2] != 0x45 || magic[3] != 0x54 {
		t.Errorf("Expected magic VNET, got %c%c%c%c", magic[0], magic[1], magic[2], magic[3])
	}
	if typ != 0 {
		t.Errorf("Expected type 0, got %d", typ)
	}
	if length != uint16(len(data)) {
		t.Errorf("Expected length %d, got %d", len(data), length)
	}
	if !bytes.Equal(payload, data) {
		t.Errorf("Expected payload %v, got %v", data, payload)
	}
}

func TestTransport_ReadMessage(t *testing.T) {
	tests := []struct {
		name    string
		magic   [4]byte
		typ     byte
		data    []byte
		wantErr bool
	}{
		{
			name:  "simple message",
			magic: [4]byte{0x56, 0x4E, 0x45, 0x54},
			typ:   0x02,
			data:  []byte("hello"),
		},
		{
			name:  "large message",
			magic: [4]byte{0x56, 0x4E, 0x45, 0x54},
			typ:   0xAA,
			data:  bytes.Repeat([]byte("test"), 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			msg := buildMessage(tt.typ, tt.data)
			mock.readBuf.Write(msg)

			transport := NewTransport(mock).(*Transport)
			dataBuf := make([]byte, len(tt.data))
			typ, n, err := transport.ReadMessage(dataBuf)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if typ != tt.typ {
					t.Errorf("ReadMessage() typ = %v, want %v", typ, tt.typ)
				}
				if n != len(tt.data) {
					t.Errorf("ReadMessage() n = %v, want %v", n, len(tt.data))
				}
				if !bytes.Equal(dataBuf[:n], tt.data) {
					t.Errorf("ReadMessage() data = %v, want %v", dataBuf[:n], tt.data)
				}
			}
		})
	}
}

func TestTransport_WriteMessage(t *testing.T) {
	tests := []struct {
		name    string
		typ     byte
		data    []byte
		wantErr bool
	}{
		{
			name: "simple message",
			typ:  0x02,
			data: []byte("hello"),
		},
		{
			name: "large message",
			typ:  0xAA,
			data: bytes.Repeat([]byte("test"), 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			transport := NewTransport(mock).(*Transport)

			n, err := transport.WriteMessage(tt.typ, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("WriteMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				expectedMsg := buildMessage(tt.typ, tt.data)
				if n != len(expectedMsg) {
					t.Errorf("WriteMessage() n = %v, want %v", n, len(expectedMsg))
				}
				if !bytes.Equal(mock.writeBuf.Bytes(), expectedMsg) {
					t.Errorf("WriteMessage() output = %v, want %v", mock.writeBuf.Bytes(), expectedMsg)
				}
			}
		})
	}
}

func TestTransport_WriteMessage_TooLarge(t *testing.T) {
	mock := newMockReadWriter()
	transport := NewTransport(mock).(*Transport)

	// Data too large (> 65531 bytes)
	data := bytes.Repeat([]byte("a"), 65532)
	_, err := transport.WriteMessage(0x02, data)

	if err == nil {
		t.Error("WriteMessage() should return error for too large message")
	}
}

func TestTransport_ReadMessage_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		typ  byte
		data []byte
	}{
		{
			name: "small message",
			typ:  0x02,
			data: []byte("test data"),
		},
		{
			name: "large message",
			typ:  0x06,
			data: bytes.Repeat([]byte("world"), 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			transport := NewTransport(mock).(*Transport)

			// Write message
			_, err := transport.WriteMessage(tt.typ, tt.data)
			if err != nil {
				t.Fatalf("WriteMessage() error = %v", err)
			}

			// Move written data to read buffer
			mock.readBuf = mock.writeBuf

			// Read message back
			dataBuf := make([]byte, len(tt.data))
			typ, n, err := transport.ReadMessage(dataBuf)
			if err != nil {
				t.Fatalf("ReadMessage() error = %v", err)
			}

			if typ != tt.typ {
				t.Errorf("RoundTrip typ = %v, want %v", typ, tt.typ)
			}
			if !bytes.Equal(dataBuf[:n], tt.data) {
				t.Errorf("RoundTrip data = %v, want %v", dataBuf[:n], tt.data)
			}
		})
	}
}

// Benchmark tests
func BenchmarkTransport_Read_Small(b *testing.B) {
	mock := newMockReadWriter()
	data := []byte("hello world")
	msg := buildRawMessage(data)

	transport := NewTransport(mock)
	out := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, err := transport.Read(out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_Read_Medium(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 100)
	msg := buildRawMessage(data)

	transport := NewTransport(mock)
	out := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, err := transport.Read(out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_Read_Large(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 1000)
	msg := buildRawMessage(data)

	transport := NewTransport(mock)
	out := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, err := transport.Read(out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_Write_Small(b *testing.B) {
	mock := newMockReadWriter()
	data := []byte("hello world")

	transport := NewTransport(mock)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_Write_Medium(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 100)

	transport := NewTransport(mock)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_Write_Large(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 1000)

	transport := NewTransport(mock)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.Write(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_ReadMessage_Small(b *testing.B) {
	mock := newMockReadWriter()
	data := []byte("hello world")
	msg := buildMessage(0x02, data)

	transport := NewTransport(mock).(*Transport)
	dataBuf := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, _, err := transport.ReadMessage(dataBuf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_ReadMessage_Medium(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 100)
	msg := buildMessage(0x02, data)

	transport := NewTransport(mock).(*Transport)
	dataBuf := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, _, err := transport.ReadMessage(dataBuf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_ReadMessage_Large(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 1000)
	msg := buildMessage(0x02, data)

	transport := NewTransport(mock).(*Transport)
	dataBuf := make([]byte, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.readBuf.Reset()
		mock.readBuf.Write(msg)
		_, _, err := transport.ReadMessage(dataBuf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_WriteMessage_Small(b *testing.B) {
	mock := newMockReadWriter()
	transport := NewTransport(mock).(*Transport)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.WriteMessage(0x02, []byte("hello world"))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_WriteMessage_Medium(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 100)
	transport := NewTransport(mock).(*Transport)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.WriteMessage(0x02, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_WriteMessage_Large(b *testing.B) {
	mock := newMockReadWriter()
	data := bytes.Repeat([]byte("test"), 1000)
	transport := NewTransport(mock).(*Transport)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock.writeBuf.Reset()
		_, err := transport.WriteMessage(0x02, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_RoundTrip_Small(b *testing.B) {
	data := []byte("hello world")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriter()
		transport := NewTransport(mock).(*Transport)

		// Write
		_, err := transport.Write(data)
		if err != nil {
			b.Fatal(err)
		}

		// Move to read buffer
		mock.readBuf = mock.writeBuf

		// Read
		out := make([]byte, len(data))
		_, err = transport.Read(out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport_RoundTrip_Large(b *testing.B) {
	data := bytes.Repeat([]byte("test"), 1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		mock := newMockReadWriter()
		transport := NewTransport(mock).(*Transport)

		// Write
		_, err := transport.Write(data)
		if err != nil {
			b.Fatal(err)
		}

		// Move to read buffer
		mock.readBuf = mock.writeBuf

		// Read
		out := make([]byte, len(data))
		_, err = transport.Read(out)
		if err != nil {
			b.Fatal(err)
		}
	}
}
