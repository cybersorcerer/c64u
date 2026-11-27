package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/audio"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/network"
)

type streamState int

const (
	streamStopped streamState = iota
	streamRunning
	streamError
)

type streamEntry struct {
	id    string
	label string
	state streamState
	err   error
	proc  *exec.Cmd // for video: child process
}

type StreamsModel struct {
	client  *api.Client
	host    string
	streams []*streamEntry
	cursor  int
	width   int
	height  int
	message string
}

type streamStartedMsg struct{ id string }
type streamStoppedMsg struct{ id string }
type streamErrMsg struct {
	id  string
	err error
}
type requestStreamMsg struct{ id string }

func NewStreamsModel(client *api.Client, host string) *StreamsModel {
	return &StreamsModel{
		client: client,
		host:   host,
		streams: []*streamEntry{
			{id: "video", label: "Video"},
			{id: "audio", label: "Audio"},
			{id: "debug", label: "Debug"},
		},
	}
}

func (m *StreamsModel) Init() tea.Cmd { return nil }

func (m *StreamsModel) active() *streamEntry {
	for _, s := range m.streams {
		if s.state == streamRunning {
			return s
		}
	}
	return nil
}

func (m *StreamsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "down", "j", "g", "G", "ctrl+d", "ctrl+u":
			m.cursor = applyListNav(msg.String(), m.cursor, len(m.streams), m.height-3)
		case "enter", " ":
			if m.cursor < 0 || m.cursor >= len(m.streams) {
				return m, nil
			}
			sel := m.streams[m.cursor]
			if sel.state == streamRunning {
				return m, m.stopCmd(sel)
			}
			// Stop any running stream first
			if act := m.active(); act != nil {
				m.doStop(act)
			}
			return m, m.startCmd(sel)
		}

	case streamStartedMsg:
		for _, s := range m.streams {
			if s.id == msg.id {
				s.state = streamRunning
				m.message = s.label + " stream started"
			}
		}

	case streamStoppedMsg:
		for _, s := range m.streams {
			if s.id == msg.id {
				s.state = streamStopped
				s.proc = nil
				m.message = s.label + " stream stopped"
			}
		}

	case streamErrMsg:
		for _, s := range m.streams {
			if s.id == msg.id {
				s.state = streamError
				s.err = msg.err
				s.proc = nil
				m.message = s.label + " error: " + msg.err.Error()
			}
		}
	}
	return m, nil
}

func (m *StreamsModel) doStop(s *streamEntry) {
	if s.proc != nil && s.proc.Process != nil {
		s.proc.Process.Kill() //nolint:errcheck
		s.proc = nil
	}
	m.client.StreamsStop(s.id) //nolint:errcheck
	s.state = streamStopped
}

func (m *StreamsModel) stopCmd(s *streamEntry) tea.Cmd {
	return func() tea.Msg {
		m.doStop(s)
		return streamStoppedMsg{s.id}
	}
}

// selfPath returns this binary, so a stream runs the build the user actually
// started. Naming the child "c64u" looked it up on PATH instead, which fails
// outright when the binary is not installed — running ./build/c64u, say — and
// would otherwise silently start a different version.
func selfPath() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return os.Args[0]
}

func (m *StreamsModel) startCmd(s *streamEntry) tea.Cmd {
	return func() tea.Msg {
		localIP, err := network.LocalIP(m.host)
		if err != nil {
			return streamErrMsg{s.id, fmt.Errorf("cannot detect local IP: %w", err)}
		}

		switch s.id {
		case "video":
			// video.Listen uses Ebiten, which has to own the main thread.
			// Spawn as a child process so the TUI stays alive.
			self := selfPath()
			args := []string{"streams", "listen", "video", "--host", m.host, "--no-color"}
			cmd := exec.Command(self, args...)
			var out bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &out
			s.proc = cmd
			if err := cmd.Start(); err != nil {
				return streamErrMsg{s.id, fmt.Errorf("cannot start %s: %w", self, err)}
			}
			// Das Kind meldet Fehler nur in seiner Ausgabe (Exit-Code 0); ein
			// Kill durch doStop ist ein normales Ende.
			wait := func() tea.Msg {
				err := cmd.Wait()
				var exit *exec.ExitError
				killed := errors.As(err, &exit) && !exit.Exited()
				if !killed && (err != nil || strings.Contains(out.String(), "✗ Error: ")) {
					return streamErrMsg{s.id, errors.New(childError(out.Bytes()))}
				}
				return streamStoppedMsg{s.id}
			}
			started := func() tea.Msg { return streamStartedMsg{s.id} }
			return tea.BatchMsg{started, wait}

		case "audio":
			startAudio := func(ip string) error {
				if err := m.client.SetConfigItem("Data Streams", "Stream Audio to",
					fmt.Sprintf("%s:%d", ip, audio.AudioPort)); err != nil {
					return err
				}
				resp, err := m.client.StreamsStart("audio", ip)
				if err != nil {
					return err
				}
				if resp.HasErrors() {
					return fmt.Errorf("%s", resp.Errors[0])
				}
				return nil
			}
			stopAudio := func() error {
				m.client.StreamsStop("audio") //nolint:errcheck
				return nil
			}
			listen := func(start func(string) error) error {
				return audio.Listen(localIP, start, stopAudio)
			}
			start := func(ip string) error {
				return network.StartWithRetry(startAudio, ip, io.Discard, time.Sleep)
			}
			if err := runUntilStarted(listen, start); err != nil {
				return streamErrMsg{s.id, err}
			}

		case "debug":
			// Debug TUI is blocking and needs the terminal — request TUI to quit
			// and hand control back to ui.go which will run the stream directly.
			return requestStreamMsg{"debug"}
		}

		return streamStartedMsg{s.id}
	}
}

// runUntilStarted runs listen in the background and waits until it has tried
// to start the stream: it returns the start's error, or listen's own error when
// it fails before starting. After a good start the listener keeps running.
func runUntilStarted(listen func(start func(string) error) error, start func(string) error) error {
	started := make(chan error, 2)
	go func() {
		started <- listen(func(ip string) error {
			err := start(ip)
			started <- err
			return err
		})
	}()
	return <-started
}

// childError is what the video child printed about its failure: the error
// line with its details, or else the last line it wrote.
func childError(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		msg, ok := strings.CutPrefix(line, "✗ Error: ")
		if !ok {
			continue
		}
		parts := []string{msg}
		for _, d := range lines[i+1:] {
			if detail, ok := strings.CutPrefix(strings.TrimSpace(d), "- "); ok {
				parts = append(parts, detail)
			}
		}
		return strings.Join(parts, ": ")
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

func (m *StreamsModel) View() string {
	var b strings.Builder

	b.WriteString(ItemStyle.Render("Streams"))
	b.WriteString("\n")

	for i, s := range m.streams {
		indicator := "○"
		stateStr := "Stopped"
		if s.state == streamRunning {
			indicator = "●"
			stateStr = "Running"
		} else if s.state == streamError {
			indicator = "✗"
			stateStr = "Error"
		}

		line := fmt.Sprintf("%s %-8s  %s", indicator, s.label, stateStr)
		if i == m.cursor {
			b.WriteString(SelectedItemStyle.Render("▶ "+line) + "\n")
		} else {
			b.WriteString(ItemStyle.Render("  "+line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ItemStyle.Render("Note: Video opens a separate window. Only one stream active at a time."))
	b.WriteString("\n")

	footer := m.message
	if footer == "" {
		footer = "↑/↓: select  Enter: start/stop  ?: help"
	}
	b.WriteString("\n" + StatusBarStyle.Width(m.width).Render(footer))

	return b.String()
}
