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
		// Compress compresses data into buf, returns number of bytes written
		Compress(data []byte, buf []byte) (int, error)
		// Decompress decompresses data into buf, returns number of bytes written
		Decompress(data []byte, buf []byte) (int, error)
	}

	// Encryptor defines the encryption interface
	// buf parameter allows reusing memory to avoid allocation
	Encryptor interface {
		// Encrypt encrypts data into buf, returns number of bytes written
		Encrypt(data []byte, buf []byte) (int, error)
		// Decrypt decrypts data into buf, returns number of bytes written
		Decrypt(data []byte, buf []byte) (int, error)
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
			encrypt:           true,
			compress:          false,
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
	EnableCompress = func(compress bool) TransportOpt {
		return func(o *TransportOptions) {
			o.compress = compress
		}
	}
	WithCompressor = func(c Compressor) TransportOpt {
		return func(o *TransportOptions) {
			o.compressor = c
		}
	}
	EnableEncrypt = func(encrypt bool) TransportOpt {
		return func(o *TransportOptions) {
			o.encrypt = encrypt
		}
	}
	WithEncryptor = func(e Encryptor) TransportOpt {
		return func(o *TransportOptions) {
			o.encryptor = e
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

		// avoid loop forever, collected no data
		if end == start {
			return nn, ErrMessageTooLarge
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

	// Determine final message type and data
	finalType := TypeBatchTransport
	finalData := (*t.buffer)[7 : 7+dataLen]
	finalDataLen := dataLen

	// Apply compression first (innermost layer)
	if t.compress && t.compressor != nil {
		// Check if data size exceeds compression threshold
		if dataLen >= t.compressThreshold {
			// Build type+length header for decompression
			// Format: originalType(1) + originalLength(2) + originalData
			headerLen := 3 + dataLen

			// Check if we can safely reuse cryptoBuffer without overflow
			// We need enough space in target: cryptoBuffer[headerLen:] capacity
			// Zstd may need up to headerLen + 17 bytes in worst case
			if headerLen+1000 < len(*t.cryptoBuffer) && (len(*t.cryptoBuffer)-headerLen >= headerLen+17) {
				// Safe to reuse cryptoBuffer
				(*t.cryptoBuffer)[0] = TypeBatchTransport
				binary.BigEndian.PutUint16((*t.cryptoBuffer)[1:3], uint16(dataLen))
				copy((*t.cryptoBuffer)[3:], finalData)

				// Compress type+length+data to cryptoBuffer[headerLen:]
				compressTarget := (*t.cryptoBuffer)[headerLen:]
				compressedLen, err := t.compressor.Compress((*t.cryptoBuffer)[:headerLen], compressTarget)
				if err != nil {
					return 0, err
				}
				// Only use compressed data if it's smaller than original
				if compressedLen < dataLen {
					finalType = TypeCompress
					finalData = compressTarget[:compressedLen]
					finalDataLen = compressedLen
				}
			} else {
				// Data too large to safely reuse buffer, use temp buffer
				tempMsg := make([]byte, headerLen)
				tempMsg[0] = TypeBatchTransport
				binary.BigEndian.PutUint16(tempMsg[1:3], uint16(dataLen))
				copy(tempMsg[3:], finalData)

				compressedLen, err := t.compressor.Compress(tempMsg, (*t.cryptoBuffer)[7:])
				if err != nil {
					return 0, err
				}
				// Only use compressed data if it's smaller than original
				if compressedLen < dataLen {
					finalType = TypeCompress
					finalData = (*t.cryptoBuffer)[7 : 7+compressedLen]
					finalDataLen = compressedLen
				}
			}
		}
	}

	// Apply encryption (outer layer) after compression
	if t.encrypt && t.encryptor != nil {
		// Construct complete inner message: type + length + data
		innerMsgLen := 3 + finalDataLen

		// Check if encrypted message fits in buffer
		encryptedMsgLen := innerMsgLen + Chacha20Poly1305NonceSize + Chacha20Poly1305Overhead
		if encryptedMsgLen > MaxTransportByteSize-7 {
			return 0, ErrMessageTooLarge
		}

		// Check if finalData overlaps with cryptoBuffer (reused from compression)
		// If yes, we need to use temp buffer, otherwise reuse cryptoBuffer
		dataInCryptoBuffer := len(finalData) > 0 && &finalData[0] == &(*t.cryptoBuffer)[0]

		if !dataInCryptoBuffer {
			// Check if we have enough space to reuse cryptoBuffer for both source and target
			canReuseCryptoBuffer := innerMsgLen+encryptedMsgLen <= len(*t.cryptoBuffer)

			if canReuseCryptoBuffer {
				// Buffer is large enough, reuse cryptoBuffer efficiently
				// Build inner message directly in cryptoBuffer
				(*t.cryptoBuffer)[0] = finalType
				binary.BigEndian.PutUint16((*t.cryptoBuffer)[1:3], uint16(finalDataLen))
				copy((*t.cryptoBuffer)[3:], finalData)

				// Encrypt from cryptoBuffer[0:innerMsgLen] to cryptoBuffer[innerMsgLen:encryptedMsgLen]
				encryptBuf := (*t.cryptoBuffer)[innerMsgLen:encryptedMsgLen]
				encryptedLen, err := t.encryptor.Encrypt((*t.cryptoBuffer)[:innerMsgLen], encryptBuf)
				if err != nil {
					return 0, err
				}
				// Write encrypted message with TypeEncrypted
				*(*[4]byte)((*t.buffer)[:4]) = t.magic
				(*t.buffer)[4] = TypeEncrypted
				binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(encryptedLen))
				copy((*t.buffer)[7:], encryptBuf[:encryptedLen])
				totalLen := 7 + encryptedLen
				_, err = t.upstream.Write((*t.buffer)[:totalLen])
				return totalLen, err
			}

			// Buffer not large enough, use temp buffer
			tempMsg := make([]byte, innerMsgLen)
			tempMsg[0] = finalType
			binary.BigEndian.PutUint16(tempMsg[1:3], uint16(finalDataLen))
			copy(tempMsg[3:], finalData)

			encryptBuf := (*t.cryptoBuffer)[:encryptedMsgLen]
			encryptedLen, err := t.encryptor.Encrypt(tempMsg, encryptBuf)
			if err != nil {
				return 0, err
			}
			// Write encrypted message with TypeEncrypted
			*(*[4]byte)((*t.buffer)[:4]) = t.magic
			(*t.buffer)[4] = TypeEncrypted
			binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(encryptedLen))
			copy((*t.buffer)[7:], encryptBuf[:encryptedLen])
			totalLen := 7 + encryptedLen
			_, err = t.upstream.Write((*t.buffer)[:totalLen])
			return totalLen, err
		}
	}

	// Write complete message (unencrypted)
	*(*[4]byte)((*t.buffer)[:4]) = t.magic
	(*t.buffer)[4] = finalType
	binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(finalDataLen))
	copy((*t.buffer)[7:], finalData)
	totalLen := 7 + finalDataLen
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
// Order: Decrypt (optional) -> Decompress (optional) -> Return final type
// For TypeEncrypted/TypeCompress, unwraps them recursively until TypeTransport/TypeBatchTransport
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

	// Handle encryption first (outer layer): if type is TypeEncrypted, decrypt first
	if msgTyp == TypeEncrypted {
		if t.encryptor == nil {
			return 0, 0, ErrMissingEncryptor
		}
		// Decrypt data using cryptoBuffer to avoid conflicts
		decryptBuf := (*t.cryptoBuffer)[7:]
		decryptedLen, decryptErr := t.encryptor.Decrypt((*t.buffer)[7:7+length], decryptBuf)
		if decryptErr != nil {
			return 0, 0, decryptErr
		}
		// Check if decrypted data fits in buffer
		if decryptedLen > MaxTransportByteSize-7 {
			return 0, 0, ErrMessageTooLarge
		}
		// Copy decrypted data back to main buffer and parse the message type from decrypted data
		copy((*t.buffer)[7:7+decryptedLen], decryptBuf[:decryptedLen])
		// Update length to decrypted length for further processing
		length = uint16(decryptedLen)

		// Parse the message from decrypted data: type is at offset 7, length is at 8-9
		if length >= 3 {
			msgTyp = (*t.buffer)[7]
			innerLength := binary.BigEndian.Uint16((*t.buffer)[8:10])
			// Validate inner message length
			if length >= 3+innerLength {
				// Move inner data to buffer offset 7 and update length
				innerDataOffset := 10
				innerDataLen := int(innerLength)
				copy((*t.buffer)[7:], (*t.buffer)[innerDataOffset:innerDataOffset+innerDataLen])
				length = uint16(innerDataLen)
			}
		}
	}

	// Handle compression (middle layer): if type is TypeCompress, decompress next
	if msgTyp == TypeCompress {
		if t.compressor == nil {
			return 0, 0, ErrMissingCompressor
		}
		// Decompress data using cryptoBuffer to avoid conflicts
		decompressedLen, err := t.compressor.Decompress((*t.buffer)[7:7+length], (*t.cryptoBuffer)[7:])
		if err != nil {
			return 0, 0, err
		}
		// Check if decompressed data fits in buffer
		if decompressedLen > MaxTransportByteSize-7 {
			return 0, 0, ErrMessageTooLarge
		}
		// Parse the message type from decompressed data (type + length header)
		// The decompressed data in cryptoBuffer is in format: type(1) + length(2) + data
		if decompressedLen >= 3 {
			msgTyp = (*t.cryptoBuffer)[7]
			innerLength := binary.BigEndian.Uint16((*t.cryptoBuffer)[8:10])
			// Validate inner message length
			if uint16(decompressedLen) >= 3+innerLength {
				// Copy original data (skipping type and length headers) to main buffer
				innerDataOffset := 10 // 3 bytes header offset in cryptoBuffer + 7 = 10
				innerDataLen := int(innerLength)
				copy((*t.buffer)[7:], (*t.cryptoBuffer)[innerDataOffset:innerDataOffset+innerDataLen])
				length = uint16(innerDataLen)
			} else {
				// Invalid compressed data format, return error
				return 0, 0, ErrDecompressionFailed
			}
		} else {
			// Compressed data too small to contain type + length header
			return 0, 0, ErrDecompressionFailed
		}
	}

	// Return final message type (should be TypeTransport or TypeBatchTransport)
	// Copy data to caller's buffer for reuse
	typ = msgTyp
	copy(data, (*t.buffer)[7:7+length])
	n = int(length)
	return
}

