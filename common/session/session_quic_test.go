package session

import (
	"testing"
)

// Basic fragmentation tests
func TestFragmentSendReceive(t *testing.T) {
	// This test demonstrates the fragmentation mechanism
	// In real usage, you would have an actual QUIC connection

	// Simulate different message sizes
	testSizes := []int{
		500,   // Small, no fragmentation
		1200,  // Near limit
		2000,  // Needs 2 fragments
		5000,  // Needs ~5 fragments
		10000, // Needs ~9 fragments
	}

	for _, size := range testSizes {
		t.Logf("Testing message size: %d bytes", size)

		// Create test data
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}

		// Calculate expected fragments
		maxPayloadSize := MaxDatagramSize - 10 // 2 magic + 8 header
		expectedFragments := (size + maxPayloadSize - 1) / maxPayloadSize

		// Simulate fragmentation
		fragments := []fragmentHeader{}
		currentMsgID := nextMessageID()
		for i := 0; i < expectedFragments; i++ {
			start := i * maxPayloadSize
			end := start + maxPayloadSize
			if end > size {
				end = size
			}
			fragSize := end - start

			header := fragmentHeader{
				Magic:         FragmentMagic,
				MessageID:     currentMsgID,
				FragmentIndex: uint16(i),
				FragmentCount: uint16(expectedFragments),
			}
			fragments = append(fragments, header)

			t.Logf("  Fragment %d: header %+v, payload size: %d",
				i+1, header, fragSize)
		}

		t.Logf("  Total fragments: %d", len(fragments))
	}
}

func TestReassembler(t *testing.T) {
	reassembler := newMessageReassembler()

	// Test case: message split into 3 fragments
	msgID := uint32(100)
	originalData := [][]byte{
		[]byte("hello "),
		[]byte("world, "),
		[]byte("this is a test!"),
	}

	// Add fragments out of order
	for i := range []int{1, 2, 0} {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: uint16(i),
			FragmentCount: 3,
		}
		result, done := reassembler.addFragment(header, originalData[i])

		if done {
			expected := "hello world, this is a test!"
			if string(result) != expected {
				t.Errorf("Expected '%s', got '%s'", expected, string(result))
			}
			t.Logf("Successfully reassembled: %s", string(result))
			break
		}

		t.Logf("Added fragment %d, waiting for more...", i)
	}
}

// Header encoding/decoding tests
func TestFragmentHeader(t *testing.T) {
	header := fragmentHeader{
		Magic:         FragmentMagic,
		MessageID:     123456,
		FragmentIndex: 2,
		FragmentCount: 5,
	}

	// Encode
	encoded := header.encode()
	if len(encoded) != 10 {
		t.Errorf("Expected encoded header to be 10 bytes, got %d", len(encoded))
	}

	// Decode
	decoded, _, err := decodeFragmentHeader(encoded, nil)
	if err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	if decoded.MessageID != header.MessageID {
		t.Errorf("MessageID mismatch: %d != %d", decoded.MessageID, header.MessageID)
	}
	if decoded.Magic != header.Magic {
		t.Errorf("Magic mismatch: %x != %x", decoded.Magic, header.Magic)
	}
	if decoded.FragmentIndex != header.FragmentIndex {
		t.Errorf("FragmentIndex mismatch: %d != %d", decoded.FragmentIndex, header.FragmentIndex)
	}
	if decoded.FragmentCount != header.FragmentCount {
		t.Errorf("FragmentCount mismatch: %d != %d", decoded.FragmentCount, header.FragmentCount)
	}

	t.Logf("Header encode/decode test passed: %+v", decoded)
}

func TestMaxDatagramSize(t *testing.T) {
	t.Logf("MaxDatagramSize: %d bytes", MaxDatagramSize)
	t.Logf("Max payload per fragment: %d bytes", MaxDatagramSize-10)

	// Test with data that fits
	smallData := make([]byte, MaxDatagramSize-10)
	t.Logf("Small data size: %d bytes", len(smallData))
	t.Logf("Header: %+v", fragmentHeader{Magic: FragmentMagic, MessageID: 1, FragmentIndex: 0, FragmentCount: 1}.encode())

	// Test with data that needs fragmentation
	largeData := make([]byte, MaxDatagramSize*3)
	fragmentCount := (len(largeData) + MaxDatagramSize - 10 - 1) / (MaxDatagramSize - 10)
	t.Logf("Large data size: %d bytes", len(largeData))
	t.Logf("Expected fragments: %d", fragmentCount)
}

