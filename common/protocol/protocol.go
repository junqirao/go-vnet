package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	MaxTransportByteSize = 65535
	// DefaultCompressThreshold is the default size threshold for compression
	DefaultCompressThreshold = 1024 * 10 // 10KB
)

const (
	TypeTransport      byte = 0x0
	TypeBatchTransport byte = 0x1
	TypeCompress       byte = 0x10
	TypeEncrypted      byte = 0x11
)

type (
	ReadWriter interface {
		io.ReadWriteCloser
		ReadMessage(data []byte) (typ byte, n int, err error)
		WriteMessage(typ byte, data []byte) (int, error)
		BatchWrite(buf [][]byte, sizes []int, headerSize int) (n int, err error)
		ParseBatch(data []byte, buf [][]byte, sizes []int, offset int) (n int, err error)
		Upstream() io.ReadWriter
		Type() string
	}
)

var (
	// Pre-computed magic for fast comparison
	transportMagicBytes = [4]byte{0x56, 0x4E, 0x45, 0x54}
)

var (
	ErrInvalidMagic      = errors.New("invalid magic number")
	ErrMessageTooLarge   = errors.New("message too large")
	ErrMissingCompressor = errors.New("compressor not configured")
	ErrMissingEncryptor  = errors.New("encryptor not configured")
)

type (
	// Compressor defines the compression interface
	// buf parameter allows reusing memory to avoid allocation
	Compressor interface {
		// Compress compresses data into buf, returns the compressed data slice
		Compress(data []byte, buf []byte) ([]byte, error)
		// Decompress decompresses data into buf, returns the decompressed data slice
		Decompress(data []byte, buf []byte) ([]byte, error)
	}

	// Encryptor defines the encryption interface
	// buf parameter allows reusing memory to avoid allocation
	Encryptor interface {
		// Encrypt encrypts data into buf, returns the encrypted data slice
		Encrypt(data []byte, buf []byte) ([]byte, error)
		// Decrypt decrypts data into buf, returns the decrypted data slice
		Decrypt(data []byte, buf []byte) ([]byte, error)
	}
)

type (
	// Transport protocol
	// | magic 4 bytes | type 1 byte | length 2 byte | data n byte |
	// magic: "VNET" (0x56 0x4E 0x45 0x54) - unique protocol identifier
	Transport struct {
		*TransportOptions
		upstream io.ReadWriteCloser
		buffer   *[MaxTransportByteSize]byte
		// cryptoBuffer is a separate buffer for encryption/decryption to avoid conflicts
		cryptoBuffer *[MaxTransportByteSize]byte
	}
	TransportOptions struct {
		magic             [4]byte
		typ               string
		encrypt           bool
		compress          bool
		compressor        Compressor
		encryptor         Encryptor
		compressThreshold int // threshold in bytes for compression
	}
	TransportOpt func(o *TransportOptions)
)

var (
	defaultTransportOptions = func() *TransportOptions {
		options := &TransportOptions{
			magic:             transportMagicBytes,
			compressThreshold: DefaultCompressThreshold,
		}
		return options
	}
	WithMagic = func(m [4]byte) TransportOpt {
		return func(o *TransportOptions) {
			o.magic = m
		}
	}
	WithType = func(typ string) TransportOpt {
		return func(o *TransportOptions) {
			o.typ = typ
		}
	}
	WithCompressor = func(c Compressor) TransportOpt {
		return func(o *TransportOptions) {
			o.compressor = c
			o.compress = true
		}
	}
	WithEncryptor = func(e Encryptor) TransportOpt {
		return func(o *TransportOptions) {
			o.encryptor = e
			o.encrypt = true
		}
	}
	WithCompressThreshold = func(threshold int) TransportOpt {
		return func(o *TransportOptions) {
			o.compressThreshold = threshold
		}
	}
)

// NewTransport creates a new Transport protocol read writer
func NewTransport(upstream io.ReadWriteCloser, opts ...TransportOpt) ReadWriter {
	options := defaultTransportOptions()
	for _, opt := range opts {
		opt(options)
	}
	return &Transport{
		upstream:         upstream,
		buffer:           &[MaxTransportByteSize]byte{},
		cryptoBuffer:     &[MaxTransportByteSize]byte{},
		TransportOptions: options,
	}
}

