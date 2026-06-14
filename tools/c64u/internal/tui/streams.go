package tui

import (
	"fmt"
	"os/exec"
	"strings"

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

func (m *StreamsModel) startCmd(s *streamEntry) tea.Cmd {
	return func() tea.Msg {
		localIP, err := network.LocalIP(m.host)
		if err != nil {
			return streamErrMsg{s.id, fmt.Errorf("cannot detect local IP: %w", err)}
		}

		switch s.id {
		case "video":
			// video.Listen uses Ebiten which requires the main thread on macOS.
			// Spawn as a child process so the TUI stays alive.
			args := []string{"streams", "listen", "video", "--host", m.host}
			cmd := exec.Command("c64u", args...)
			s.proc = cmd
			if err := cmd.Start(); err != nil {
				// fallback: try with full path from os.Executable
				return streamErrMsg{s.id, fmt.Errorf("cannot start video stream: %w", err)}
			}
			// Watch for process exit in background
			go func() {
				cmd.Wait() //nolint:errcheck
			}()

		case "audio":
			go func() {
				audio.Listen( //nolint:errcheck
					localIP,
					func(ip string) error {
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
					},
					func() error {
						m.client.StreamsStop("audio") //nolint:errcheck
						return nil
					},
				)
			}()

		case "debug":
			// Debug TUI is blocking and needs the terminal — request TUI to quit
			// and hand control back to ui.go which will run the stream directly.
			return requestStreamMsg{"debug"}
		}

		return streamStartedMsg{s.id}
	}
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
