package debug

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// entry builds a 6510 stream word in the documented layout.
func entry(flags, data uint8, addr uint16) uint32 {
	return uint32(flags)<<24 | uint32(data)<<16 | uint32(addr)
}

// documentedPacket is a datagram as the documentation describes it: a
// sequence number, a zero reserved word, then the entries.
func documentedPacket(seq uint16, words []uint32) []byte {
	out := binary.LittleEndian.AppendUint32(nil, uint32(seq))
	for _, w := range words {
		out = binary.LittleEndian.AppendUint32(out, w)
	}
	return out
}

// continuousStream cuts the entries, as one run of bytes, into datagrams of
// size bytes (a multiple of 4, as 1444 is) the way the C64 Ultimate does: the first datagram starts with
// two bytes that belong to nothing, so every entry starts at byte 2 of its
// datagram and one of them straddles each boundary.
func continuousStream(words []uint32, size int) [][]byte {
	stream := []byte{0, 0}
	for _, w := range words {
		stream = binary.LittleEndian.AppendUint32(stream, w)
	}
	var out [][]byte
	for len(stream) >= size {
		out = append(out, stream[:size])
		stream = stream[size:]
	}
	return out
}

func manyEntries(n int) []uint32 {
	words := make([]uint32, n)
	for i := range words {
		words[i] = entry(0xFF, uint8(i), 0x1000+uint16(i)) // distinct, never zero in the upper half
	}
	return words
}

var sample = []uint32{
	entry(0xFF, 0xA5, 0xE5CE), // read $A5 from $E5CE
	entry(0xFE, 0x07, 0xD020), // write $07 to $D020
	entry(0xF7, 0x4C, 0xEA31), // read with IRQ# asserted
}

func TestDecoder_DocumentedFraming(t *testing.T) {
	var d Decoder
	seq, words, ok := d.Words(documentedPacket(7, sample))
	if !ok || seq != 7 || d.Continuous() {
		t.Fatalf("ok=%v seq=%d continuous=%v, want true 7 false", ok, seq, d.Continuous())
	}
	for i, w := range sample {
		if words[i] != w {
			t.Errorf("word %d = %08x, want %08x", i, words[i], w)
		}
	}
}

func TestDecoder_ContinuousFramingReassemblesTheEntries(t *testing.T) {
	want := manyEntries(1500) // 6002 bytes: four 1444-byte datagrams and a remainder
	var d Decoder
	var got []uint32
	for _, p := range continuousStream(want, 1444) {
		_, words, ok := d.Words(p)
		if !ok {
			t.Fatal("datagram rejected")
		}
		got = append(got, words...)
	}
	if !d.Continuous() {
		t.Fatal("stream not recognised as continuous")
	}
	// 360 whole entries in the first datagram, then the straddling one plus
	// 360 in each of the other three.
	if len(got) != 360+3*361 {
		t.Fatalf("got %d entries, want %d", len(got), 360+3*361)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("entry %d = %08x, want %08x (entries lost or misaligned)", i, got[i], want[i])
		}
	}
}

// A zero in header bytes 2-3 says nothing once the stream is known to use the
// continuous framing: an access to address $0000 puts one there.
func TestDecoder_ContinuousStaysContinuous(t *testing.T) {
	dgs := continuousStream(manyEntries(1000), 1444)
	var d Decoder
	d.Words(dgs[0])
	if !d.Continuous() {
		t.Fatal("not recognised on the first datagram")
	}
	second := append([]byte(nil), dgs[1]...)
	second[2], second[3] = 0, 0
	d.Words(second)
	if !d.Continuous() {
		t.Error("fell back to the documented framing on a zero in bytes 2-3")
	}
}

