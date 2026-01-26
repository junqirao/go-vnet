package protocol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

type ZSTDCompressWrapper struct {
	*TransportOptions

	upstream io.ReadWriteCloser
	bufioR   *bufio.Reader
	bufioW   *bufio.Writer

	encoder *zstd.Encoder
	decoder *zstd.Decoder

	// Buffer for reading protocol header and compressed data
	header [7]byte
	buffer [65530]byte

	// Reusable sliceReader to avoid allocation
	sliceR sliceReader
}

var (
	// Pre-computed magic for fast comparison
	compressMagicBytes = [4]byte{0x56, 0x4E, 0x45, 0x54}

	// Sync pool for encoder buffers
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 65530)
		},
	}
)

var (
	ErrInvalidCompressMagic = errors.New("invalid compress magic number")
	ErrCompressDataTooLarge = errors.New("compress data too large")
)

// NewZSTDCompressWrapper creates a new ZSTD compression wrapper
// It wraps an io.ReadWriteCloser and provides transparent compression
// | magic 4 bytes | type 1 byte (TypeCompress) | length 2 byte | data n byte |
// magic: "VNET" (0x56 0x4E 0x45 0x54) - unique protocol identifier
func NewZSTDCompressWrapper(upstream io.ReadWriteCloser, opt *TransportOptions) *ZSTDCompressWrapper {
	// Use faster compression level (SpeedBetterCompression) for better performance
	var encoder *zstd.Encoder
	if opt.compress.enable {
		encoder, _ = zstd.NewWriter(nil,
			zstd.WithEncoderConcurrency(1),
			zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	}
	decoder, _ := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1))

	return &ZSTDCompressWrapper{
		TransportOptions: opt,
		upstream:         upstream,
		bufioR:           bufio.NewReaderSize(upstream, 65535),
		bufioW:           bufio.NewWriterSize(upstream, 65535),
		encoder:          encoder,
		decoder:          decoder,
	}
}

// Read reads decompressed data from upstream
// Protocol: | magic 4 bytes | type 1 byte (TypeCompress) | length 2 byte | data n byte |
// If type is TypeCompress, data is compressed and will be decompressed
// If type is not TypeCompress, data is returned as-is
func (z *ZSTDCompressWrapper) Read(p []byte) (n int, err error) {
	// Read protocol header (magic 4 bytes + type 1 byte + length 2 bytes = 7 bytes)
	if _, err = io.ReadFull(z.bufioR, z.header[:]); err != nil {
		return 0, err
	}

	// Validate magic number using direct array comparison
	if *(*[4]byte)(z.header[:4]) != compressMagicBytes {
		return 0, ErrInvalidCompressMagic
	}

	// Check type - handle both compressed and uncompressed data
	isCompressed := z.header[4] == TypeCompress

	// Read data length
	length := binary.BigEndian.Uint16(z.header[5:7])
	if length > 65530 {
		return 0, ErrCompressDataTooLarge
	}

	if isCompressed {
		// Read compressed data into buffer
		if _, err = io.ReadFull(z.bufioR, z.buffer[:length]); err != nil {
			return 0, err
		}

		// Reset and reuse the decoder
		_ = z.decoder.Reset(io.NopCloser(z.sliceR.reset(z.buffer[:length])))

		// Read decompressed data
		return z.decoder.Read(p)
	}

	// For uncompressed data, read header and data and return them together
	// Ensure buffer is large enough for header + data
	totalLen := 7 + int(length)
	if len(p) < totalLen {
		return 0, io.ErrShortBuffer
	}

	// Copy header to output
	copy(p, z.header[:7])

	// Read data directly into p (after header)
	n, err = io.ReadFull(z.bufioR, p[7:totalLen])
	return 7 + n, err
}

// sliceReader implements io.Reader for a byte slice (reusable)
type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) reset(data []byte) *sliceReader {
	r.data = data
	r.pos = 0
	return r
}

func (r *sliceReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Write writes compressed data to upstream
// Protocol: | magic 4 bytes | type 1 byte (TypeCompress) | length 2 byte | data n byte |
func (z *ZSTDCompressWrapper) Write(p []byte) (n int, err error) {
	// Not enabled
	if z.encoder == nil || len(p) < z.compress.threshold {
		return z.upstream.Write(p)
	}
	// Get buffer from pool for compression
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)

	// Compress the data (reusing pooled buffer)
	compressedData := z.encoder.EncodeAll(p, buf[:0])
	compressedLen := len(compressedData)
	if compressedLen > 65530 {
		return 0, ErrCompressDataTooLarge
	}

	// Build protocol header directly in z.header
	*(*[4]byte)(z.header[:4]) = compressMagicBytes
	z.header[4] = TypeCompress
	binary.BigEndian.PutUint16(z.header[5:7], uint16(compressedLen))

	// Write header and compressed data in a single write
	// Use buffered writer for small writes
	if _, err = z.bufioW.Write(z.header[:]); err != nil {
		return 0, err
	}

	if _, err = z.bufioW.Write(compressedData[:compressedLen]); err != nil {
		return 0, err
	}

	// Flush buffer to ensure data is sent
	if err = z.bufioW.Flush(); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close flushes any buffered data and closes the encoder, decoder, and underlying upstream
func (z *ZSTDCompressWrapper) Close() error {
	// Flush any buffered data before closing
	if z.bufioW != nil {
		_ = z.bufioW.Flush()
	}

	// Close encoder
	if z.encoder != nil {
		_ = z.encoder.Close()
	}

	// Close decoder
	if z.decoder != nil {
		z.decoder.Close()
	}
	return z.upstream.Close()
}

func (z *ZSTDCompressWrapper) Upstream() io.ReadWriteCloser {
	return z.upstream
}
