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

// DecodePacket reads a UDP packet, decodes every entry and writes the
// readable form to w.
// raw=true: structured output, raw=false: address and data only
func DecodePacket(data []byte, w io.Writer, raw bool) {
	if len(data) < minPacketSize {
		return
	}

	seq := binary.LittleEndian.Uint16(data[0:2])
	payload := data[pktHeaderSize:]
	count := len(payload) / entrySize

	if !raw {
		fmt.Fprintf(w, "--- PKT #%05d (%d entries) ---\n", seq, count)
	}

	for i := 0; i < count; i++ {
		word := binary.LittleEndian.Uint32(payload[i*entrySize:])
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
