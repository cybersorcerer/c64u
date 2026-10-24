package debug

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Paketformat laut Dokumentation:
// Byte 0-1: Sequenznummer (16-bit LE)
// Byte 2-3: reserviert
// Byte 4...: 360 x 32-bit entries (1440 bytes) = 1444 bytes total

const (
	pktHeaderSize = 4
	entriesPerPkt = 360
	entrySize     = 4
	minPacketSize = pktHeaderSize + entrySize
)

// Entry is a decoded 6510/VIC bus access.
type Entry struct {
	PHI2      bool
	Game      bool // GAME# (active low)
	ExROM     bool // EXROM# (active low)
	BA        bool
	IRQ       bool // IRQ# (active low)
	ROM       bool // ROM# (active low)
	NMI       bool // NMI# (active low)
	RW        bool // true = Read, false = Write
	Data      uint8
	Address   uint16
	Is1541    bool // true for a 1541 stream entry
	ATN       bool
	DataLine  bool
	Clock     bool
	Sync      bool
	ByteReady bool
}

// decode1541 decodes a 1541 CPU entry.
func decode1541(w uint32) Entry {
	return Entry{
		Is1541:    true,
		PHI2:      false, // bit31 = '0' for the 1541
		ATN:       w&(1<<30) != 0,
		DataLine:  w&(1<<29) != 0,
		Clock:     w&(1<<28) != 0,
		Sync:      w&(1<<27) != 0,
		ByteReady: w&(1<<26) != 0,
		IRQ:       w&(1<<25) == 0, // IRQ# is active low
		RW:        w&(1<<24) != 0,
		Data:      uint8((w >> 16) & 0xFF),
		Address:   uint16(w & 0xFFFF),
	}
}

// decode6510 decodes a 6510/VIC entry.
func decode6510(w uint32) Entry {
	return Entry{
		PHI2:    w&(1<<31) != 0,
		Game:    w&(1<<30) == 0, // active low
		ExROM:   w&(1<<29) == 0, // active low
		BA:      w&(1<<28) != 0,
		IRQ:     w&(1<<27) == 0, // active low
		ROM:     w&(1<<26) == 0, // active low
		NMI:     w&(1<<25) == 0, // active low
		RW:      w&(1<<24) != 0,
		Data:    uint8((w >> 16) & 0xFF),
		Address: uint16(w & 0xFFFF),
	}
}

// format6510 renders a 6510/VIC entry as readable text.
func format6510(e Entry) string {
	rw := "R"
	if !e.RW {
		rw = "W"
	}
	phi2 := "0"
	if e.PHI2 {
		phi2 = "1"
	}
	flags := ""
	if !e.Game {
		flags += " GAME"
	}
	if !e.ExROM {
		flags += " EXROM"
	}
	if !e.IRQ {
		flags += " IRQ"
	}
	if !e.NMI {
		flags += " NMI"
	}
	return fmt.Sprintf("PHI2=%s %s $%04X = $%02X%s", phi2, rw, e.Address, e.Data, flags)
}

// format1541 renders a 1541 entry as readable text.
func format1541(e Entry) string {
	rw := "R"
	if !e.RW {
		rw = "W"
	}
	flags := ""
	if e.ATN {
		flags += " ATN"
	}
	if e.Clock {
		flags += " CLK"
	}
	if e.DataLine {
		flags += " DATA"
	}
	if e.Sync {
		flags += " SYNC"
	}
	return fmt.Sprintf("1541 %s $%04X = $%02X%s", rw, e.Address, e.Data, flags)
}