// Validation tests
func TestDecodeInvalidFragmentHeader(t *testing.T) {
	// Test case 1: Invalid magic number
	t.Run("Invalid magic number", func(t *testing.T) {
		datagram := []byte{0x12, 0x34, 0, 0, 0, 1, 0, 0, 0, 5, 1, 2, 3}

		_, _, err := decodeFragmentHeader(datagram, nil)
		if err == nil {
			t.Error("Expected error for invalid magic number")
		}
		t.Logf("Expected error: %v", err)
	})

	// Test case 2: FragmentIndex >= FragmentCount
	t.Run("Invalid index >= count", func(t *testing.T) {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     1,
			FragmentIndex: 10,
			FragmentCount: 5,
		}
		datagram := append(header.encode(), []byte("payload")...)

		_, _, err := decodeFragmentHeader(datagram, nil)
		if err == nil {
			t.Error("Expected error for invalid fragment header")
		}
		t.Logf("Expected error: %v", err)
	})

	// Test case 3: Too short data
	t.Run("Too short data", func(t *testing.T) {
		datagram := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}

		_, _, err := decodeFragmentHeader(datagram, nil)
		if err == nil {
			t.Error("Expected error for too short data")
		}
		t.Logf("Expected error: %v", err)
	})

	// Test case 4: Valid header
	t.Run("Valid header", func(t *testing.T) {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     123,
			FragmentIndex: 2,
			FragmentCount: 5,
		}
		payload := []byte("test payload")
		datagram := append(header.encode(), payload...)

		decodedHeader, decodedPayload, err := decodeFragmentHeader(datagram, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if decodedHeader.MessageID != header.MessageID {
			t.Errorf("MessageID mismatch: %d != %d", decodedHeader.MessageID, header.MessageID)
		}
		if decodedHeader.Magic != header.Magic {
			t.Errorf("Magic mismatch: %x != %x", decodedHeader.Magic, header.Magic)
		}
		if decodedHeader.FragmentIndex != header.FragmentIndex {
			t.Errorf("FragmentIndex mismatch: %d != %d", decodedHeader.FragmentIndex, header.FragmentIndex)
		}
		if decodedHeader.FragmentCount != header.FragmentCount {
			t.Errorf("FragmentCount mismatch: %d != %d", decodedHeader.FragmentCount, header.FragmentCount)
		}
		if string(decodedPayload) != string(payload) {
			t.Errorf("Payload mismatch: %s != %s", string(decodedPayload), string(payload))
		}
	})

	// Test case 5: Edge case - FragmentIndex == FragmentCount - 1 (last fragment)
	t.Run("Last fragment (index == count - 1)", func(t *testing.T) {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     999,
			FragmentIndex: 4,
			FragmentCount: 5,
		}
		payload := []byte("last fragment")
		datagram := append(header.encode(), payload...)

		decodedHeader, decodedPayload, err := decodeFragmentHeader(datagram, nil)
		if err != nil {
			t.Fatalf("Unexpected error for last fragment: %v", err)
		}

		if decodedHeader.FragmentIndex != 4 {
			t.Errorf("FragmentIndex mismatch: %d != 4", decodedHeader.FragmentIndex)
		}
		if string(decodedPayload) != string(payload) {
			t.Errorf("Payload mismatch")
		}
	})
}

func TestFragmentHeaderEdgeCases(t *testing.T) {
	// Test single fragment (count = 1, index = 0)
	t.Run("Single fragment", func(t *testing.T) {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     1,
			FragmentIndex: 0,
			FragmentCount: 1,
		}
		payload := []byte("single fragment data")
		datagram := append(header.encode(), payload...)

		decodedHeader, decodedPayload, err := decodeFragmentHeader(datagram, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if decodedHeader.FragmentIndex != 0 || decodedHeader.FragmentCount != 1 {
			t.Error("Invalid single fragment header")
		}
		if string(decodedPayload) != string(payload) {
			t.Error("Payload mismatch")
		}
	})
}

