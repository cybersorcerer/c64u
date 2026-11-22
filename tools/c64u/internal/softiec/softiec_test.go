package softiec

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// fakeDevice stands in for a C64 Ultimate. It mirrors what the real device
// returned when this package was written, so the shape of the data is the
// device's own rather than an invention.
type fakeDevice struct {
	categories []string
	items      map[string]interface{}
	busIDMin   float64
	busIDMax   float64
	// enableValues is what the device offers for the enable setting; empty
	// stands for firmware that does not report it.
	enableValues []interface{}

	path      string
	lastError string
	enabled   bool

	sent []string
	// pathAfterKeys is applied on the next status read after keys are sent,
	// standing in for the C64 executing the typed CD command.
	pathAfterKeys string
	setErr        error
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		categories: []string{"Drive A Settings", "SoftIEC Drive Settings", "Printer Settings"},
		items: map[string]interface{}{
			"Default Path":      "/USB0/",
			"IEC Drive":         "Enabled",
			"Soft Drive Bus ID": float64(11),
		},
		busIDMin:  8,
		busIDMax:  30,
		path:      "/USB0/",
		lastError: "00, OK,00,00",
		enabled:   true,
	}
}

func (f *fakeDevice) GetConfigCategories() ([]string, error) { return f.categories, nil }

func (f *fakeDevice) GetConfigCategory(category string) (map[string]interface{}, error) {
	return map[string]interface{}{category: f.items}, nil
}

func (f *fakeDevice) GetConfigItem(category, item string) (map[string]interface{}, error) {
	detail := map[string]interface{}{"min": f.busIDMin, "max": f.busIDMax}
	if _, isNumber := f.items[item].(float64); !isNumber {
		detail = map[string]interface{}{}
		if len(f.enableValues) > 0 {
			detail["values"] = f.enableValues
		}
	}
	return map[string]interface{}{category: map[string]interface{}{item: detail}}, nil
}

// newFirmware121Device mirrors firmware 1.2.1, where the enable setting has a
// third value and the default path setting is gone.
func newFirmware121Device(current string) *fakeDevice {
	d := newFakeDevice()
	d.items = map[string]interface{}{
		"IEC Drive":           current,
		"Soft Drive Bus ID":   float64(11),
		"Log Every Operation": "Disabled",
	}
	d.enableValues = []interface{}{"UCI Only", "Enabled", "Disabled"}
	return d
}

// Firmware 1.2.1 added a second Enabled/Disabled setting to the category. Map
// order is random, so a wrong pick shows up only on some runs; repeat.
func TestLoadSettingsIgnoresOtherSwitches(t *testing.T) {
	for range 50 {
		s, err := LoadSettings(newFirmware121Device("Enabled"))
		if err != nil {
			t.Fatalf("LoadSettings: %v", err)
		}
		if s.EnableItem != "IEC Drive" {
			t.Fatalf("enable item = %q, want IEC Drive", s.EnableItem)
		}
	}
}

func (f *fakeDevice) SetConfigItem(category, item, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.items[item] = value
	return nil
}

func (f *fakeDevice) DrivesList() (*api.Response, error) {
	return &api.Response{Data: map[string]interface{}{
		"drives": []interface{}{
			map[string]interface{}{"a": map[string]interface{}{"enabled": true, "bus_id": float64(9)}},
			map[string]interface{}{"IEC Drive": map[string]interface{}{
				"enabled":    f.enabled,
				"bus_id":     float64(11),
				"last_error": f.lastError,
				"partitions": []interface{}{map[string]interface{}{"id": float64(0), "path": f.path}},
			}},
		},
	}}, nil
}

func (f *fakeDevice) SendKeys(text string, delay time.Duration) error {
	f.sent = append(f.sent, text)
	if f.pathAfterKeys != "" {
		f.path = f.pathAfterKeys
	}
	return nil
}

// The polling schedule is sized for a real C64 working through a BASIC line;
// the fake device answers instantly.
func TestMain(m *testing.M) {
	pollAttempts, pollInterval = 3, time.Millisecond
	os.Exit(m.Run())
}

