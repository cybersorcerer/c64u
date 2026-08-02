package debugger

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	colorPrimary   = lipgloss.Color("4")  // Blue
	colorBright    = lipgloss.Color("12") // Bright Blue
	colorGreen     = lipgloss.Color("2")
	colorYellow    = lipgloss.Color("3")
	colorRed       = lipgloss.Color("1")
	colorGray      = lipgloss.Color("8")
	colorWhite     = lipgloss.Color("7")
	colorHighlight = lipgloss.Color("11") // Bright Yellow
	colorOrange    = lipgloss.Color("9")  // Bright Red / Orange

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(colorBright).
			Padding(0, 1).
			Bold(true)

	pausedHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(colorOrange).
				Padding(0, 1).
				Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	pcStyle = lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true)

	flagOnStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	flagOffStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	currentLineStyle = lipgloss.NewStyle().
				Foreground(colorHighlight).
				Bold(true).
				Reverse(true)

	mnemonicStyle = lipgloss.NewStyle().
			Foreground(colorBright).
			Bold(true)

	addrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")) // Bright Cyan

	illegalStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	commentStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	ioWriteStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	ioReadStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")). // black text
			Background(colorBright).
			Padding(0, 1).
			Bold(true)

	statusPausedStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorOrange).
				Padding(0, 1).
				Bold(true)
)

// suppress unused-variable warnings for styles not yet used in View
var _ = flagOnStyle
var _ = flagOffStyle

// ── Messages ──────────────────────────────────────────────────────────────────

type tickMsg time.Time
type streamErrorMsg struct{ err error }

// ── Model ─────────────────────────────────────────────────────────────────────

const refreshRate = 200 * time.Millisecond

// Model is the Bubbletea model for the live debugger TUI.
type Model struct {
	tracker *Tracker
	conn    *net.UDPConn

	// program is set before startUDPReceiver; protected by programMu
	programMu sync.Mutex
	program   *tea.Program

	// paused is accessed from both UI goroutine and UDP goroutine
	pausedMu sync.Mutex
	paused   bool

	width  int
	height int

	// Frozen snapshot (only accessed from UI goroutine)
	frozenInstrs []DisasmLine
	frozenIO     []IOAccess
	frozenState  CPUState

	// Scroll offset — 0 = follow newest (only from UI goroutine)
	scroll int

	// Watchpoint-Eingabemodus
	watchInput    bool   // true = Eingabefeld aktiv
	watchInputBuf string // aktuell getippter Text
	watchInputErr string // Fehlermeldung bei ungültigem Ausdruck

	lastErr  string
	pktCount atomic.Uint64
}

func New(conn *net.UDPConn) *Model {
	return &Model{
		tracker: NewTracker(200, 60),
		conn:    conn,
	}
}

func (m *Model) isPaused() bool {
	m.pausedMu.Lock()
	defer m.pausedMu.Unlock()
	return m.paused
}

func (m *Model) setPaused(v bool) {
	m.pausedMu.Lock()
	defer m.pausedMu.Unlock()
	m.paused = v
}

