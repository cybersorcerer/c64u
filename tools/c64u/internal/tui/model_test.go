package tui

import "testing"

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
