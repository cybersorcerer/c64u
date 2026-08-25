//go:build darwin

package video

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"net"
	"sync"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/petscii"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	VideoPort     = 11000
	headerSize    = 12
	PixelsPerLine = 384
	FrameLines    = 272 // PAL
	windowScale   = 2   // 768x544 native window
)

// VIC palette: 16 colors
var vicPalette = [16]color.RGBA{
	{0x00, 0x00, 0x00, 0xFF}, // 0 Black
	{0xFF, 0xFF, 0xFF, 0xFF}, // 1 White
	{0x88, 0x20, 0x00, 0xFF}, // 2 Red
	{0x68, 0xD4, 0xA8, 0xFF}, // 3 Cyan
	{0xBB, 0x3F, 0xB8, 0xFF}, // 4 Purple
	{0x55, 0xA0, 0x49, 0xFF}, // 5 Green
	{0x20, 0x27, 0x9D, 0xFF}, // 6 Blue
	{0xED, 0xF1, 0x71, 0xFF}, // 7 Yellow
	{0xB9, 0x74, 0x18, 0xFF}, // 8 Orange
	{0x78, 0x53, 0x00, 0xFF}, // 9 Brown
	{0xDD, 0x77, 0x66, 0xFF}, // A Light Red
	{0x55, 0x55, 0x55, 0xFF}, // B Dark Grey
	{0x88, 0x88, 0x88, 0xFF}, // C Medium Grey
	{0xAA, 0xFF, 0x9E, 0xFF}, // D Light Green
	{0x70, 0x7C, 0xE6, 0xFF}, // E Light Blue
	{0xBB, 0xBB, 0xBB, 0xFF}, // F Light Grey
}

// keyMap maps Ebiten keys to raw PETSCII bytes for keys that cannot be
// expressed as printable ASCII (cursor keys, function keys, control keys).
var keyMap = []struct {
	key     ebiten.Key
	petscii byte
}{
	{ebiten.KeyArrowUp, 0x91},
	{ebiten.KeyArrowDown, 0x11},
	{ebiten.KeyArrowLeft, 0x9D},
	{ebiten.KeyArrowRight, 0x1D},
	// Function key codes are interleaved, not sequential: F2, F4, F6 and F8
	// are the shifted variants of F1, F3, F5 and F7.
	{ebiten.KeyF1, 0x85},
	{ebiten.KeyF2, 0x89},
	{ebiten.KeyF3, 0x86},
	{ebiten.KeyF4, 0x8A},
	{ebiten.KeyF5, 0x87},
	{ebiten.KeyF6, 0x8B},
	{ebiten.KeyF7, 0x88},
	{ebiten.KeyF8, 0x8C},
	{ebiten.KeyEnter, 0x0D},
	{ebiten.KeyBackspace, 0x14},
	{ebiten.KeyDelete, 0x14},
	{ebiten.KeyHome, 0x13},
}

// cbmMap maps keys pressed together with Left Alt (CBM modifier) to PETSCII.
// Alt+Shift acts as CBM+Shift → toggle charset (0x08).
var cbmMap = []struct {
	key     ebiten.Key
	petscii byte
}{
	{ebiten.KeyShiftLeft, 0x08},
	{ebiten.KeyShiftRight, 0x08},
}

const escConfirmFrames = 60 * 3 // 3 seconds at 60 TPS

// Game implements ebiten.Game and holds the framebuffer
type Game struct {
	mu            sync.Mutex
	img           *image.RGBA
	tex           *ebiten.Image
	dirty         bool
	stopFn        func() error
	resetFn       func() error
	conn          *net.UDPConn
	sendFn        func([]byte) error
	escPending    bool
	escFramesLeft int
}

func newGame(conn *net.UDPConn, stopFn func() error, sendFn func([]byte) error, resetFn func() error) *Game {
	return &Game{
		img:     image.NewRGBA(image.Rect(0, 0, PixelsPerLine, FrameLines)),
		tex:     ebiten.NewImage(PixelsPerLine, FrameLines),
		conn:    conn,
		stopFn:  stopFn,
		sendFn:  sendFn,
		resetFn: resetFn,
	}
}