func TestLoadSettingsDiscoversItemsByShape(t *testing.T) {
	d := newFakeDevice()
	s, err := LoadSettings(d)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Category != "SoftIEC Drive Settings" {
		t.Errorf("category = %q", s.Category)
	}
	if s.EnableItem != "IEC Drive" || !s.Enabled {
		t.Errorf("enable item = %q enabled=%v", s.EnableItem, s.Enabled)
	}
	if s.BusIDItem != "Soft Drive Bus ID" || s.BusID != 11 {
		t.Errorf("bus id item = %q value=%d", s.BusIDItem, s.BusID)
	}
	if s.PathItem != "Default Path" || s.DefaultPath != "/USB0/" {
		t.Errorf("path item = %q value=%q", s.PathItem, s.DefaultPath)
	}
	if s.MinBusID != 8 || s.MaxBusID != 30 {
		t.Errorf("bus id range = %d-%d, want 8-30", s.MinBusID, s.MaxBusID)
	}
}

// The API documentation names the category differently from the device.
func TestLoadSettingsAcceptsOtherFirmwareNames(t *testing.T) {
	d := newFakeDevice()
	d.categories = []string{"Drive A Settings", "Software IEC Settings"}
	d.items = map[string]interface{}{
		"Path":            "/Usb0/",
		"Software IEC":    "Disabled",
		"IEC Drive Bus a": float64(9),
	}
	s, err := LoadSettings(d)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Category != "Software IEC Settings" {
		t.Errorf("category = %q", s.Category)
	}
	if s.EnableItem != "Software IEC" || s.Enabled {
		t.Errorf("enable item = %q enabled=%v", s.EnableItem, s.Enabled)
	}
	if s.PathItem != "Path" || s.BusIDItem != "IEC Drive Bus a" {
		t.Errorf("path=%q busID=%q", s.PathItem, s.BusIDItem)
	}
}

func TestLoadSettingsWithoutIECCategory(t *testing.T) {
	d := newFakeDevice()
	d.categories = []string{"Drive A Settings", "Printer Settings"}
	if _, err := LoadSettings(d); err == nil {
		t.Error("expected an error when the device has no SoftIEC category")
	}
}

func TestSetBusIDRejectsOutOfRange(t *testing.T) {
	d := newFakeDevice()
	s, _ := LoadSettings(d)

	for _, id := range []int{7, 31} {
		if err := SetBusID(d, s, id); err == nil {
			t.Errorf("bus ID %d was accepted", id)
		}
	}
	if err := SetBusID(d, s, 12); err != nil {
		t.Errorf("bus ID 12 rejected: %v", err)
	}
	if s.BusID != 12 {
		t.Errorf("bus ID not applied, still %d", s.BusID)
	}
}

func TestSetEnabledWritesTheConfigItem(t *testing.T) {
	d := newFakeDevice()
	s, _ := LoadSettings(d)

	if err := SetEnabled(d, s, false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if got := d.items["IEC Drive"]; got != disabledValue {
		t.Errorf("config item is %v, want %s", got, disabledValue)
	}
	if s.Enabled {
		t.Error("settings still report enabled")
	}
}

// Firmware 1.2.1 ships with "UCI Only" as the default, so a device that was
// never switched on reports it.
func TestLoadSettingsRecognisesUCIOnly(t *testing.T) {
	d := newFirmware121Device("UCI Only")
	s, err := LoadSettings(d)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.EnableItem != "IEC Drive" || s.Enabled {
		t.Errorf("enable item = %q enabled=%v", s.EnableItem, s.Enabled)
	}
	if s.PathItem != "" {
		t.Errorf("path item = %q, firmware 1.2.1 has none", s.PathItem)
	}
}

// Taking the drive off the bus keeps it usable through the Command Interface,
// which is what Disabled meant before firmware 1.2.1.
func TestSetEnabledOffKeepsCommandInterface(t *testing.T) {
	d := newFirmware121Device("Enabled")
	s, _ := LoadSettings(d)

	if err := SetEnabled(d, s, false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if got := d.items["IEC Drive"]; got != "UCI Only" {
		t.Errorf("config item is %v, want UCI Only", got)
	}
}

func TestDisableCompletely(t *testing.T) {
	for name, d := range map[string]*fakeDevice{
		"firmware 1.2.1": newFirmware121Device("Enabled"),
		"older firmware": newFakeDevice(),
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := LoadSettings(d)
			if err := DisableCompletely(d, s); err != nil {
				t.Fatalf("DisableCompletely: %v", err)
			}
			if got := d.items["IEC Drive"]; got != disabledValue {
				t.Errorf("config item is %v, want %s", got, disabledValue)
			}
			if s.Enabled {
				t.Error("settings still report enabled")
			}
		})
	}
}