// Read reads a complete message from upstream
// | magic 4 bytes | type 1 byte | length 2 byte | data n byte |
func (t *Transport) Read(p []byte) (n int, err error) {
	// Read magic (4 bytes)
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[:4]); err != nil {
		return 0, err
	}

	// Validate magic number using direct array comparison (faster)
	if *(*[4]byte)((*t.buffer)[:4]) != t.magic {
		return 0, ErrInvalidMagic
	}

	// Read type and length (3 bytes)
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[4:7]); err != nil {
		return 0, err
	}

	length := binary.BigEndian.Uint16((*t.buffer)[5:7])
	if length > 65530 {
		return 0, ErrMessageTooLarge
	}

	// Read data
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[7:7+length]); err != nil {
		return 0, err
	}

	// Copy only data to output buffer (skip magic, type and length)
	copy(p, (*t.buffer)[7:7+length])
	return int(length), nil
}

// Write writes p as a complete message with protocol header to upstream
// | magic 4 bytes | type 1 byte | length 2 byte | data n byte |
func (t *Transport) Write(p []byte) (n int, err error) {
	return t.WriteMessage(TypeTransport, p)
}

// BatchWrite writes multiple messages in a single batch with optimized format
// generic header:  | magic 4 bytes | type 1 byte | length 2 bytes | data n bytes |
// data format: | sizes_length 2 bytes | [size_data 2 bytes]... | combined data n bytes |
func (t *Transport) BatchWrite(buf [][]byte, sizes []int, headerSize int) (n int, err error) {
	length := len(buf)

	start := 0
	nn := 0
	for start < length {
		end := start
		totalSize := 0
		// Calculate available space considering encryption overhead
		// When encrypted: outer header (7) + encryption overhead (16) + sizes_length (2)
		// When not encrypted: outer header (7) + sizes_length (2)
		baseOverhead := 7 + 2 // outer header + sizes_length
		if t.encrypt && t.encryptor != nil {
			baseOverhead += Chacha20Poly1305Overhead // +16 for encryption overhead
		}
		remaining := MaxTransportByteSize - baseOverhead
		for end < length && remaining >= (2+sizes[end]) {
			remaining -= 2 + sizes[end]
			totalSize += sizes[end]
			end++
		}

		nn, err = t.batchWrite(buf[start:end], sizes[start:end], headerSize)
		if err != nil {
			return nn, err
		}
		n += nn
		start = end
	}
	return
}

func (t *Transport) batchWrite(buf [][]byte, sizes []int, headerSize int) (n int, err error) {
	// Calculate total data length:
	// 2 bytes for sizes_length + 2*len(sizes) bytes for size_data + sum of each buf slice
	dataLen := 2 + 2*len(sizes)
	for i := range sizes {
		dataLen += sizes[i]
	}

	if dataLen > 65530 {
		return 0, ErrMessageTooLarge
	}

	// Build magic
	*(*[4]byte)((*t.buffer)[:4]) = t.magic

	// Build type and length in generic header
	(*t.buffer)[4] = TypeBatchTransport
	binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(dataLen))

	// Write sizes_length (2 bytes) in data format
	offset := 7
	binary.BigEndian.PutUint16((*t.buffer)[offset:offset+2], uint16(len(sizes)))
	offset += 2

	// Write all size_data (2 bytes each)
	for i := 0; i < len(sizes); i++ {
		binary.BigEndian.PutUint16((*t.buffer)[offset:offset+2], uint16(sizes[i]))
		offset += 2
	}

	// Write combined data from buf, skipping headerSize bytes from each
	for i := range buf {
		copy((*t.buffer)[offset:], buf[i][headerSize:headerSize+sizes[i]])
		offset += sizes[i]
	}

	// Apply encryption if enabled
	if t.encrypt && t.encryptor != nil {
		// Construct complete inner batch message: type + length + data
		innerMsgLen := 3 + dataLen
		if innerMsgLen > MaxTransportByteSize-7 {
			return 0, ErrMessageTooLarge
		}
		// Build inner message in cryptoBuffer
		innerMsg := (*t.cryptoBuffer)[7 : 7+innerMsgLen]
		innerMsg[0] = TypeBatchTransport
		binary.BigEndian.PutUint16(innerMsg[1:3], uint16(dataLen))
		copy(innerMsg[3:], (*t.buffer)[7:7+dataLen])
		// Encrypt the complete inner message
		encrypted, err := t.encryptor.Encrypt(innerMsg, (*t.cryptoBuffer)[7+innerMsgLen:])
		if err != nil {
			return 0, err
		}
		// Write encrypted message with TypeEncrypted
		*(*[4]byte)((*t.buffer)[:4]) = t.magic
		(*t.buffer)[4] = TypeEncrypted
		binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(len(encrypted)))
		copy((*t.buffer)[7:], encrypted)
		totalLen := 7 + len(encrypted)
		_, err = t.upstream.Write((*t.buffer)[:totalLen])
		return totalLen, err
	}

	// Write complete message (unencrypted)
	totalLen := 7 + dataLen
	_, err = t.upstream.Write((*t.buffer)[:totalLen])
	return totalLen, err
}

