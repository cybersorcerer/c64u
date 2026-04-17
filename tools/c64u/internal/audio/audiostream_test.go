//go:build darwin

package audio

import (
	"encoding/binary"
	"testing"
)

// buildAudioPacket creates a test audio packet with given sequence number and sample value.
func buildAudioPacket(seq uint16, sampleVal int16) []byte {
	buf := make([]byte, headerSize+payloadSize)
	binary.LittleEndian.PutUint16(buf[0:2], seq)
	// Fill with stereo samples (left=sampleVal, right=sampleVal)
	for i := 0; i < samplesPerPkt; i++ {
		off := headerSize + i*bytesPerSample
		binary.LittleEndian.PutUint16(buf[off:], uint16(sampleVal))   // left
		binary.LittleEndian.PutUint16(buf[off+2:], uint16(sampleVal)) // right
	}
	return buf
}

func TestPacketSize(t *testing.T) {
	// Header (2) + 192 samples * 4 bytes = 770
	if headerSize+payloadSize != 770 {
		t.Errorf("packet size = %d, want 770", headerSize+payloadSize)
	}
}

func TestSamplesPerPacket(t *testing.T) {
	if samplesPerPkt != 192 {
		t.Errorf("samplesPerPkt = %d, want 192", samplesPerPkt)
	}
}

func TestBytesPerSample(t *testing.T) {
	// 16-bit stereo = 2 channels * 2 bytes
	if bytesPerSample != 4 {
		t.Errorf("bytesPerSample = %d, want 4", bytesPerSample)
	}
}

func TestPayloadSize(t *testing.T) {
	if payloadSize != 768 {
		t.Errorf("payloadSize = %d, want 768", payloadSize)
	}
}

func TestSequenceNumber(t *testing.T) {
	pkt := buildAudioPacket(42, 100)
	seq := binary.LittleEndian.Uint16(pkt[0:2])
	if seq != 42 {
		t.Errorf("seq = %d, want 42", seq)
	}
}

func TestSampleValues(t *testing.T) {
	pkt := buildAudioPacket(0, -1) // -1 in int16 = 0xFFFF
	// Check first left sample
	left := int16(binary.LittleEndian.Uint16(pkt[headerSize:]))
	if left != -1 {
		t.Errorf("left sample = %d, want -1", left)
	}
	// Check first right sample
	right := int16(binary.LittleEndian.Uint16(pkt[headerSize+2:]))
	if right != -1 {
		t.Errorf("right sample = %d, want -1", right)
	}
}

func TestSilencePadding(t *testing.T) {
	// Missing 2 packets should produce 2 * payloadSize bytes of silence
	missing := 2
	silence := make([]byte, missing*payloadSize)
	if len(silence) != 2*768 {
		t.Errorf("silence length = %d, want %d", len(silence), 2*768)
	}
	for i, b := range silence {
		if b != 0 {
			t.Errorf("silence[%d] = %d, want 0", i, b)
		}
	}
}

func TestSampleRate(t *testing.T) {
	if sampleRate != 48000 {
		t.Errorf("sampleRate = %d, want 48000", sampleRate)
	}
}
