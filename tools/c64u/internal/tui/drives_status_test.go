package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// Reproduces the reported sequence: enable a drive, the green status bar shows,
// then it auto-clears. The drives view must keep its own footer throughout.
func TestDrivesFooterSurvivesStatusClear(t *testing.T) {
	m := NewMainModel(nil, "")
	m.SetTab(1) // Drives
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m.Update(drivesFetchedMsg{
		{Name: "Drive A", Letter: "a", Mode: "1541", Enabled: true},
		{Name: "IEC Drive", Letter: "softiec", Enabled: false},
	})

	const hint = "Enter: actions"

	before := m.View()
	if !strings.Contains(before, hint) {
		t.Fatalf("footer missing before any status message:\n%s", before)
	}

	m.Update(statusMsg("Drive SOFTIEC enabled"))
	withStatus := m.View()
	if !strings.Contains(withStatus, "Drive SOFTIEC enabled") {
		t.Error("status message not rendered")
	}
	if !strings.Contains(withStatus, hint) {
		t.Errorf("footer missing while status is shown:\n%s", withStatus)
	}

	m.Update(ClearStatusMsg{ID: m.statusID})
	after := m.View()
	if strings.Contains(after, "Drive SOFTIEC enabled") {
		t.Error("status message still rendered after clear")
	}
	if !strings.Contains(after, hint) {
		t.Errorf("footer missing after status cleared:\n%s", after)
	}

	// A frame that shrinks leaves the previous frame's trailing rows on screen,
	// which is what made the footer disappear. Every frame must be full height.
	for name, frame := range map[string]string{"before": before, "withStatus": withStatus, "after": after} {
		if got := strings.Count(frame, "\n") + 1; got != 40 {
			t.Errorf("%s frame is %d lines, want 40", name, got)
		}
	}
}

func TestDriveActionResultReportsAPIErrors(t *testing.T) {
	if got := driveActionResult(&api.Response{Errors: []string{"no such drive"}}, nil, "Drive X enabled"); got != "Error: no such drive" {
		t.Errorf("API error reported as %q", got)
	}
	if got := driveActionResult(nil, errors.New("connection refused"), "Drive X enabled"); got != "Error: connection refused" {
		t.Errorf("transport error reported as %q", got)
	}
	if got := driveActionResult(&api.Response{}, nil, "Drive X enabled"); got != "Drive X enabled" {
		t.Errorf("success reported as %q", got)
	}
}