// ParseBatch parses a batch message from BatchWrite encoded data into separate messages
// offset specifies the start position in each buf slice to write data to
func (t *Transport) ParseBatch(data []byte, buf [][]byte, sizes []int, offset int) (n int, err error) {
	dataOffset := 0

	// Read sizes_length (2 bytes)
	sizesLen := int(binary.BigEndian.Uint16(data[dataOffset : dataOffset+2]))
	dataOffset += 2

	// Read all size_data (2 bytes each) into sizes
	for i := 0; i < sizesLen; i++ {
		if i >= len(sizes) {
			err = errors.New("sizes array too small")
			return
		}
		sizes[i] = int(binary.BigEndian.Uint16(data[dataOffset : dataOffset+2]))
		dataOffset += 2
	}

	// Read combined data into buf
	for i := 0; i < sizesLen; i++ {
		if i >= len(buf) {
			err = errors.New("buf array too small")
			return
		}
		if len(buf[i]) < offset+sizes[i] {
			err = errors.New("buf slice too small")
			return
		}
		copy(buf[i][offset:offset+sizes[i]], data[dataOffset:dataOffset+sizes[i]])
		dataOffset += sizes[i]
	}

	n = sizesLen
	return
}

// ReadMessage reads type and data separately without extra allocation
// data parameter is provided by caller for reuse
// Handles nested compression and encryption layers
// For TypeCompress/TypeEncrypted, unwraps them and returns the inner message type
// For other types, returns directly
func (t *Transport) ReadMessage(data []byte) (typ byte, n int, err error) {
	// Read magic (4 bytes)
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[:4]); err != nil {
		return
	}

	// Validate magic number using direct array comparison (faster)
	if *(*[4]byte)((*t.buffer)[:4]) != t.magic {
		return 0, 0, ErrInvalidMagic
	}

	// Read type and length
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[4:7]); err != nil {
		return
	}

	msgTyp := (*t.buffer)[4]
	length := binary.BigEndian.Uint16((*t.buffer)[5:7])

	// Read data
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[7:7+length]); err != nil {
		return
	}

	// Handle nested encryption: if type is TypeEncrypted, decrypt first
	if msgTyp == TypeEncrypted {
		if t.encryptor == nil {
			return 0, 0, ErrMissingEncryptor
		}
		// Decrypt data using cryptoBuffer to avoid conflicts
		decrypted, decryptErr := t.encryptor.Decrypt((*t.buffer)[7:7+length], (*t.cryptoBuffer)[7:])
		if decryptErr != nil {
			return 0, 0, decryptErr
		}
		// Check if decrypted data fits in buffer
		if len(decrypted) > MaxTransportByteSize-7 {
			return 0, 0, ErrMessageTooLarge
		}
		// Copy decrypted data back to main buffer and parse the message type from decrypted data
		newLen := len(decrypted)
		copy((*t.buffer)[7:7+newLen], decrypted)
		// Check if decrypted data contains a valid message
		if newLen >= 3 {
			// Parse the message from decrypted data: type is at offset 7, length is at 8-9
			innerType := (*t.buffer)[7]
			innerLength := binary.BigEndian.Uint16((*t.buffer)[8:10])
			// Check if inner message is valid (header: 3 bytes + data length)
			if newLen >= int(3+innerLength) {
				// Copy inner message data to caller's buffer
				innerDataOffset := 10
				copy(data, (*t.buffer)[innerDataOffset:innerDataOffset+int(innerLength)])
				return innerType, int(innerLength), nil
			}
		}
		// If decrypted data doesn't contain a valid message, return as raw data
		typ = TypeTransport // Default to TypeTransport for decrypted content
		copy(data, (*t.buffer)[7:7+newLen])
		n = newLen
		return
	}

	// Handle nested compression: if type is TypeCompress, decompress next
	if msgTyp == TypeCompress {
		if t.compressor == nil {
			return 0, 0, ErrMissingCompressor
		}
		// Decompress data using cryptoBuffer to avoid conflicts
		decompressed, err := t.compressor.Decompress((*t.buffer)[7:7+length], (*t.cryptoBuffer)[7:])
		if err != nil {
			return 0, 0, err
		}
		// Check if decompressed data fits in buffer
		if len(decompressed) > MaxTransportByteSize-7 {
			return 0, 0, ErrMessageTooLarge
		}
		// Copy decompressed data back to main buffer and continue processing
		newLen := len(decompressed)
		copy((*t.buffer)[7:7+newLen], decompressed)
		return t.ReadMessage(data)
	}

	// For other types, return directly
	typ = msgTyp
	// Copy data to caller's buffer for reuse
	copy(data, (*t.buffer)[7:7+length])
	n = int(length)
	return
}

