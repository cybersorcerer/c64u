// Package crt writes VICE .crt cartridge images, the format the Ultimate
// expects in /flash/carts.
package crt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	signature     = "C64 CARTRIDGE   " // 16 bytes, space padded
	headerLen     = 0x40
	formatVersion = 0x0100
	nameLen       = 32

	// ChipROM is the chip packet type for read-only memory.
	ChipROM uint16 = 0

	// HardwareNormal is cartridge hardware type 0: a plain 8K, 16K or UMAX
	// cartridge. The Ultimate supports many more; this package does not
	// restrict the value.
	HardwareNormal uint16 = 0

	// HardwareMagicDesk is type 19. Its bank register at $DE00 has a disable
	// bit, so a utility cartridge can unmap itself after boot and leave the
	// full 38911 BASIC bytes free.
	HardwareMagicDesk uint16 = 19
)

// Cartridge describes a cartridge image.
//
// EXROM and GAME carry the C64 lines as stored in the header: 0 means the line
// is asserted. A plain 8K cartridge at $8000 uses EXROM 0 and GAME 1.
type Cartridge struct {
	Name        string
	HardwareTyp uint16
	SubType     uint8
	EXROM       uint8
	GAME        uint8
	Chips       []Chip
}

// Chip is one ROM image inside the cartridge.
type Chip struct {
	Type        uint16
	Bank        uint16
	LoadAddress uint16
	Data        []byte
}

// EightK returns a plain type 0 cartridge holding a single 8 KB ROM at $8000.
func EightK(name string, rom []byte) (*Cartridge, error) {
	return EightKType(name, rom, HardwareNormal)
}

// EightKType is EightK with an explicit hardware type, for cartridges that
// behave like an 8 KB ROM at $8000 but need their own type number - Magic Desk,
// for instance, whose disable bit lets a utility cartridge get out of the way
// once it has installed itself.
func EightKType(name string, rom []byte, hardwareType uint16) (*Cartridge, error) {
	if len(rom) != 0x2000 {
		return nil, fmt.Errorf("8K cartridge needs exactly 8192 bytes, got %d", len(rom))
	}
	return &Cartridge{
		Name:        name,
		HardwareTyp: hardwareType,
		EXROM:       0,
		GAME:        1,
		Chips: []Chip{{
			Type:        ChipROM,
			LoadAddress: 0x8000,
			Data:        rom,
		}},
	}, nil
}

// WriteTo emits the .crt image.
func (c *Cartridge) WriteTo(w io.Writer) (int64, error) {
	if len(c.Chips) == 0 {
		return 0, fmt.Errorf("cartridge has no chip packets")
	}

	var buf bytes.Buffer

	buf.WriteString(signature)
	binary.Write(&buf, binary.BigEndian, uint32(headerLen))
	binary.Write(&buf, binary.BigEndian, uint16(formatVersion))
	binary.Write(&buf, binary.BigEndian, c.HardwareTyp)
	buf.WriteByte(c.EXROM)
	buf.WriteByte(c.GAME)
	buf.WriteByte(c.SubType)
	buf.Write(make([]byte, 5)) // reserved

	name := make([]byte, nameLen)
	copy(name, c.Name) // truncated when longer, zero padded when shorter
	buf.Write(name)

	if buf.Len() != headerLen {
		return 0, fmt.Errorf("header is %d bytes, want %d", buf.Len(), headerLen)
	}

	for _, chip := range c.Chips {
		buf.WriteString("CHIP")
		binary.Write(&buf, binary.BigEndian, uint32(0x10+len(chip.Data)))
		binary.Write(&buf, binary.BigEndian, chip.Type)
		binary.Write(&buf, binary.BigEndian, chip.Bank)
		binary.Write(&buf, binary.BigEndian, chip.LoadAddress)
		binary.Write(&buf, binary.BigEndian, uint16(len(chip.Data)))
		buf.Write(chip.Data)
	}

	n, err := w.Write(buf.Bytes())
	return int64(n), err
}
