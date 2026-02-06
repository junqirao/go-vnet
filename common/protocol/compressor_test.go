package protocol

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"
)

func TestNewZstdCompressor(t *testing.T) {
	tests := []struct {
		name    string
		level   int
		wantErr bool
	}{
		{
			name:    "default level",
			level:   DefaultCompressionLevel,
			wantErr: false,
		},
		{
			name:    "fastest level",
			level:   FastestCompressionLevel,
			wantErr: false,
		},
		{
			name:    "best level",
			level:   BestCompressionLevel,
			wantErr: false,
		},
		{
			name:    "level too low",
			level:   0,
			wantErr: true,
		},
		{
			name:    "level too high",
			level:   20,
			wantErr: true,
		},
		{
			name:    "negative level",
			level:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewZstdCompressor(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewZstdCompressor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewDefaultZstdCompressor(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}
	if compressor == nil {
		t.Fatal("NewDefaultZstdCompressor() returned nil")
	}
}

func TestZstdCompressor_Compress(t *testing.T) {
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
			name: "single byte",
			data: []byte("a"),
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
		{
			name: "highly compressible data",
			data: bytes.Repeat([]byte("AAAA"), 500),
		},
		{
			name: "random data",
			data: generateRandomBytes(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Allocate buffer with sufficient capacity
			buf := make([]byte, 0, len(tt.data)+ZstdOverhead)
			compressedLen, err := compressor.Compress(tt.data, buf)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}

			compressed := buf[:compressedLen]

			// Compressed data should not be larger than original + overhead
			if len(compressed) > len(tt.data)+ZstdOverhead {
				t.Errorf("Compress() len = %v, should be <= %v", len(compressed), len(tt.data)+ZstdOverhead)
			}

			// Compressed length should match what we got
			if compressedLen != len(compressed) {
				t.Errorf("Compress() length mismatch: returned %v, actual %v", compressedLen, len(compressed))
			}
		})
	}
}

func TestZstdCompressor_Compress_BufferTooSmall(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name        string
		data        []byte
		bufferCap   int
		expectError bool
	}{
		{
			name:        "buffer exactly data size (no overhead)",
			data:        bytes.Repeat([]byte("test"), 100),
			bufferCap:   400,
			expectError: true,
		},
		{
			name:        "buffer with overhead",
			data:        bytes.Repeat([]byte("test"), 100),
			bufferCap:   400 + ZstdOverhead,
			expectError: false,
		},
		{
			name:        "buffer slightly smaller than needed",
			data:        bytes.Repeat([]byte("test"), 100),
			bufferCap:   400 + ZstdOverhead - 1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 0, tt.bufferCap)
			compressedLen, err := compressor.Compress(tt.data, buf)
			if tt.expectError {
				if err == nil {
					t.Errorf("Compress() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Compress() unexpected error = %v", err)
				}
				compressed := buf[:compressedLen]
				// Verify we can decompress it
				decompressBuf := make([]byte, 0, len(tt.data))
				decompressedLen, err := compressor.Decompress(compressed, decompressBuf)
				if err != nil {
					t.Errorf("Decompress() error = %v", err)
				}
				decompressed := decompressBuf[:decompressedLen]
				if !bytes.Equal(decompressed, tt.data) {
					t.Errorf("Decompress() data mismatch")
				}
			}
		})
	}
}

func TestZstdCompressor_Decompress(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "single byte",
			data: []byte("a"),
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
		{
			name: "highly compressible data",
			data: bytes.Repeat([]byte("AAAA"), 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First compress the data
			compressBuf := make([]byte, 0, len(tt.data)+ZstdOverhead)
			compressedLen, err := compressor.Compress(tt.data, compressBuf)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			compressed := compressBuf[:compressedLen]

			// Now decompress it
			decompressBuf := make([]byte, 0, len(tt.data))
			decompressedLen, err := compressor.Decompress(compressed, decompressBuf)
			if err != nil {
				t.Fatalf("Decompress() error = %v", err)
			}
			decompressed := decompressBuf[:decompressedLen]

			// Verify decompressed data matches original
			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("Decompress() data = %v, want %v", decompressed, tt.data)
			}

			// Verify decompressed length matches
			if decompressedLen != len(tt.data) {
				t.Errorf("Decompress() len = %v, want %v", decompressedLen, len(tt.data))
			}
		})
	}
}