func (m *Model) Init() tea.Cmd {
	return tickEvery(refreshRate)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Watchpoint-Eingabemodus fängt alle Tasten ab
		if m.watchInput {
			switch msg.String() {
			case "enter":
				expr := strings.TrimSpace(m.watchInputBuf)
				if expr != "" {
					wp, err := ParseWatchpoint(expr)
					if err != nil {
						m.watchInputErr = err.Error()
					} else {
						m.tracker.Watches.Add(wp)
						m.watchInput = false
						m.watchInputBuf = ""
						m.watchInputErr = ""
					}
				} else {
					m.watchInput = false
					m.watchInputErr = ""
				}
			case "esc":
				m.watchInput = false
				m.watchInputBuf = ""
				m.watchInputErr = ""
			case "backspace", "ctrl+h":
				if len(m.watchInputBuf) > 0 {
					m.watchInputBuf = m.watchInputBuf[:len(m.watchInputBuf)-1]
				}
			default:
				// Nur druckbare ASCII-Zeichen annehmen
				if len(msg.String()) == 1 {
					m.watchInputBuf += strings.ToUpper(msg.String())
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			// Close socket first so UDP goroutine unblocks, then quit
			m.conn.Close()
			// Nil out program so goroutine won't Send after quit
			m.programMu.Lock()
			m.program = nil
			m.programMu.Unlock()
			return m, tea.Quit

		case " ":
			if !m.isPaused() {
				// Freeze snapshot
				m.frozenInstrs = m.tracker.Instructions()
				m.frozenIO = m.tracker.IOAccesses()
				m.frozenState = m.tracker.State()
				m.scroll = 0
				m.setPaused(true)
			} else {
				m.setPaused(false)
				m.scroll = 0
			}

		case "up", "k":
			if m.isPaused() {
				m.scroll++
			}

		case "down", "j":
			if m.isPaused() && m.scroll > 0 {
				m.scroll--
			}

		case "pgup", "u":
			if m.isPaused() {
				m.scroll += m.visibleLines()
			}

		case "pgdown", "d":
			if m.isPaused() {
				m.scroll -= m.visibleLines()
				if m.scroll < 0 {
					m.scroll = 0
				}
			}

		case "home", "g":
			m.scroll = 0

		case "w":
			m.watchInput = true
			m.watchInputBuf = ""
			m.watchInputErr = ""

		case "W":
			wps := m.tracker.Watches.All()
			if len(wps) > 0 {
				// Letzten Watchpoint entfernen
				m.tracker.Watches.Remove(len(wps) - 1)
			}
		}

	case streamErrorMsg:
		// Socket closed on quit — suppress "use of closed network connection"
		if !strings.Contains(msg.err.Error(), "closed") {
			m.lastErr = msg.err.Error()
		}
		return m, nil

	case tickMsg:
		if !m.isPaused() && m.tracker.WatchTriggered() {
			m.frozenInstrs = m.tracker.Instructions()
			m.frozenIO = m.tracker.IOAccesses()
			m.frozenState = m.tracker.State()
			m.scroll = 0
			m.setPaused(true)
		}
		return m, tickEvery(refreshRate)
	}

	return m, nil
}

func (m *Model) visibleLines() int {
	lines := m.height - 12
	if lines < 5 {
		lines = 5
	}
	return lines
}

// ── Background UDP receiver ───────────────────────────────────────────────────

func (m *Model) startUDPReceiver() {
	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := m.conn.ReadFromUDP(buf)
			if err != nil {
				m.programMu.Lock()
				p := m.program
				m.programMu.Unlock()
				if p != nil {
					p.Send(streamErrorMsg{err: err})
				}
				return
			}
			if !m.isPaused() {
				entries := decodePacketEntries(buf[:n])
				for _, e := range entries {
					m.tracker.Feed(e)
				}
			}
			m.pktCount.Add(1)
		}
	}()
}

// ── Packet decoding ───────────────────────────────────────────────────────────

func decodePacketEntries(data []byte) []debug.Entry {
	const (
		headerSize = 4
		entrySize  = 4
	)
	if len(data) < headerSize+entrySize {
		return nil
	}
	payload := data[headerSize:]
	count := len(payload) / entrySize
	entries := make([]debug.Entry, 0, count)
	for i := 0; i < count; i++ {
		word := binary.LittleEndian.Uint32(payload[i*entrySize:])
		if word == 0 {
			continue
		}
		var e debug.Entry
		if word&(1<<31) == 0 {
			e = decode1541(word)
		} else {
			e = decode6510(word)
		}
		entries = append(entries, e)
	}
	return entries
}

func decode6510(w uint32) debug.Entry {
	return debug.Entry{
		PHI2:    w&(1<<31) != 0,
		Game:    w&(1<<30) == 0,
		ExROM:   w&(1<<29) == 0,
		BA:      w&(1<<28) != 0,
		IRQ:     w&(1<<27) == 0,
		ROM:     w&(1<<26) == 0,
		NMI:     w&(1<<25) == 0,
		RW:      w&(1<<24) != 0,
		Data:    uint8((w >> 16) & 0xFF),
		Address: uint16(w & 0xFFFF),
	}
}

