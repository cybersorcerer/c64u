package video

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"net"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
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

// Game implements ebiten.Game and holds the framebuffer
type Game struct {
	mu       sync.Mutex
	img      *image.RGBA
	tex      *ebiten.Image
	dirty    bool
	stopFn   func() error
	conn     *net.UDPConn
}

func newGame(conn *net.UDPConn, stopFn func() error) *Game {
	return &Game{
		img:    image.NewRGBA(image.Rect(0, 0, PixelsPerLine, FrameLines)),
		tex:    ebiten.NewImage(PixelsPerLine, FrameLines),
		conn:   conn,
		stopFn: stopFn,
	}
}

func (g *Game) Update() error {
	if ebiten.IsWindowBeingClosed() {
		g.conn.Close()
		g.stopFn()
		return ebiten.Termination
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
func Listen(localIP string, startFn func(ip string) error, stopFn func() error) error {
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

	g := newGame(conn, stopFn)
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