// WriteMessage writes magic, type and data as a complete message
// Supports nested compression and encryption
// Order: Compress (optional) -> Encrypt (optional) -> Send
func (t *Transport) WriteMessage(typ byte, data []byte) (int, error) {
	currentData := data
	currentTyp := typ

	// Apply compression first (innermost layer) for standard types
	// Only compress if data size exceeds threshold
	if t.compress && t.compressor != nil && (typ == TypeTransport || typ == TypeBatchTransport) {
		// Check if data size exceeds compression threshold
		if len(currentData) >= t.compressThreshold {
			// Build original type+length header for decompression
			// Format: originalType(1) + originalLength(2) + originalData
			headerLen := 3 + len(currentData)

			// Check if we can safely reuse cryptoBuffer without overflow
			// We need enough space in target: cryptoBuffer[headerLen:] capacity
			// Zstd may need up to headerLen + 17 bytes in worst case
			if headerLen+1000 < len(*t.cryptoBuffer) && (len(*t.cryptoBuffer)-headerLen >= headerLen+17) {
				// Safe to reuse cryptoBuffer
				(*t.cryptoBuffer)[0] = typ
				binary.BigEndian.PutUint16((*t.cryptoBuffer)[1:3], uint16(len(currentData)))
				copy((*t.cryptoBuffer)[3:], currentData)

				// Compress type+length+data to cryptoBuffer[headerLen:]
				compressTarget := (*t.cryptoBuffer)[headerLen:]
				compressedLen, err := t.compressor.Compress((*t.cryptoBuffer)[:headerLen], compressTarget)
				if err != nil {
					return 0, err
				}
				// Only use compressed data if it's smaller than original
				if compressedLen < len(currentData) {
					currentData = compressTarget[:compressedLen]
					currentTyp = TypeCompress
				}
			} else {
				// Data too large to safely reuse buffer, use temp buffer
				tempMsg := make([]byte, headerLen)
				tempMsg[0] = typ
				binary.BigEndian.PutUint16(tempMsg[1:3], uint16(len(currentData)))
				copy(tempMsg[3:], currentData)

				compressedLen, err := t.compressor.Compress(tempMsg, (*t.cryptoBuffer)[7:])
				if err != nil {
					return 0, err
				}
				// Only use compressed data if it's smaller than original
				if compressedLen < len(currentData) {
					currentData = (*t.cryptoBuffer)[7 : 7+compressedLen]
					currentTyp = TypeCompress
				}
			}
		}
	}

	// Apply encryption (outer layer) after compression
	if t.encrypt && t.encryptor != nil && (typ == TypeTransport || typ == TypeBatchTransport || currentTyp == TypeCompress) {
		// Construct complete inner message: type + length + data
		innerLen := len(currentData)
		innerMsgLen := 3 + innerLen

		// Check if encrypted message fits in buffer
		encryptedMsgLen := innerMsgLen + Chacha20Poly1305NonceSize + Chacha20Poly1305Overhead
		if encryptedMsgLen > 65530 {
			return 0, ErrMessageTooLarge
		}

		// Check if currentData overlaps with cryptoBuffer (reused from compression)
		// If yes, we need to use temp buffer, otherwise reuse cryptoBuffer
		dataInCryptoBuffer := len(currentData) > 0 && &currentData[0] == &(*t.cryptoBuffer)[0]
		var encryptBuf []byte

		if dataInCryptoBuffer {
			// currentData is from cryptoBuffer, need temp buffer to avoid overlap
			tempMsg := make([]byte, innerMsgLen)
			tempMsg[0] = currentTyp
			binary.BigEndian.PutUint16(tempMsg[1:3], uint16(innerLen))
			copy(tempMsg[3:], currentData)
			encryptBuf = (*t.cryptoBuffer)[:encryptedMsgLen]
			var err error
			encryptedLen, err := t.encryptor.Encrypt(tempMsg, encryptBuf)
			if err != nil {
				return 0, err
			}
			// Write encrypted message with TypeEncrypted
			return t.writeRawMessage(TypeEncrypted, encryptBuf[:encryptedLen])
		}

		// Check if we have enough space to reuse cryptoBuffer for both source and target
		canReuseCryptoBuffer := innerMsgLen+encryptedMsgLen <= len(*t.cryptoBuffer)

		if canReuseCryptoBuffer {
			// Buffer is large enough, reuse cryptoBuffer efficiently
			// Build inner message directly in cryptoBuffer
			(*t.cryptoBuffer)[0] = currentTyp
			binary.BigEndian.PutUint16((*t.cryptoBuffer)[1:3], uint16(innerLen))
			copy((*t.cryptoBuffer)[3:], currentData)

			// Encrypt from cryptoBuffer[0:innerMsgLen] to cryptoBuffer[innerMsgLen:encryptedMsgLen]
			encryptBuf := (*t.cryptoBuffer)[innerMsgLen:encryptedMsgLen]
			encryptedLen, err := t.encryptor.Encrypt((*t.cryptoBuffer)[:innerMsgLen], encryptBuf)
			if err != nil {
				return 0, err
			}
			// Write encrypted message with TypeEncrypted
			return t.writeRawMessage(TypeEncrypted, encryptBuf[:encryptedLen])
		}

		// Buffer not large enough, use temp buffer
		tempMsg := make([]byte, innerMsgLen)
		tempMsg[0] = currentTyp
		binary.BigEndian.PutUint16(tempMsg[1:3], uint16(innerLen))
		copy(tempMsg[3:], currentData)

		encryptBuf = (*t.cryptoBuffer)[:encryptedMsgLen]
		encryptedLen, err := t.encryptor.Encrypt(tempMsg, encryptBuf)
		if err != nil {
			return 0, err
		}
		// Write encrypted message with TypeEncrypted
		return t.writeRawMessage(TypeEncrypted, encryptBuf[:encryptedLen])
	}

	// If neither compression nor encryption enabled, write raw message
	return t.writeRawMessage(currentTyp, currentData)
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
