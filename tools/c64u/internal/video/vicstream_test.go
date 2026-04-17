//go:build darwin

package video

import (
	"encoding/binary"
	"testing"
)

// buildPacket creates a minimal valid video packet for testing.
// lineNum: raw line number (may include bit15 for last-packet flag)
// lines: number of lines in packet (always 4 in real stream)
// colors: palette index repeated for all pixels
func buildPacket(lineNum uint16, lines uint8, color uint8) []byte {
	buf := make([]byte, headerSize+int(lines)*(PixelsPerLine/2))
	binary.LittleEndian.PutUint16(buf[0:2], 0)              // seq
	binary.LittleEndian.PutUint16(buf[2:4], 0)              // frame
	binary.LittleEndian.PutUint16(buf[4:6], lineNum)        // line
	binary.LittleEndian.PutUint16(buf[6:8], PixelsPerLine)  // width
	buf[8] = lines                                           // lines per packet
	buf[9] = 4                                               // bits per pixel
	binary.LittleEndian.PutUint16(buf[10:12], 0)            // encoding

	// Fill pixel data: each byte = two nibbles of same color
	nibble := (color & 0x0F) | ((color & 0x0F) << 4)
	for i := headerSize; i < len(buf); i++ {
		buf[i] = nibble
	}
	return buf
}

func TestPixelUnpack_CorrectColors(t *testing.T) {
	// Color 5 (green) in both nibbles
	pkt := buildPacket(0, 4, 5)

	lineNum := binary.LittleEndian.Uint16(pkt[4:6])
	lines := int(pkt[8])
	bytesPerLine := PixelsPerLine / 2
	raw := pkt[headerSize:]
	baseLine := int(lineNum & 0x7FFF)

	work := [FrameLines][PixelsPerLine]uint8{}
	for l := 0; l < lines; l++ {
		row := baseLine + l
		lineStart := l * bytesPerLine
		for x := 0; x < bytesPerLine; x++ {
			b := raw[lineStart+x]
			work[row][x*2] = b & 0x0F
			work[row][x*2+1] = (b >> 4) & 0x0F
		}
	}

	for l := 0; l < lines; l++ {
		for x := 0; x < PixelsPerLine; x++ {
			if work[l][x] != 5 {
				t.Errorf("pixel [%d][%d] = %d, want 5", l, x, work[l][x])
			}
		}
	}
}

func TestLastPacketFlag(t *testing.T) {
	// bit 15 set = last packet of frame
	pkt := buildPacket(0x8000|268, 4, 0)
	lineNum := binary.LittleEndian.Uint16(pkt[4:6])

	isLast := lineNum&0x8000 != 0
	baseLine := int(lineNum & 0x7FFF)

	if !isLast {
		t.Error("expected isLast=true for lineNum with bit15 set")
	}
	if baseLine != 268 {
		t.Errorf("baseLine = %d, want 268", baseLine)
	}
}

func TestHeaderSize(t *testing.T) {
	if headerSize != 12 {
		t.Errorf("headerSize = %d, want 12", headerSize)
	}
}

func TestPacketPayloadSize(t *testing.T) {
	// 4 lines * 384 pixels / 2 bytes per pixel = 768 bytes payload + 12 header = 780 total
	expected := 4*(PixelsPerLine/2) + headerSize
	if expected != 780 {
		t.Errorf("packet size = %d, want 780", expected)
	}
}

func TestPaletteSize(t *testing.T) {
	if len(vicPalette) != 16 {
		t.Errorf("palette has %d entries, want 16", len(vicPalette))
	}
	// Black must be index 0
	if vicPalette[0].R != 0 || vicPalette[0].G != 0 || vicPalette[0].B != 0 {
		t.Errorf("color 0 (black) = %v, want {0,0,0}", vicPalette[0])
	}
	// White must be index 1
	if vicPalette[1].R != 0xFF || vicPalette[1].G != 0xFF || vicPalette[1].B != 0xFF {
		t.Errorf("color 1 (white) = %v, want {255,255,255}", vicPalette[1])
	}
}