func TestZstdCompressor_Decompress_InvalidData(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "invalid zstd data",
			data: []byte{0xFF, 0xFE, 0xFD, 0xFC},
		},
		{
			name: "truncated data",
			data: []byte{0x28, 0xB5, 0x2F, 0xFD},
		},
		{
			name: "corrupted header",
			data: bytes.Repeat([]byte{0xAA}, 100),
		},
		{
			name: "empty compressed data",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 0, 1000)
			_, err := compressor.Decompress(tt.data, buf)
			if err == nil {
				t.Errorf("Decompress() expected error for invalid data, got nil")
			}
			if err != ErrDecompressionFailed {
				t.Errorf("Decompress() error = %v, want %v", err, ErrDecompressionFailed)
			}
		})
	}
}

func TestZstdCompressor_RoundTrip(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "random data 1KB",
			data: generateRandomBytes(1024),
		},
		{
			name: "random data 10KB",
			data: generateRandomBytes(10240),
		},
		{
			name: "all zeros",
			data: bytes.Repeat([]byte{0x00}, 1000),
		},
		{
			name: "all ones",
			data: bytes.Repeat([]byte{0xFF}, 1000),
		},
		{
			name: "mixed data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
		{
			name: "repeated pattern",
			data: bytes.Repeat([]byte{0xAA, 0xBB, 0xCC, 0xDD}, 256),
		},
		{
			name: "ascii text",
			data: []byte("The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			compressBuf := make([]byte, 0, len(tt.data)+ZstdOverhead)
			compressedLen, err := compressor.Compress(tt.data, compressBuf)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			compressed := compressBuf[:compressedLen]

			// Decompress
			decompressBuf := make([]byte, 0, len(tt.data))
			decompressedLen, err := compressor.Decompress(compressed, decompressBuf)
			if err != nil {
				t.Fatalf("Decompress() error = %v", err)
			}
			decompressed := decompressBuf[:decompressedLen]

			// Verify round-trip
			if !bytes.Equal(decompressed, tt.data) {
				t.Errorf("RoundTrip failed: got len=%d, want len=%d", len(decompressed), len(tt.data))
			}
		})
	}
}

func TestZstdCompressor_CompressionLevels(t *testing.T) {
	data := bytes.Repeat([]byte("test"), 250) // 1KB of data

	for level := FastestCompressionLevel; level <= BestCompressionLevel; level++ {
		t.Run(fmt.Sprintf("level_%d", level), func(t *testing.T) {
			compressor, err := NewZstdCompressor(level)
			if err != nil {
				t.Fatalf("NewZstdCompressor(%d) error = %v", level, err)
			}

			compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
			compressedLen, err := compressor.Compress(data, compressBuf)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			compressed := compressBuf[:compressedLen]

			// Verify decompression works
			decompressBuf := make([]byte, 0, len(data))
			decompressedLen, err := compressor.Decompress(compressed, decompressBuf)
			if err != nil {
				t.Fatalf("Decompress() error = %v", err)
			}
			decompressed := decompressBuf[:decompressedLen]

			if !bytes.Equal(decompressed, data) {
				t.Errorf("RoundTrip failed for level %d", level)
			}
		})
	}
}

func TestZstdCompressor_ZeroAllocation(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	data := bytes.Repeat([]byte("test"), 250) // 1KB

	// Warm up
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	for i := 0; i < 10; i++ {
		compressor.Compress(data, compressBuf)
	}

	// Test for zero allocation
	allocs := testing.AllocsPerRun(100, func() {
		buf := make([]byte, 0, len(data)+ZstdOverhead)
		compressor.Compress(data, buf)
	})

	// Compress should not allocate beyond the initial buffer
	if allocs > 1 {
		t.Errorf("Compress() allocated %v times, expected <= 1", allocs)
	}
}