func decode1541(w uint32) debug.Entry {
	return debug.Entry{
		Is1541:    true,
		ATN:       w&(1<<30) != 0,
		DataLine:  w&(1<<29) != 0,
		Clock:     w&(1<<28) != 0,
		Sync:      w&(1<<27) != 0,
		ByteReady: w&(1<<26) != 0,
		IRQ:       w&(1<<25) == 0, // IRQ# ist aktiv-low
		RW:        w&(1<<24) != 0,
		Data:      uint8((w >> 16) & 0xFF),
		Address:   uint16(w & 0xFFFF),
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.width == 0 {
		return "Initialisiere..."
	}

	paused := m.isPaused()
	var state CPUState
	var instrs []DisasmLine
	var ioAccesses []IOAccess

	if paused {
		state = m.frozenState
		instrs = m.frozenInstrs
		ioAccesses = m.frozenIO
	} else {
		state = m.tracker.State()
		instrs = m.tracker.Instructions()
		ioAccesses = m.tracker.IOAccesses()
	}

	header := m.renderHeader(state, paused)
	regPanel := m.renderRegisters(state)
	statusBar := m.renderStatusBar(paused)

	headerH := lipgloss.Height(header)
	regH := lipgloss.Height(regPanel)
	statusH := lipgloss.Height(statusBar)
	remaining := m.height - headerH - regH - statusH
	if remaining < 5 {
		remaining = 5
	}

	// disasmW and rightW are the TOTAL column widths (incl. each panel's border
	// and padding); they sum to m.width so the two columns fill the row exactly.
	disasmW := m.width * 55 / 100
	rightW := m.width - disasmW
	ioH := remaining * 60 / 100
	watchH := remaining - ioH

	disasmPanel := m.renderDisasm(instrs, disasmW, remaining, paused)
	ioPanel := m.renderIOLog(ioAccesses, rightW, ioH)
	watchPanel := m.renderWatchPanel(rightW, watchH)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, ioPanel, watchPanel)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, disasmPanel, rightCol)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		regPanel,
		bottomRow,
		statusBar,
	)
}

func (m *Model) renderHeader(state CPUState, paused bool) string {
	pauseStr := ""
	if paused {
		pauseStr = "  ⏸ PAUSED"
	}
	title := fmt.Sprintf(" C64 Disassembler%s  •  PC: %s  •  Packets: %d  •  Instructions: %d ",
		pauseStr,
		pcStyle.Render(fmt.Sprintf("$%04X", state.PC)),
		m.pktCount.Load(),
		state.InstrCount,
	)
	if paused {
		return pausedHeaderStyle.Width(m.width).Render(title)
	}
	return headerStyle.Width(m.width).Render(title)
}

func (m *Model) renderRegisters(state CPUState) string {
	regStyle := lipgloss.NewStyle().Foreground(colorWhite)
	labelStyle := lipgloss.NewStyle().Foreground(colorGray)
	flagOnStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	flagOffStyle := lipgloss.NewStyle().Foreground(colorGray)

	flagDefs := []struct {
		name string
		mask uint8
	}{
		{"N", FlagN}, {"V", FlagV}, {"-", 0x40}, {"B", FlagB},
		{"D", FlagD}, {"I", FlagI}, {"Z", FlagZ}, {"C", FlagC},
	}

	var flagParts []string
	for _, f := range flagDefs {
		set := state.Status&f.mask != 0
		if set {
			flagParts = append(flagParts, flagOnStyle.Render(f.name))
		} else {
			flagParts = append(flagParts, flagOffStyle.Render(f.name))
		}
	}

	content := fmt.Sprintf("%s %s  %s %s  %s %s",
		labelStyle.Render("PC:"),
		pcStyle.Render(fmt.Sprintf("%04X", state.PC)),
		labelStyle.Render("SP:"),
		regStyle.Render(fmt.Sprintf("%02X", state.SP)),
		labelStyle.Render("SR:"),
		strings.Join(flagParts, ""),
	)
	return panelStyle.Width(panelInnerW(m.width)).Render(content)
}

