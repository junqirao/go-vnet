package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	MaxTransportBatchSize = 46
	MaxTransportByteSize  = 65535
)

const (
	TypeTransport      byte = 0x0
	TypeBatchTransport byte = 0x1
)

type (
	ReadWriter interface {
		io.ReadWriter
		ReadMessage(data []byte) (typ byte, n int, err error)
		WriteMessage(typ byte, data []byte) (int, error)
		BatchWrite(buf [][]byte, sizes []int, headerSize int) (n int, err error)
		ParseBatch(data []byte, buf [][]byte, sizes []int, offset int) (n int, err error)
	}
)

var (
	// Pre-computed magic for fast comparison
	transportMagicBytes = [4]byte{0x56, 0x4E, 0x45, 0x54}
)

var (
	ErrInvalidMagic    = errors.New("invalid magic number")
	ErrMessageTooLarge = errors.New("message too large")
)

// Transport protocol
// | magic 4 bytes | type 1 byte | length 2 byte | data n byte |
// magic: "VNET" (0x56 0x4E 0x45 0x54) - unique protocol identifier
type Transport struct {
	upstream io.ReadWriter
	buffer   *[MaxTransportByteSize]byte
	magic    [4]byte
}

// NewTransport creates a new Transport protocol read writer
func NewTransport(upstream io.ReadWriter, magic ...[4]byte) ReadWriter {
	mg := transportMagicBytes
	if len(magic) > 0 {
		mg = magic[0]
	}
	return &Transport{
		upstream: upstream,
		buffer:   &[MaxTransportByteSize]byte{},
		magic:    mg,
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
	if length == 1 {
		return t.Write(buf[0][headerSize : sizes[0]+headerSize])
	}
	nn := 0
	for i := 0; i < length; i += MaxTransportBatchSize {
		end := i + MaxTransportBatchSize
		if end > length {
			end = length
		}
		nn, err = t.batchWrite(buf[i:end], sizes[i:end], headerSize)
		if err != nil {
			return nn, err
		}
		n += nn
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

	// Write complete message
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

	typ = (*t.buffer)[4]
	length := binary.BigEndian.Uint16((*t.buffer)[5:7])

	// Read data
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[7:7+length]); err != nil {
		return
	}

	// Copy data to caller's buffer for reuse
	copy(data, (*t.buffer)[7:7+length])
	n = int(length)
	return
}

// WriteMessage writes magic, type and data as a complete message
func (t *Transport) WriteMessage(typ byte, data []byte) (int, error) {
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