func TestZstdCompressor_Concurrent(t *testing.T) {
	compressor, err := NewDefaultZstdCompressor()
	if err != nil {
		t.Fatalf("NewDefaultZstdCompressor() error = %v", err)
	}

	data := bytes.Repeat([]byte("test"), 100)
	done := make(chan bool, 10)

	// Run multiple compressions concurrently
	for i := 0; i < 10; i++ {
		go func() {
			compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
			compressedLen, err := compressor.Compress(data, compressBuf)
			if err != nil {
				t.Errorf("Compress() error = %v", err)
			}
			compressed := compressBuf[:compressedLen]

			decompressBuf := make([]byte, 0, len(data))
			decompressedLen, err := compressor.Decompress(compressed, decompressBuf)
			if err != nil {
				t.Errorf("Decompress() error = %v", err)
			}
			decompressed := decompressBuf[:decompressedLen]

			if !bytes.Equal(decompressed, data) {
				t.Errorf("Concurrent round-trip failed")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Helper function to generate random bytes
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Benchmark tests

func BenchmarkZstdCompressor_Compress_Small(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := []byte("hello world")
	buf := make([]byte, 0, len(data)+ZstdOverhead)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(data, buf)
	}
}

func BenchmarkZstdCompressor_Compress_Medium(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 100)
	buf := make([]byte, 0, len(data)+ZstdOverhead)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(data, buf)
	}
}

func BenchmarkZstdCompressor_Compress_Large(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("data"), 1000)
	buf := make([]byte, 0, len(data)+ZstdOverhead)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(data, buf)
	}
}

func BenchmarkZstdCompressor_Compress_10KB(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 2500)
	buf := make([]byte, 0, len(data)+ZstdOverhead)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(data, buf)
	}
}

func BenchmarkZstdCompressor_Compress_Random(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := generateRandomBytes(1024)
	buf := make([]byte, 0, len(data)+ZstdOverhead)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Compress(data, buf)
	}
}

func BenchmarkZstdCompressor_Decompress_Small(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := []byte("hello world")
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	compressedLen, _ := compressor.Compress(data, compressBuf)
	compressed := compressBuf[:compressedLen]
	decompressBuf := make([]byte, 0, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Decompress(compressed, decompressBuf)
	}
}

func BenchmarkZstdCompressor_Decompress_Medium(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 100)
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	compressedLen, _ := compressor.Compress(data, compressBuf)
	compressed := compressBuf[:compressedLen]
	decompressBuf := make([]byte, 0, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Decompress(compressed, decompressBuf)
	}
}

func BenchmarkZstdCompressor_Decompress_Large(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("data"), 1000)
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	compressedLen, _ := compressor.Compress(data, compressBuf)
	compressed := compressBuf[:compressedLen]
	decompressBuf := make([]byte, 0, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Decompress(compressed, decompressBuf)
	}
}

func BenchmarkZstdCompressor_Decompress_10KB(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 2500)
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	compressedLen, _ := compressor.Compress(data, compressBuf)
	compressed := compressBuf[:compressedLen]
	decompressBuf := make([]byte, 0, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = compressor.Decompress(compressed, decompressBuf)
	}
}

func BenchmarkZstdCompressor_RoundTrip_Medium(b *testing.B) {
	compressor, _ := NewDefaultZstdCompressor()
	data := bytes.Repeat([]byte("test"), 100)
	compressBuf := make([]byte, 0, len(data)+ZstdOverhead)
	decompressBuf := make([]byte, 0, len(data))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		compressedLen, _ := compressor.Compress(data, compressBuf)
		compressed := compressBuf[:compressedLen]
		_, _ = compressor.Decompress(compressed, decompressBuf)
	}
}

func BenchmarkZstdCompressor_CompressionLevels(b *testing.B) {
	data := bytes.Repeat([]byte("test"), 250)

	for level := FastestCompressionLevel; level <= BestCompressionLevel; level++ {
		b.Run(fmt.Sprintf("level_%d", level), func(b *testing.B) {
			compressor, _ := NewZstdCompressor(level)
			buf := make([]byte, 0, len(data)+ZstdOverhead)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = compressor.Compress(data, buf)
			}
		})
	}
}
