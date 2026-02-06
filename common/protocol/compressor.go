package protocol

import (
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	// DefaultCompressionLevel is the default zstd compression level (2 - default/balanced)
	DefaultCompressionLevel = 2
	// FastestCompressionLevel is the fastest zstd compression level (1)
	FastestCompressionLevel = 1
	// BestCompressionLevel is the best zstd compression level (4)
	BestCompressionLevel = 4
	// ZstdOverhead is the maximum additional bytes that compression may add to the data
	// When data cannot be compressed, zstd may store the original data with frame headers
	// Maximum frame header size is 17 bytes (14 + 3 from zstd specification)
	ZstdOverhead = 17
)

var (
	// ErrDecompressionFailed is returned when decompression fails
	ErrDecompressionFailed = errors.New("decompression failed")
)

// ZstdCompressor implements Compressor interface using zstd compression
// Uses sync.Pool to reuse encoders and decoders for zero-allocation performance
type ZstdCompressor struct {
	encoderPool      *sync.Pool
	decoderPool      *sync.Pool
	compressionLevel int
}

// NewZstdCompressor creates a new zstd compressor with specified compression level
// ranges from 1 (fastest) to 19 (best compression), 3 is recommended for most use cases
// Returns error if compression level is invalid
func NewZstdCompressor(level int) (*ZstdCompressor, error) {
	if level < FastestCompressionLevel || level > BestCompressionLevel {
		return nil, errors.New("compression level must be between 1 and 19")
	}

	c := &ZstdCompressor{
		compressionLevel: level,
	}

	// Initialize encoder pool
	c.encoderPool = &sync.Pool{
		New: func() interface{} {
			encoder, err := zstd.NewWriter(nil,
				zstd.WithEncoderLevel(zstd.EncoderLevel(level)),
				zstd.WithEncoderConcurrency(1), // Use single goroutine for minimal overhead
				zstd.WithZeroFrames(true),      // Use zero frames for better performance
			)
			if err != nil {
				panic(err)
			}
			return encoder
		},
	}

	// Initialize decoder pool
	c.decoderPool = &sync.Pool{
		New: func() interface{} {
			decoder, err := zstd.NewReader(nil,
				zstd.WithDecoderConcurrency(1),   // Use single goroutine for minimal overhead
				zstd.WithDecoderMaxMemory(1<<30), // Max 1GB memory for decompression
			)
			if err != nil {
				panic(err)
			}
			return decoder
		},
	}

	return c, nil
}

// NewDefaultZstdCompressor creates a new zstd compressor with default compression level (3)
func NewDefaultZstdCompressor() (*ZstdCompressor, error) {
	return NewZstdCompressor(DefaultCompressionLevel)
}

// Compress compresses data into buf, returns number of bytes written
// buf must have sufficient capacity: cap(buf) >= len(data) + ZstdOverhead
// ZstdOverhead (17 bytes) accounts for the maximum frame header size when data cannot be compressed
// Uses sync.Pool to reuse encoder and avoid allocations
func (c *ZstdCompressor) Compress(data []byte, buf []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// Check buffer capacity
	requiredCapacity := len(data) + ZstdOverhead
	if cap(buf) < requiredCapacity {
		return 0, fmt.Errorf("buffer capacity too small: got %d, need at least %d (data %d + overhead %d)",
			cap(buf), requiredCapacity, len(data), ZstdOverhead)
	}

	// Get encoder from pool
	encoder := c.encoderPool.Get().(*zstd.Encoder)
	defer c.encoderPool.Put(encoder)

	// EncodeAll directly to the provided buffer slice
	// This is the most efficient way - no intermediate allocation
	compressed := encoder.EncodeAll(data, buf[:0])

	// If the encoder didn't use the provided buffer, copy the data
	// This happens when the provided buffer is too small
	if len(compressed) > 0 && cap(buf) >= len(compressed) {
		if len(compressed) > 0 {
			// Check if compressed uses a different underlying array
			if len(buf) == 0 || &compressed[0] != &buf[0] {
				// Copy to buffer if it's a different underlying array
				copy(buf, compressed)
			}
		}
		return len(compressed), nil
	}

	// Buffer is too small, return error
	return 0, fmt.Errorf("buffer capacity too small: got %d, need at least %d",
		cap(buf), len(compressed))
}

// Decompress decompresses data into buf, returns number of bytes written
// buf must have sufficient capacity to hold decompressed data
// Uses sync.Pool to reuse decoder and avoid allocations
func (c *ZstdCompressor) Decompress(data []byte, buf []byte) (int, error) {
	if len(data) == 0 {
		return 0, ErrDecompressionFailed
	}

	// Get decoder from pool
	decoder := c.decoderPool.Get().(*zstd.Decoder)
	defer c.decoderPool.Put(decoder)

	// DecodeAll directly to the provided buffer slice
	// We use the buffer slice directly to avoid allocation
	decompressed, err := decoder.DecodeAll(data, buf[:0])
	if err != nil {
		return 0, ErrDecompressionFailed
	}

	// If the decoder didn't use the provided buffer, copy the data
	// This happens when the provided buffer is too small
	if len(decompressed) > 0 && cap(buf) >= len(decompressed) {
		if len(decompressed) > 0 {
			// Check if decompressed uses a different underlying array
			if len(buf) == 0 || &decompressed[0] != &buf[0] {
				// Copy to buffer if it's a different underlying array
				copy(buf, decompressed)
			}
		}
		return len(decompressed), nil
	}

	// Buffer is too small, return error
	return 0, errors.New("buffer capacity too small for decompressed data")
}