// Safety tests
func TestReassemblerSafety(t *testing.T) {
	reassembler := newMessageReassembler()

	// Test 1: FragmentIndex out of bounds
	t.Run("FragmentIndex out of bounds", func(t *testing.T) {
		msgID := uint32(1)

		// Add first fragment with count=3
		header1 := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: 0,
			FragmentCount: 3,
		}
		reassembler.addFragment(nil, header1, []byte("part1"))

		// Try to add fragment with index 10 (out of bounds)
		header2 := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: 10,
			FragmentCount: 3,
		}
		_, done := reassembler.addFragment(nil, header2, []byte("invalid"))
		if done {
			t.Error("Should not complete message with invalid fragment index")
		}
	})

	// Test 2: FragmentCount mismatch
	t.Run("FragmentCount mismatch", func(t *testing.T) {
		msgID := uint32(2)

		// Add fragment with count=3
		header1 := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: 0,
			FragmentCount: 3,
		}
		reassembler.addFragment(nil, header1, []byte("part1"))

		// Try to add fragment with count=5 (mismatch)
		header2 := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     msgID,
			FragmentIndex: 1,
			FragmentCount: 5,
		}
		_, done := reassembler.addFragment(nil, header2, []byte("part2"))
		if done {
			t.Error("Should not complete message with fragment count mismatch")
		}
	})
}

func TestReassemblerConcurrentMessages(t *testing.T) {
	reassembler := newMessageReassembler()

	// Test multiple messages with different fragment counts
	msgs := []struct {
		msgID     uint32
		count     uint16
		fragments []string
	}{
		{1, 2, []string{"a", "b"}},
		{2, 3, []string{"c", "d", "e"}},
		{3, 1, []string{"f"}},
	}

	// Add fragments in interleaved order
	for i := 0; i < 2; i++ {
		for _, msg := range msgs {
			if i < len(msg.fragments) {
				header := fragmentHeader{
					Magic:         FragmentMagic,
					MessageID:     msg.msgID,
					FragmentIndex: uint16(i),
					FragmentCount: msg.count,
				}
				reassembler.addFragment(nil, header, []byte(msg.fragments[i]))
			}
		}
	}

	// Add remaining fragments
	for _, msg := range msgs {
		if len(msg.fragments) > 2 {
			for i := 2; i < len(msg.fragments); i++ {
				header := fragmentHeader{
					Magic:         FragmentMagic,
					MessageID:     msg.msgID,
					FragmentIndex: uint16(i),
					FragmentCount: msg.count,
				}
				reassembler.addFragment(nil, header, []byte(msg.fragments[i]))
			}
		}
	}

	t.Logf("Concurrent messages test completed without panic")
}

// Magic number discrimination tests
func TestMagicNumberDiscrimination(t *testing.T) {
	// Test that non-fragmented messages (without magic) are not detected as fragments
	t.Run("JSON message is not a fragment", func(t *testing.T) {
		// This is exact data from user's error log
		jsonMessage := []byte(`{"code":"...`)

		_, _, err := decodeFragmentHeader(jsonMessage, nil)
		if err == nil {
			t.Error("Expected error for JSON message (no magic number)")
		}

		t.Logf("JSON message correctly rejected as fragment: %v", err)
	})

	// Test that fragmented messages (with magic) are correctly identified
	t.Run("Fragmented message is detected", func(t *testing.T) {
		header := fragmentHeader{
			Magic:         FragmentMagic,
			MessageID:     12345,
			FragmentIndex: 0,
			FragmentCount: 2,
		}
		datagram := append(header.encode(), []byte("payload data")...)

		decoded, payload, err := decodeFragmentHeader(datagram, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if decoded.Magic != FragmentMagic {
			t.Error("Magic number not preserved")
		}
		if string(payload) != "payload data" {
			t.Error("Payload not preserved")
		}

		t.Logf("Fragmented message correctly detected: magic=0x%04X", decoded.Magic)
	})

	// Test edge cases
	t.Run("Message with partial magic", func(t *testing.T) {
		// Only 1 byte of magic present
		datagram := []byte{0xD0, 0, 0, 0, 1, 0, 0, 0, 5, 1, 2, 3}

		_, _, err := decodeFragmentHeader(datagram, nil)
		if err == nil {
			t.Error("Expected error for partial magic number")
		}

		t.Logf("Partial magic correctly rejected: %v", err)
	})

	// Test that very small messages (< 10 bytes) are not fragments
	t.Run("Small message without magic", func(t *testing.T) {
		smallMessage := []byte("hello")

		_, _, err := decodeFragmentHeader(smallMessage, nil)
		if err == nil {
			t.Error("Expected error for small message (too short for header)")
		}

		t.Logf("Small message correctly rejected as fragment: %v", err)
	})
}
