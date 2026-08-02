package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	ansiRE     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	viewNameRE = regexp.MustCompile(`View: *(.*)`)
)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestSetTabSelectsMatchingView(t *testing.T) {
	m := NewMainModel(nil, "")
	for i, tab := range tabs {
		m.SetTab(i)
		if m.ActiveTab() != i {
			t.Errorf("ActiveTab() = %d after SetTab(%d)", m.ActiveTab(), i)
		}
		if m.viewState != tab.State {
			t.Errorf("SetTab(%d) left viewState %v, want %v", i, m.viewState, tab.State)
		}
	}
}

func TestSetTabIgnoresOutOfRange(t *testing.T) {
	m := NewMainModel(nil, "")
	m.SetTab(len(tabs) - 1)
	before := m.ActiveTab()
	m.SetTab(-1)
	m.SetTab(len(tabs))
	if m.ActiveTab() != before {
		t.Errorf("out-of-range SetTab changed the tab to %d, want %d", m.ActiveTab(), before)
	}
}

// Every tab needs its own help section. The Streams view had none, so pressing
// ? there showed only the global keys.
func TestHelpCoversEveryTab(t *testing.T) {
	for i, tab := range tabs {
		m := NewMainModel(nil, "")
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		m.SetTab(i)
		m.showHelp = true

		help := stripANSI(m.helpView())
		if !strings.Contains(help, "Keyboard Shortcuts") {
			t.Fatalf("tab %q rendered no help at all", tab.Label)
		}
		// helpView leaves the view name empty when its switch has no case for
		// the state, which is exactly the gap this guards against.
		name := viewNameRE.FindStringSubmatch(help)
		if name == nil || strings.TrimSpace(name[1]) == "" {
			t.Errorf("tab %q (%v) has no help section of its own", tab.Label, tab.State)
		}
	}
}

// A restarted TUI must come back on the tab the stream was launched from,
// which is what ui.go does via ActiveTab/SetTab around a stream handoff.
func TestTabSurvivesRestart(t *testing.T) {
	streams := -1
	for i, tab := range tabs {
		if tab.State == ViewStreams {
			streams = i
		}
	}
	if streams < 0 {
		t.Fatal("no Streams tab defined")
	}

	first := NewMainModel(nil, "")
	first.SetTab(streams)

	second := NewMainModel(nil, "")
	second.SetTab(first.ActiveTab())

	if second.viewState != ViewStreams {
		t.Errorf("restarted model shows %v, want ViewStreams", second.viewState)
	}
}

// The video stream is spawned as a child process. Naming it "c64u" looked the
// binary up on PATH, so it failed whenever c64u was not installed — running
// ./build/c64u, for instance.
func TestSelfPathIsAnExecutableFile(t *testing.T) {
	p := selfPath()
	if p == "" {
		t.Fatal("selfPath returned nothing")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("selfPath returned %q, want an absolute path", p)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("selfPath returned %q, which does not exist: %v", p, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("selfPath returned %q, which is not executable", p)
	}
}
