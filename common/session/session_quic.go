package session

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/quic-go/quic-go"
)

type (
	quicSendReceiver struct {
		*quic.Conn
		reassembler *messageReassembler
	}
)

const (
	// MaxDatagramSize QUIC datagram maximum size limit
	// Account for IP(20) + UDP(8) + QUIC headers (~20-40)
	MaxDatagramSize = 1000

	// FragmentMagic identifies fragmented datagrams
	// 0xD0C0 = "fragment" in leetspeak / distinctive pattern
	FragmentMagic = 0xD0C0
)

// Fragment header format (10 bytes total):
// [0:2] Magic (2 bytes) - 0xD0C0 to identify fragmented message
// [2:6] MessageID (4 bytes) - unique identifier for message
// [6:8] FragmentIndex (2 bytes) - current fragment index (0-based)
// [8:10] FragmentCount (2 bytes) - total number of fragments
type fragmentHeader struct {
	Magic         uint16
	MessageID     uint32
	FragmentIndex uint16
	FragmentCount uint16
}

func (h fragmentHeader) encode() []byte {
	buf := make([]byte, 10)
	binary.BigEndian.PutUint16(buf[0:2], h.Magic)
	binary.BigEndian.PutUint32(buf[2:6], h.MessageID)
	binary.BigEndian.PutUint16(buf[6:8], h.FragmentIndex)
	binary.BigEndian.PutUint16(buf[8:10], h.FragmentCount)
	return buf
}

func decodeFragmentHeader(data []byte, ctx context.Context) (fragmentHeader, []byte, error) {
	if len(data) < 10 {
		return fragmentHeader{}, nil, errors.New("data too short for fragment header")
	}

	magic := binary.BigEndian.Uint16(data[0:2])
	if magic != FragmentMagic {
		return fragmentHeader{}, nil, errors.New("invalid magic number - not a fragment")
	}

	header := fragmentHeader{
		Magic:         magic,
		MessageID:     binary.BigEndian.Uint32(data[2:6]),
		FragmentIndex: binary.BigEndian.Uint16(data[6:8]),
		FragmentCount: binary.BigEndian.Uint16(data[8:10]),
	}

	// Validate header: FragmentIndex must be less than FragmentCount
	if header.FragmentIndex >= header.FragmentCount {
		g.Log().Debugf(ctx, "error: invalid fragment header - FragmentIndex %d >= FragmentCount %d, MessageID: %d",
			header.FragmentIndex, header.FragmentCount, header.MessageID)
		return fragmentHeader{}, nil, errors.New("invalid fragment header: index >= count")
	}

	return header, data[10:], nil
}

// Message reassembler for collecting fragments
type messageReassembler struct {
	mu       sync.Mutex
	messages map[uint32]*partialMessage
}

type partialMessage struct {
	fragments [][]byte
	received  uint16
}

func newMessageReassembler() *messageReassembler {
	return &messageReassembler{
		messages: make(map[uint32]*partialMessage),
	}
}

func (r *messageReassembler) addFragment(ctx context.Context, header fragmentHeader, data []byte) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg, exists := r.messages[header.MessageID]
	if !exists {
		msg = &partialMessage{
			fragments: make([][]byte, header.FragmentCount),
			received:  0,
		}
		r.messages[header.MessageID] = msg
	}

	// Safety check: FragmentCount must match
	if len(msg.fragments) != int(header.FragmentCount) {
		// FragmentCount mismatch, ignore this fragment and delete old message
		g.Log().Debugf(ctx, "warning: fragment count mismatch for message %d (expected %d, got %d), deleting old message",
			header.MessageID, len(msg.fragments), header.FragmentCount)
		delete(r.messages, header.MessageID)
		return nil, false
	}

	// Safety check: FragmentIndex must be within bounds
	if int(header.FragmentIndex) >= len(msg.fragments) {
		// Invalid fragment index, ignore this fragment
		g.Log().Debugf(ctx, "warning: invalid fragment index %d (count: %d) for message %d",
			header.FragmentIndex, len(msg.fragments), header.MessageID)
		return nil, false
	}

	if msg.fragments[header.FragmentIndex] != nil {
		// Duplicate fragment, ignore
		return nil, false
	}

	msg.fragments[header.FragmentIndex] = data
	msg.received++

	if msg.received == header.FragmentCount {
		// All fragments received, assemble message
		delete(r.messages, header.MessageID)
		totalSize := 0
		for _, frag := range msg.fragments {
			totalSize += len(frag)
		}
		result := make([]byte, 0, totalSize)
		for _, frag := range msg.fragments {
			result = append(result, frag...)
		}
		return result, true
	}

	return nil, false
}