func (g *Game) Update() error {
	if ebiten.IsWindowBeingClosed() {
		g.conn.Close()
		g.stopFn()
		return ebiten.Termination
	}

	if g.escPending {
		g.escFramesLeft--
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.escPending = false
			if g.resetFn != nil {
				g.resetFn() //nolint:errcheck
			}
			return nil
		}
		// Any other key cancels reset-confirm and forwards the key normally
		if g.escFramesLeft <= 0 {
			g.escPending = false
		}
	}

	if g.sendFn != nil && !g.escPending {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.escPending = true
			g.escFramesLeft = escConfirmFrames
			return nil
		}

		cbm := ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)

		if cbm {
			for _, m := range cbmMap {
				if inpututil.IsKeyJustPressed(m.key) {
					g.sendFn([]byte{m.petscii}) //nolint:errcheck
				}
			}
		} else {
			for _, m := range keyMap {
				if inpututil.IsKeyJustPressed(m.key) {
					g.sendFn([]byte{m.petscii}) //nolint:errcheck
				}
			}
			// Printable chars via AppendInputChars — layout-aware, then PETSCII-encoded
			chars := ebiten.AppendInputChars(nil)
			for _, r := range chars {
				if r < 0x80 {
					if encoded, err := petscii.Encode(string(r)); err == nil {
						g.sendFn(encoded) //nolint:errcheck
					}
				}
			}
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.mu.Lock()
	dirty := g.dirty
	if dirty {
		g.tex.WritePixels(g.img.Pix)
		g.dirty = false
	}
	g.mu.Unlock()

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(windowScale, windowScale)
	screen.DrawImage(g.tex, op)

	if g.escPending {
		ebiten.SetWindowTitle("C64 Ultimate — Press ESC again to RESET  |  Any other key cancels")
	} else {
		ebiten.SetWindowTitle("C64 Ultimate — Video Stream")
	}
}

func (g *Game) Layout(outsideW, outsideH int) (int, int) {
	return PixelsPerLine * windowScale, FrameLines * windowScale
}

// receiveLoop reads UDP packets and fills the framebuffer
func (g *Game) receiveLoop() {
	buf := make([]byte, 2048)
	// working buffer for the current in-progress frame
	work := image.NewRGBA(image.Rect(0, 0, PixelsPerLine, FrameLines))

	for {
		n, _, err := g.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < headerSize {
			continue
		}

		lineNum := binary.LittleEndian.Uint16(buf[4:6])
		lines := int(buf[8])
		bpp := buf[9]
		if bpp != 4 || lines == 0 {
			continue
		}

		isLast := lineNum&0x8000 != 0
		baseLine := int(lineNum & 0x7FFF)

		// Payload: lines * (PixelsPerLine/2) bytes
		// Each byte = 2 pixels: low nibble = left pixel, high nibble = right pixel
		bytesPerLine := PixelsPerLine / 2 // 192
		raw := buf[headerSize:n]
		for l := 0; l < lines; l++ {
			row := baseLine + l
			if row >= FrameLines {
				break
			}
			lineStart := l * bytesPerLine
			if lineStart+bytesPerLine > len(raw) {
				break
			}
			for x := 0; x < bytesPerLine; x++ {
				b := raw[lineStart+x]
				c0 := vicPalette[b&0x0F]
				c1 := vicPalette[(b>>4)&0x0F]
				off0 := (row*PixelsPerLine + x*2) * 4
				off1 := (row*PixelsPerLine + x*2 + 1) * 4
				work.Pix[off0] = c0.R
				work.Pix[off0+1] = c0.G
				work.Pix[off0+2] = c0.B
				work.Pix[off0+3] = 0xFF
				work.Pix[off1] = c1.R
				work.Pix[off1+1] = c1.G
				work.Pix[off1+2] = c1.B
				work.Pix[off1+3] = 0xFF
			}
		}

		if isLast {
			g.mu.Lock()
			copy(g.img.Pix, work.Pix)
			g.dirty = true
			g.mu.Unlock()
		}
	}
}

// Listen starts the video stream, opens a native window and renders frames.
// sendFn is called for each keystroke; resetFn is called on double-Escape.
func Listen(localIP string, startFn func(ip string) error, stopFn func() error, sendFn func([]byte) error, resetFn func() error) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", VideoPort))
	if err != nil {
		return fmt.Errorf("resolve UDP: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen UDP :%d: %w", VideoPort, err)
	}

	if err := startFn(localIP); err != nil {
		conn.Close()
		return err
	}

	g := newGame(conn, stopFn, sendFn, resetFn)
	go g.receiveLoop()

	ebiten.SetWindowSize(PixelsPerLine*windowScale, FrameLines*windowScale)
	ebiten.SetWindowTitle("C64 Ultimate — Video Stream")
	ebiten.SetWindowClosingHandled(true)
	ebiten.SetTPS(60)

	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		return err
	}
	return nil
}
