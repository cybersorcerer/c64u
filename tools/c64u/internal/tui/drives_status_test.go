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

func actionValues(m *DrivesModel) []string {
	var values []string
	for _, it := range m.actionSelector.Items {
		values = append(values, it.Value)
	}
	return values
}

// SoftIEC and the printer are not floppies: mounting, ROMs and drive modes do
// not apply to either of them.
func TestConfigDrivesOfferNoFloppyActions(t *testing.T) {
	m := NewDrivesModel(nil)
	floppyOnly := map[string]bool{
		"unmount": true, "loadrom": true, "reset": true,
		"mode_1541": true, "mode_1571": true, "mode_1581": true,
	}

	for letter := range configDrives {
		for _, enabled := range []bool{false, true} {
			m.openActionMenu(DriveItem{Name: letter, Letter: letter, Enabled: enabled})
			for _, v := range actionValues(m) {
				if floppyOnly[v] {
					t.Errorf("%q (enabled=%v) offers the floppy action %q", letter, enabled, v)
				}
			}
		}
	}

	// A real floppy keeps the full action list.
	m.openActionMenu(DriveItem{Name: "Drive A", Letter: "a", Enabled: true})
	if len(m.actionSelector.Items) < 2 {
		t.Errorf("drive A offers only %d actions", len(m.actionSelector.Items))
	}
}

func TestPrinterIsAPlainToggle(t *testing.T) {
	m := NewDrivesModel(nil)

	m.openActionMenu(DriveItem{Name: "Printer Emulation", Letter: printerDrive, Enabled: false})
	if got := actionValues(m); len(got) != 1 || got[0] != "on" {
		t.Errorf("disabled printer offers %v, want [on]", got)
	}

	m.openActionMenu(DriveItem{Name: "Printer Emulation", Letter: printerDrive, Enabled: true})
	if got := actionValues(m); len(got) != 1 || got[0] != "off" {
		t.Errorf("enabled printer offers %v, want [off]", got)
	}
}

// The device number has to be settable, and enabling SoftIEC asks for it.
func TestSoftIECActions(t *testing.T) {
	m := NewDrivesModel(nil)

	m.openActionMenu(DriveItem{Name: "IEC Drive", Letter: softIECDrive, Enabled: false})
	if got := actionValues(m); len(got) != 1 || got[0] != "softiec-enable" {
		t.Errorf("disabled SoftIEC offers %v, want [softiec-enable]", got)
	}

	m.openActionMenu(DriveItem{Name: "IEC Drive", Letter: softIECDrive, Enabled: true, BusID: 11})
	want := []string{"softiec-root", "softiec-busid", "off"}
	got := actionValues(m)
	if len(got) != len(want) {
		t.Fatalf("enabled SoftIEC offers %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("action %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// Both prompts have to prefill the current value, so confirming unchanged input
// is a no-op rather than a surprise.
func TestSoftIECPromptsPrefillCurrentValues(t *testing.T) {
	m := NewDrivesModel(nil)
	m.drives = []DriveItem{{Name: "IEC Drive", Letter: softIECDrive, Enabled: true, BusID: 12, Path: "/USB0/development/"}}

	if _, handled := m.startInput("softiec-root"); !handled {
		t.Fatal("softiec-root did not open a prompt")
	}
	if m.state != drivesRootInput || m.ti.Value() != "/USB0/development/" {
		t.Errorf("root prompt state=%v value=%q", m.state, m.ti.Value())
	}

	if _, handled := m.startInput("softiec-busid"); !handled {
		t.Fatal("softiec-busid did not open a prompt")
	}
	if m.state != drivesBusIDInput || m.ti.Value() != "12" {
		t.Errorf("bus ID prompt state=%v value=%q", m.state, m.ti.Value())
	}

	if _, handled := m.startInput("off"); handled {
		t.Error("a plain action should not open a prompt")
	}
}

// The drives list names these entries; the mapping to the identifier the API
// and the config toggles use has to hold for every one of them.
func TestDriveLetterMapping(t *testing.T) {
	cases := map[string]string{
		"a":                 "a",
		"b":                 "b",
		"Drive A":           "a",
		"IEC Drive":         softIECDrive,
		"Printer Emulation": printerDrive,
	}
	for name, want := range cases {
		if got := driveLetter(name); got != want {
			t.Errorf("driveLetter(%q) = %q, want %q", name, got, want)
		}
	}
}

// Every enabled drive shows the device number it answers on, so a LOAD from
// the C64 can address it without looking it up elsewhere.
func TestDriveStateText(t *testing.T) {
	cases := []struct {
		name  string
		drive DriveItem
		want  string
	}{
		{"empty floppy", DriveItem{Letter: "a", Mode: "1541", BusID: 9, Enabled: true}, "1541 • #9 • Empty"},
		{"floppy with disk", DriveItem{Letter: "b", Mode: "1541", BusID: 10, Enabled: true, Mounted: "games.d64"}, "1541 • #10 • games.d64"},
		{"softiec", DriveItem{Letter: softIECDrive, BusID: 11, Enabled: true, Path: "/USB0/development/"}, "#11 • /USB0/development/"},
		{"printer", DriveItem{Letter: printerDrive, BusID: 4, Enabled: true}, "#4"},
		{"disabled", DriveItem{Letter: softIECDrive, BusID: 11, Enabled: false, Path: "/USB0/"}, "Disabled"},
	}
	for _, c := range cases {
		if got := driveStateText(c.drive); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestConfigDriveRowShowsNoDiskState(t *testing.T) {
	m := NewDrivesModel(nil)
	m.width = 80
	m.loading = false
	m.drives = []DriveItem{{Name: "Printer Emulation", Letter: printerDrive, Enabled: true}}

	view := m.View()
	if strings.Contains(view, "Empty") {
		t.Errorf("printer row claims an empty disk slot:\n%s", view)
	}
	if !strings.Contains(view, "Enabled") {
		t.Errorf("printer row does not show its state:\n%s", view)
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
