package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/ebitengine/oto/v3"
)

const (
	AudioPort      = 11001
	headerSize     = 2
	samplesPerPkt  = 192
	bytesPerSample = 4 // 16-bit stereo = 2 ch * 2 bytes
	payloadSize    = samplesPerPkt * bytesPerSample // 768
	packetSize     = headerSize + payloadSize       // 770
	sampleRate     = 48000
	bufferSize     = payloadSize * 8 // ~8 packets of jitter buffer
)

// pcmReader is an io.Reader that oto pulls audio from.
// It blocks until data is available.
type pcmReader struct {
	mu   sync.Mutex
	buf  []byte
	cond *sync.Cond
	done bool
}

func newPCMReader() *pcmReader {
	r := &pcmReader{}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *pcmReader) push(data []byte) {
	r.mu.Lock()
	// Keep buffer bounded to avoid runaway memory on slow readers
	if len(r.buf) < bufferSize*4 {
		r.buf = append(r.buf, data...)
	}
	r.cond.Signal()
	r.mu.Unlock()
}

func (r *pcmReader) close() {
	r.mu.Lock()
	r.done = true
	r.cond.Broadcast()
	r.mu.Unlock()
}

func (r *pcmReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.buf) == 0 && !r.done {
		r.cond.Wait()
	}
	if len(r.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// Listen opens a UDP socket on AudioPort, starts the audio stream via startFn,
// and plays back the received PCM audio until interrupted.
func Listen(localIP string, startFn func(ip string) error, stopFn func() error) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", AudioPort))
	if err != nil {
		return fmt.Errorf("resolve UDP: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen UDP :%d: %w", AudioPort, err)
	}

	if err := startFn(localIP); err != nil {
		conn.Close()
		return err
	}
	defer func() {
		conn.Close()
		stopFn()
	}()

	// Initialize oto audio context: 48kHz, stereo, 16-bit signed LE
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return fmt.Errorf("init audio: %w", err)
	}
	<-ready

	pcm := newPCMReader()
	player := ctx.NewPlayer(pcm)
	player.Play()
	defer func() {
		player.Close()
		pcm.close()
	}()

	var lastSeq uint16
	first := true
	buf := make([]byte, 2048)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if n < headerSize+bytesPerSample {
			continue
		}

		seq := binary.LittleEndian.Uint16(buf[0:2])
		if !first {
			expected := lastSeq + 1
			if seq != expected {
				// Fill missing packets with silence to keep timing
				missing := int(seq - expected)
				if missing > 0 && missing < 100 {
					silence := make([]byte, missing*payloadSize)
					pcm.push(silence)
				}
			}
		}
		first = false
		lastSeq = seq

		// Push raw PCM payload (skip 2-byte sequence header)
		pcm.push(buf[headerSize:n])
	}

	return nil
}