func (m *Model) renderDisasm(instrs []DisasmLine, width, height int, paused bool) string {
	innerH := height - 3
	if innerH < 1 {
		innerH = 1
	}

	n := len(instrs)
	if n == 0 {
		title := headerStyle.Render(" Disassembly — waiting for data... ")
		return panelStyle.Width(panelInnerW(width)).Height(panelInnerH(height)).Render(title)
	}

	// currentPos: Position des aktuellen Eintrags im virtuellen Gesamtstrom.
	// Der Puffer rotiert mit fester Größe instrMax=200, aber state.InstrCount
	// zählt wie viele Instruktionen insgesamt gesehen wurden. Das Fenster muss
	// sich mit jeder neuen Instruktion bewegen — deshalb basieren wir windowStart
	// auf InstrCount, nicht auf n (das ist immer 200).
	// Live-Modus: letzte innerH Einträge, neuester ganz unten, kein Balken.
	// Pause-Modus: Fenster einfrieren, scroll geht rückwärts, letzter Eintrag
	//              bekommt den Pfeil damit man sieht wo der PC war.
	windowStart := n - innerH
	if paused {
		windowStart = n - innerH - m.scroll
	}
	if windowStart < 0 {
		windowStart = 0
	}

	lines := make([]string, innerH)
	for i := range lines {
		lines[i] = ""
	}
	// Leerzeilen oben wenn Puffer noch nicht voll
	offset := innerH - (n - windowStart)
	if offset < 0 {
		offset = 0
	}
	lineIdx := offset
	for bufIdx := windowStart; bufIdx < n && lineIdx < innerH; bufIdx++ {
		dl := instrs[bufIdx]
		isCur := (bufIdx == n-1) && paused
		r := dl.Result
		hexPart := ""
		for _, b := range r.Bytes {
			hexPart += fmt.Sprintf("%02X ", b)
		}
		if isCur {
			lines[lineIdx] = currentLineStyle.Render(fmt.Sprintf(
				"▶ $%04X  %-9s  %-4s %-12s %s",
				r.Address, hexPart, r.Mnemonic, r.Operand, r.Comment,
			))
		} else {
			addr := addrStyle.Render(fmt.Sprintf("$%04X", r.Address))
			hex := addrStyle.Render(fmt.Sprintf("%-9s", hexPart))
			var mnem string
			if r.Illegal {
				mnem = illegalStyle.Render(fmt.Sprintf("*%-4s", r.Mnemonic))
			} else {
				mnem = mnemonicStyle.Render(fmt.Sprintf("%-4s", r.Mnemonic))
			}
			operand := lipgloss.NewStyle().Foreground(colorWhite).Render(fmt.Sprintf("%-12s", r.Operand))
			comment := ""
			if r.Comment != "" {
				comment = commentStyle.Render("; " + r.Comment)
			}
			lines[lineIdx] = fmt.Sprintf("  %s  %s  %s %s %s", addr, hex, mnem, operand, comment)
		}
		lineIdx++
	}

	title := headerStyle.Render(" Disassembly ")
	if paused {
		title = pausedHeaderStyle.Render(fmt.Sprintf(" Disassembly  -%d ", m.scroll))
	}
	return panelStyle.Width(panelInnerW(width)).Height(panelInnerH(height)).Render(title + "\n" + strings.Join(lines, "\n"))
}

// Lipgloss Style.Width/Height set the size INCLUDING padding but EXCLUDING the
// border; the border is drawn on top. So to make a bordered panel occupy exactly
// `total` cells we pass total minus only the border size (2 per axis here).
const panelBorderX = 2
const panelBorderY = 2

// panelInnerH returns the Height value so the rendered panel occupies `total` rows.
func panelInnerH(total int) int {
	h := total - panelBorderY
	if h < 1 {
		h = 1
	}
	return h
}

// panelInnerW returns the Width value so the rendered panel occupies `total` cols.
func panelInnerW(total int) int {
	w := total - panelBorderX
	if w < 1 {
		w = 1
	}
	return w
}