// Decoder turns the datagrams of one debug stream into 32-bit words in the
// documented bit layout.
//
// The documentation says a datagram is a 4-byte header (16-bit sequence
// number, 16 reserved bits, zero) followed by 360 little-endian 32-bit
// entries. The C64 Ultimate (seen on firmware 1.1.0) frames the same entries
// differently:
//
//   - the entries are one continuous stream that is cut into 1444-byte
//     datagrams wherever it happens to fall, so each datagram starts with the
//     upper half of an entry begun at the end of the previous one, and the
//     entries proper start at byte 2;
//   - only the first few datagrams of a stream (14-47 were seen) carry a
//     header, as a (sequence number, 0) pair in the place of the first entry;
//     after that there is no header and no sequence number at all.
//
// Read as documented, such a stream has every entry's address half paired
// with the control and data of the entry before it, so the disassembler never
// finds an instruction. Checked against the KERNAL's keyboard-wait loop at
// $E5CD-$E5D6 on a BASIC prompt: as documented, no entry has an address in
// the loop (0 of 888,480); with this framing every read in it returns the ROM
// byte (98,943 of 98,943).
//
// The two framings are told apart by header bytes 2-3, which are reserved
// and zero in the documented layout. On this device they hold a sequence
// number or an address, so one non-zero value settles it, and the choice then
// sticks: a later datagram can hold a zero there by chance. A stream's very
// first datagram has sequence number zero and is therefore decoded as
// documented, which is wrong for that one datagram.
//
// Use one Decoder per stream; it is not safe for concurrent use.
type Decoder struct {
	continuous bool   // device framing, see above
	haveTail   bool   // tail holds the lower half of an entry split across datagrams
	tail       uint16 // lower half (the address) of that entry
	count      uint16 // datagrams seen, numbers them when the stream has no sequence number
}

// Continuous reports whether the stream has been recognised as the device's
// continuous framing rather than the documented one.
func (d *Decoder) Continuous() bool { return d.continuous }

// Words returns the entries of a datagram as words in the documented layout,
// and its sequence number: the one in the header for the documented framing,
// otherwise the number of datagrams seen so far. ok is false for a datagram
// too short to hold an entry.
func (d *Decoder) Words(data []byte) (seq uint16, words []uint32, ok bool) {
	if len(data) < minPacketSize {
		return 0, nil, false
	}
	d.count++

	if !d.continuous && binary.LittleEndian.Uint16(data[2:4]) != 0 {
		d.continuous = true
	}

	if !d.continuous {
		seq = binary.LittleEndian.Uint16(data[0:2])
		payload := data[pktHeaderSize:]
		words = make([]uint32, len(payload)/entrySize)
		for i := range words {
			words[i] = binary.LittleEndian.Uint32(payload[i*entrySize:])
		}
		return seq, words, true
	}

	seq = d.count - 1
	words = make([]uint32, 0, len(data)/entrySize+1)

	// The entry split across the datagram boundary: its address half ended
	// the previous datagram, its control and data half starts this one.
	if d.haveTail {
		words = append(words, uint32(d.tail)|uint32(binary.LittleEndian.Uint16(data[0:2]))<<16)
	}

	i := 2
	for ; i+entrySize <= len(data); i += entrySize {
		w := binary.LittleEndian.Uint32(data[i:])
		// In the first datagrams the first "entry" is the (sequence
		// number, 0) header pair. No 6510 or VIC entry has zero control
		// and data bits, so a word with a zero upper half there is it.
		if i == 2 && w>>16 == 0 {
			continue
		}
		words = append(words, w)
	}

	d.haveTail = i+2 <= len(data)
	if d.haveTail {
		d.tail = binary.LittleEndian.Uint16(data[i:])
	}
	return seq, words, true
}

// DecodePacket reads a UDP packet, decodes every entry and writes the
// readable form to w.
// raw=true: structured output, raw=false: address and data only
func (d *Decoder) DecodePacket(data []byte, w io.Writer, raw bool) {
	seq, words, ok := d.Words(data)
	if !ok {
		return
	}
	count := len(words)

	if !raw {
		fmt.Fprintf(w, "--- PKT #%05d (%d entries) ---\n", seq, count)
	}

	for i := 0; i < count; i++ {
		word := words[i]
		if word == 0 {
			continue // skip blank lines
		}

		var line string
		if word&(1<<31) == 0 {
			// bit31=0 → 1541
			e := decode1541(word)
			line = format1541(e)
		} else {
			e := decode6510(word)
			line = format6510(e)
		}

		if raw {
			fmt.Fprintln(w, line)
		} else {
			fmt.Fprintf(w, "  %3d: %s\n", i, line)
		}
	}
}