// Cleanup old incomplete messages (call periodically)
func (r *messageReassembler) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Simple cleanup: remove all old messages
	// For production, track creation time per message
	r.messages = make(map[uint32]*partialMessage)
}

var (
	messageIDCounter uint32
	messageIDMu      sync.Mutex
)

func nextMessageID() uint32 {
	messageIDMu.Lock()
	defer messageIDMu.Unlock()
	messageIDCounter++
	return messageIDCounter
}

func (q *quicSendReceiver) Send(data []byte) (err error) {
	if len(data) <= MaxDatagramSize-10 {
		// Single fragment, no header needed for backward compatibility
		return q.Conn.SendDatagram(data)
	}

	// Fragment the data
	msgID := nextMessageID()
	maxPayloadSize := MaxDatagramSize - 10 // Reserve space for header (2 magic + 8 data)
	fragmentCount := (len(data) + maxPayloadSize - 1) / maxPayloadSize
	fragmentCountUint := uint16(fragmentCount)

	for i := uint16(0); i < fragmentCountUint; i++ {
		start := int(i) * maxPayloadSize
		end := start + maxPayloadSize
		if end > len(data) {
			end = len(data)
		}

		fragmentData := data[start:end]
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: i,
			FragmentCount: fragmentCountUint,
		}

		datagram := append(header.encode(), fragmentData...)
		if err := q.Conn.SendDatagram(datagram); err != nil {
			return fmt.Errorf("failed to send fragment %d/%d: %w", i, fragmentCountUint, err)
		}
	}

	return nil
}

func (q *quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	// Initialize reassembler if not exists
	if q.reassembler == nil {
		q.reassembler = newMessageReassembler()
	}

	// Set timeout for receiving complete message
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		datagram, err := q.Conn.ReceiveDatagram(ctx)
		if err != nil {
			return nil, err
		}

		// Check if it's a fragment (has magic number)
		if len(datagram) >= 10 {
			// Try to decode as fragment
			header, payload, err := decodeFragmentHeader(datagram, nil)
			if err == nil {
				// Valid fragment header
				g.Log().Debugf(ctx, "received fragment %d/%d (msgID: %d)",
					header.FragmentIndex+1, header.FragmentCount, header.MessageID)

				completeMsg, done := q.reassembler.addFragment(nil, header, payload)
				if done {
					g.Log().Debugf(ctx, "reassembled complete message, size: %d", len(completeMsg))
					return completeMsg, nil
				}
				continue
			}
		}

		// Single message (no fragment magic number)
		return datagram, nil
	}
}

func (q *quicSendReceiver) CloseWithError(err error) {
	if q.reassembler != nil {
		q.reassembler.cleanup()
	}
	desc := "connection closed"
	code := 0
	if err != nil {
		desc = err.Error()
		var ee *Error
		if errors.As(err, &ee) {
			code = ee.Code()
		}
	}
	_ = q.Conn.CloseWithError(quic.ApplicationErrorCode(code), desc)
}

func SendReceiverFromQuicConn(conn *quic.Conn) SendReceiveCloser {
	return &quicSendReceiver{Conn: conn, reassembler: nil}
}