func (m *Model) renderIOLog(accesses []IOAccess, width, height int) string {
	innerH := height - 3
	if innerH < 1 {
		innerH = 1
	}

	end := len(accesses)
	start := end - innerH
	if start < 0 {
		start = 0
	}
	visible := accesses[start:]

	lines := make([]string, 0, innerH)
	for _, acc := range visible {
		rw := ioReadStyle.Render("R")
		if acc.Write {
			rw = ioWriteStyle.Render("W")
		}
		name := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render(acc.Comment)
		val := addrStyle.Render(fmt.Sprintf("$%02X", acc.Data))
		addr := addrStyle.Render(fmt.Sprintf("$%04X", acc.Address))
		lines = append(lines, fmt.Sprintf("%s %s %-16s = %s", rw, addr, name, val))
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}

	title := headerStyle.Render(" I/O Access ")
	return panelStyle.Width(panelInnerW(width)).Height(panelInnerH(height)).Render(title + "\n" + strings.Join(lines, "\n"))
}

func (m *Model) renderWatchPanel(width, height int) string {
	wps := m.tracker.Watches.All()

	labelStyle := lipgloss.NewStyle().Foreground(colorGray)
	addrBoldStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	hitStyle := lipgloss.NewStyle().Foreground(colorWhite)
	condStyle := lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	trigStyle := lipgloss.NewStyle().Foreground(colorRed).Bold(true).Reverse(true)
	promptStyle := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(colorRed)
	hintStyle := lipgloss.NewStyle().Foreground(colorGray)

	innerH := height - 3
	if innerH < 1 {
		innerH = 1
	}

	lines := make([]string, 0, innerH)

	if m.watchInput {
		prompt := promptStyle.Render("Address> ")
		input := m.watchInputBuf + "█"
		lines = append(lines, prompt+input)
		if m.watchInputErr != "" {
			lines = append(lines, errStyle.Render("  "+m.watchInputErr))
		}
		lines = append(lines, hintStyle.Render("  Format: D020  oder  D020=05"))
		lines = append(lines, hintStyle.Render("  Enter: OK   Esc: Abbrechen"))
	}

	for i, wp := range wps {
		addrPart := addrBoldStyle.Render(fmt.Sprintf("$%04X", wp.Address))
		var condPart string
		if wp.HasCondition {
			condPart = condStyle.Render(fmt.Sprintf("=%02X", wp.ConditionValue))
		}
		valPart := hitStyle.Render(fmt.Sprintf("→$%02X", wp.LastValue))
		hitPart := labelStyle.Render(fmt.Sprintf(" ×%d", wp.HitCount))
		idx := labelStyle.Render(fmt.Sprintf("%2d. ", i+1))
		line := idx + addrPart + condPart + " " + valPart + hitPart
		if wp.ConditionMet {
			line = trigStyle.Render(fmt.Sprintf(" BREAK $%04X=%02X ", wp.Address, wp.LastValue))
		}
		lines = append(lines, line)
	}

	if len(wps) == 0 && !m.watchInput {
		lines = append(lines, labelStyle.Render("  (no watchpoints)"))
		lines = append(lines, labelStyle.Render("  w: add watchpoint"))
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	title := headerStyle.Render(fmt.Sprintf(" Watchpoints (%d) ", len(wps)))
	return panelStyle.Width(panelInnerW(width)).Height(panelInnerH(height)).Render(title + "\n" + strings.Join(lines, "\n"))
}

func (m *Model) renderStatusBar(paused bool) string {
	var text string
	if paused {
		text = "PAUSED  ↑/k: back  ↓/j: forward  fn+↑/u: page up  fn+↓/d: page down  Space: resume  q: quit"
	} else {
		text = "Space: pause  w: add watch  W: remove watch  q/Ctrl+C: quit"
	}
	if m.lastErr != "" {
		text = "ERROR: " + m.lastErr
	}
	if paused {
		return statusPausedStyle.Width(m.width).Render(text)
	}
	return statusStyle.Width(m.width).Render(text)
}

// Run starts the debugger TUI.
func Run(conn *net.UDPConn) error {
	m := New(conn)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.programMu.Lock()
	m.program = p
	m.programMu.Unlock()
	m.startUDPReceiver()
	_, err := p.Run()
	return err
}
