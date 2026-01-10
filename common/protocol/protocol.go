package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

type (
	ReadWriter interface {
		io.ReadWriter
		ReadMessage(data []byte) (typ byte, n int, err error)
		WriteMessage(typ byte, data []byte) (int, error)
	}
)

var (
	// Pre-computed magic for fast comparison
	transportMagicBytes = [4]byte{0x56, 0x4E, 0x45, 0x54}
)

var (
	ErrInvalidMagic = errors.New("invalid magic number")
)

// Transport protocol
// | magic 4 bytes | type 1 byte | length 2 byte | data n byte |
// magic: "VNET" (0x56 0x4E 0x45 0x54) - unique protocol identifier
type Transport struct {
	upstream io.ReadWriter
	buffer   *[65535]byte
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
		buffer:   &[65535]byte{},
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
		return 0, errors.New("invalid magic number")
	}

	// Read type and length (3 bytes)
	if _, err = io.ReadFull(t.upstream, (*t.buffer)[4:7]); err != nil {
		return 0, err
	}

	length := binary.BigEndian.Uint16((*t.buffer)[5:7])
	if length > 65530 {
		return 0, errors.New("message too large")
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
	length := len(p)
	if length > 65530 {
		return 0, errors.New("message too large")
	}

	// Build magic using pre-computed bytes
	*(*[4]byte)((*t.buffer)[:4]) = t.magic

	// Build type and length
	(*t.buffer)[4] = 0 // type byte
	binary.BigEndian.PutUint16((*t.buffer)[5:7], uint16(length))

	// Copy data
	copy((*t.buffer)[7:7+length], p)

	// Write complete message
	_, err = t.upstream.Write((*t.buffer)[:7+length])
	return 7 + length, err
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
		return 0, errors.New("message too large")
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