// WriteMessage writes magic, type and data as a complete message
// Supports nested compression and encryption
func (t *Transport) WriteMessage(typ byte, data []byte) (int, error) {
	currentData := data
	currentTyp := typ
	totalWritten := 0

	// Apply nested encryption first (innermost layer)
	if t.encrypt && t.encryptor != nil && (typ == TypeTransport || typ == TypeBatchTransport) {
		// Construct complete inner message: type + length + data
		innerLen := len(currentData)
		if innerLen > 65530 {
			return 0, ErrMessageTooLarge
		}
		// Build inner message in cryptoBuffer
		innerMsg := (*t.cryptoBuffer)[7 : 7+3+innerLen]
		innerMsg[0] = typ
		binary.BigEndian.PutUint16(innerMsg[1:3], uint16(innerLen))
		copy(innerMsg[3:], currentData)
		// Encrypt the complete inner message
		encrypted, err := t.encryptor.Encrypt(innerMsg, (*t.cryptoBuffer)[7+3+innerLen:])
		if err != nil {
			return 0, err
		}
		// Write encrypted message with TypeEncrypted
		n, err := t.writeRawMessage(TypeEncrypted, encrypted)
		if err != nil {
			return 0, err
		}
		totalWritten = n
		currentData = (*t.buffer)[:totalWritten-7] // The complete message written so far (excluding magic)
		currentTyp = TypeEncrypted
	}

	// Apply compression (wraps encryption if both enabled) for standard types
	// Only compress if data size exceeds threshold
	if t.compress && t.compressor != nil && (typ == TypeTransport || typ == TypeBatchTransport) {
		// Check if data size exceeds compression threshold
		if len(currentData) >= t.compressThreshold {
			// Use cryptoBuffer for compression to avoid conflicts
			compressed, err := t.compressor.Compress(currentData, (*t.cryptoBuffer)[7:])
			if err != nil {
				return 0, err
			}
			// Only use compressed data if it's smaller than original
			if len(compressed) < len(currentData) {
				// Write compressed message with TypeCompress
				// If already encrypted, compress the encrypted message; otherwise compress original data
				var n int
				if currentTyp == TypeEncrypted {
					// Need to wrap the encrypted message
					n, err = t.writeRawMessage(TypeCompress, compressed)
				} else {
					n, err = t.writeRawMessage(TypeCompress, compressed)
				}
				if err != nil {
					return 0, err
				}
				totalWritten = n
				currentTyp = TypeCompress
			}
		}
	}

	// If neither compression nor encryption enabled, write raw message
	if currentTyp == typ {
		return t.writeRawMessage(typ, currentData)
	}

	return totalWritten, nil
}

// writeRawMessage writes a message without applying compression/encryption
func (t *Transport) writeRawMessage(typ byte, data []byte) (int, error) {
	length := len(data)
	if length > 65530 {
		return 0, ErrMessageTooLarge
	}

	// Use buffer for magic (direct array assignment)
	*(*[4]byte)((*t.buffer)[:4]) = t.magic

	// Use buffer for type and length
	(*t.buffer)[4] = typ
	binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(length))

	// Copy data to buffer
	copy((*t.buffer)[7:7+length], data)

	// Write complete message
	_, err := t.upstream.Write((*t.buffer)[:7+length])
	return 7 + length, err
}

func (t *Transport) Upstream() io.ReadWriter {
	return t.upstream
}

func (t *Transport) Close() (err error) {
	return t.upstream.Close()
}

func (t *Transport) Type() string {
	return t.typ
}