func TestCDCommand(t *testing.T) {
	got := CDCommand(11, "/USB0/development/")
	want := `OPEN1,11,15,"CD:/USB0/development/":CLOSE1` + "\n"
	if got != want {
		t.Errorf("CDCommand = %q, want %q", got, want)
	}
}

func TestValidatePath(t *testing.T) {
	for _, bad := range []string{"", "   ", "USB0/games", `/USB0/"quoted"/`} {
		if err := ValidatePath(bad); err == nil {
			t.Errorf("ValidatePath(%q) accepted it", bad)
		}
	}
	if err := ValidatePath("/USB0/development/"); err != nil {
		t.Errorf("ValidatePath rejected a good path: %v", err)
	}
}

func TestSamePath(t *testing.T) {
	equal := [][2]string{
		{"/USB0/development/", "/USB0/development"},
		{"/USB0/DEVELOPMENT/", "/usb0/development/"},
		{"/USB0/", "/USB0/"},
	}
	for _, p := range equal {
		if !SamePath(p[0], p[1]) {
			t.Errorf("SamePath(%q, %q) = false", p[0], p[1])
		}
	}
	if SamePath("/USB0/development/", "/USB0/temp/") {
		t.Error("different directories compared equal")
	}
}

func TestReadStatus(t *testing.T) {
	d := newFakeDevice()
	st, err := ReadStatus(d)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if !st.Present || !st.Enabled || st.BusID != 11 || st.Path != "/USB0/" {
		t.Errorf("status = %+v", st)
	}
}

func TestSetRootConfirmsTheChange(t *testing.T) {
	d := newFakeDevice()
	d.pathAfterKeys = "/USB0/development/"

	st, err := SetRoot(d, 11, "/USB0/development")
	if err != nil {
		t.Fatalf("SetRoot: %v", err)
	}
	if st.Path != "/USB0/development/" {
		t.Errorf("status path = %q", st.Path)
	}
	if len(d.sent) != 1 || !strings.Contains(d.sent[0], `"CD:/USB0/development"`) {
		t.Errorf("typed %q", d.sent)
	}
}

// If the C64 is not at the BASIC prompt the line goes nowhere; reporting
// success then would be worse than reporting nothing.
func TestSetRootFailsWhenNothingHappens(t *testing.T) {
	d := newFakeDevice()
	d.pathAfterKeys = "" // C64 ignores the keystrokes

	_, err := SetRoot(d, 11, "/USB0/development")
	if err == nil {
		t.Fatal("SetRoot reported success although the path did not change")
	}
	if !strings.Contains(err.Error(), "/USB0/") {
		t.Errorf("error should name the path still served, got %v", err)
	}
}

func TestSetRootReportsDriveError(t *testing.T) {
	d := newFakeDevice()
	d.lastError = "71,DIRECTORY ERROR,00,00"

	_, err := SetRoot(d, 11, "/USB0/nowhere")
	if err == nil || !strings.Contains(err.Error(), "DIRECTORY ERROR") {
		t.Errorf("error = %v, want the drive's own message", err)
	}
}