// The first datagrams of a stream carry a (sequence number, 0) pair where the
// first entry would be. It must not come out as an entry.
func TestDecoder_HeaderPairIsNotAnEntry(t *testing.T) {
	real := manyEntries(5)
	dg := []byte{0xAA, 0xFF} // upper half of an entry begun in the previous datagram
	dg = binary.LittleEndian.AppendUint16(dg, 9)
	dg = binary.LittleEndian.AppendUint16(dg, 0)
	for _, w := range real {
		dg = binary.LittleEndian.AppendUint32(dg, w)
	}
	dg = binary.LittleEndian.AppendUint16(dg, 0x1234) // lower half of the next entry

	var d Decoder
	_, words, ok := d.Words(dg)
	if !ok || len(words) != len(real) {
		t.Fatalf("ok=%v, %d entries, want %d", ok, len(words), len(real))
	}
	for i := range real {
		if words[i] != real[i] {
			t.Errorf("entry %d = %08x, want %08x", i, words[i], real[i])
		}
	}
	// ...and the straddling entry completes in the next datagram
	next := binary.LittleEndian.AppendUint16(nil, 0xFFF7)
	next = binary.LittleEndian.AppendUint32(next, real[0])
	next = binary.LittleEndian.AppendUint16(next, 0xBEEF) // lower half of yet another entry
	_, words, ok = d.Words(next)
	if !ok || len(words) != 2 || words[0] != 0xFFF71234 || words[1] != real[0] {
		t.Errorf("ok=%v words=%08x; want [fff71234 %08x]", ok, words, real[0])
	}
}

func TestDecoder_TooShort(t *testing.T) {
	var d Decoder
	if _, _, ok := d.Words([]byte{1, 2, 3, 4, 5, 6, 7}); ok {
		t.Error("a datagram without a whole entry was accepted")
	}
}

func TestDecodePacket_PrintsTheSameInBothFramings(t *testing.T) {
	entries := manyEntries(120)

	var documented bytes.Buffer
	var d1 Decoder
	d1.DecodePacket(documentedPacket(3, entries), &documented, true)

	var continuous bytes.Buffer
	var d2 Decoder
	for _, p := range continuousStream(entries, 204) { // 50 entries and half of one
		d2.DecodePacket(p, &continuous, true)
	}

	// The continuous stream ends with an entry still waiting for its other
	// half, so it can only be a leading part of the documented output.
	if continuous.Len() < documented.Len()/2 || !bytes.HasPrefix(documented.Bytes(), continuous.Bytes()) {
		t.Errorf("documented:\n%.400s\ncontinuous:\n%.400s", documented.String(), continuous.String())
	}
}

// Seven datagrams captured from a C64 Ultimate (firmware 1.1.0, "6510 Only")
// at the BASIC prompt, taken where the first datagrams' headers give way to
// bare entries. The CPU is in the KERNAL's keyboard-wait loop,
//
//	E5CD  A5 C6     LDA $C6
//	E5CF  85 CC     STA $CC
//	E5D1  8D 92 02  STA $0292
//	E5D4  F0 F7     BEQ $E5CD
//
// so every read of an address inside it must return the byte at that address.
func TestDecoder_RealCaptureFromC64Ultimate(t *testing.T) {
	raw, err := os.ReadFile("testdata/c64ultimate-fw1.1.0-6510.bin")
	if err != nil {
		t.Fatal(err)
	}
	const size = 1444
	if len(raw)%size != 0 {
		t.Fatalf("fixture is %d bytes, not a whole number of datagrams", len(raw))
	}
	rom := map[uint16]uint8{0xE5CD: 0xA5, 0xE5CE: 0xC6, 0xE5CF: 0x85, 0xE5D0: 0xCC,
		0xE5D1: 0x8D, 0xE5D2: 0x92, 0xE5D3: 0x02, 0xE5D4: 0xF0, 0xE5D5: 0xF7}

	var d Decoder
	reads, entries := 0, 0
	for n := 0; n*size < len(raw); n++ {
		_, words, ok := d.Words(raw[n*size : (n+1)*size])
		if !ok {
			t.Fatalf("datagram %d rejected", n)
		}
		for _, w := range words {
			entries++
			addr, data, read := uint16(w), uint8(w>>16), w&(1<<24) != 0
			if want, inLoop := rom[addr]; inLoop && read {
				reads++
				if data != want {
					t.Errorf("datagram %d: read of $%04X returned $%02X, ROM has $%02X", n, addr, data, want)
				}
			}
		}
	}
	if !d.Continuous() {
		t.Error("stream was not recognised as continuous")
	}
	if reads < 150 {
		t.Errorf("only %d reads of the wait loop in %d entries; the framing is probably wrong", reads, entries)
	}
}
