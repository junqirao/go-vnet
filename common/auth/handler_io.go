package auth

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type ioHandler struct {
	pipe io.ReadWriter
}

// NewIOHandler creates a new I/O handler that uses the provided ReadWriter for communication
func NewIOHandler(pipe io.ReadWriter) HandleFunc {
	return &ioHandler{pipe: pipe}
}

func (h *ioHandler) Do(ctx context.Context, au AuthorizedHandler, send ...map[string]any) (map[string]any, error) {
	// Authenticate and serialize the payload
	authData, err := au.Make(ctx, send...)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Write the length of the auth data (4 bytes, big-endian)
	dataLen := make([]byte, 4)
	binary.BigEndian.PutUint32(dataLen, uint32(len(authData)))

	// Write the length prefix and then the actual data
	if _, err := h.pipe.Write(dataLen); err != nil {
		return nil, fmt.Errorf("failed to write data length: %w", err)
	}

	if _, err := h.pipe.Write(authData); err != nil {
		return nil, fmt.Errorf("failed to write auth data: %w", err)
	}

	// Read the response length
	var respLen uint32
	if err := binary.Read(h.pipe, binary.BigEndian, &respLen); err != nil {
		return nil, fmt.Errorf("failed to read response length: %w", err)
	}

	// Read the response data
	respData := make([]byte, respLen)
	if _, err := io.ReadFull(h.pipe, respData); err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}

	res := map[string]any{}
	err = json.Unmarshal(respData, &res)
	return res, err
}
