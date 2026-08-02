// Package softiec drives the C64 Ultimate's SoftIEC (DOS emulation) drive.
//
// SoftIEC appears in the drives list but is not a floppy drive, and the drives
// endpoint does not control it: PUT /v1/drives/softiec:on answers success while
// the drive stays disabled. Enabling it and its bus ID live in the device
// configuration instead.
//
// The directory SoftIEC serves has no REST endpoint at all. It is changed with
// a CBM DOS "CD" command over the IEC bus, which this package types on the C64
// keyboard and then verifies by reading the drive's active partition back.
package softiec

import (
	"fmt"
	"strings"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// Device is the part of the C64 Ultimate API this package needs.
type Device interface {
	GetConfigCategories() ([]string, error)
	GetConfigCategory(category string) (map[string]interface{}, error)
	GetConfigItem(category, item string) (map[string]interface{}, error)
	SetConfigItem(category, item, value string) error
	DrivesList() (*api.Response, error)
	SendKeys(text string, delay time.Duration) error
}

// Settings locates the SoftIEC options and holds their current values. The
// names differ between firmware versions — the device reports "SoftIEC Drive
// Settings" while the API documentation calls it "Software IEC Settings" — so
// they are discovered rather than hard-coded.
type Settings struct {
	Category   string
	EnableItem string
	BusIDItem  string
	PathItem   string

	Enabled     bool
	BusID       int
	DefaultPath string

	MinBusID int
	MaxBusID int
}

// Status is the drive's live state, as reported by the drives list.
type Status struct {
	Present   bool
	Enabled   bool
	BusID     int
	Path      string // directory currently served
	LastError string // CBM DOS status of the last command
}

const (
	enabledValue  = "Enabled"
	disabledValue = "Disabled"
)

// LoadSettings discovers the SoftIEC configuration and reads it.
func LoadSettings(d Device) (*Settings, error) {
	category, err := findCategory(d)
	if err != nil {
		return nil, err
	}

	raw, err := d.GetConfigCategory(category)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", category, err)
	}
	items, ok := raw[category].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("category %q returned no settings", category)
	}

	s := &Settings{Category: category, MinBusID: 8, MaxBusID: 30}
	classifyItems(items, s)

	if s.EnableItem == "" {
		return nil, fmt.Errorf("category %q has no enable setting", category)
	}
	if s.BusIDItem != "" {
		if lo, hi, ok := busIDRange(d, category, s.BusIDItem); ok {
			s.MinBusID, s.MaxBusID = lo, hi
		}
	}
	return s, nil
}

// findCategory returns the configuration category holding the SoftIEC settings.
func findCategory(d Device) (string, error) {
	categories, err := d.GetConfigCategories()
	if err != nil {
		return "", fmt.Errorf("read configuration categories: %w", err)
	}
	for _, c := range categories {
		// "SoftIEC Drive Settings" and "Software IEC Settings" both match;
		// no other category on the device mentions IEC.
		if strings.Contains(strings.ToLower(c), "iec") {
			return c, nil
		}
	}
	return "", fmt.Errorf("no SoftIEC configuration category on this device")
}

// classifyItems assigns the settings by the shape of their values, so renamed
// items in another firmware are still found: the enable setting is the one
// holding Enabled or Disabled, the bus ID is the number, the default path is
// the absolute path.
func classifyItems(items map[string]interface{}, s *Settings) {
	for name, value := range items {
		switch v := value.(type) {
		case string:
			switch {
			case strings.EqualFold(v, enabledValue), strings.EqualFold(v, disabledValue):
				s.EnableItem = name
				s.Enabled = strings.EqualFold(v, enabledValue)
			case strings.HasPrefix(v, "/"):
				s.PathItem = name
				s.DefaultPath = v
			}
		case float64:
			s.BusIDItem = name
			s.BusID = int(v)
		}
	}
}

func busIDRange(d Device, category, item string) (lo, hi int, ok bool) {
	raw, err := d.GetConfigItem(category, item)
	if err != nil {
		return 0, 0, false
	}
	cat, ok := raw[category].(map[string]interface{})
	if !ok {
		return 0, 0, false
	}
	detail, ok := cat[item].(map[string]interface{})
	if !ok {
		return 0, 0, false
	}
	minV, minOK := detail["min"].(float64)
	maxV, maxOK := detail["max"].(float64)
	if !minOK || !maxOK {
		return 0, 0, false
	}
	return int(minV), int(maxV), true
}

// SetEnabled turns SoftIEC on or off.
func SetEnabled(d Device, s *Settings, on bool) error {
	value := disabledValue
	if on {
		value = enabledValue
	}
	if err := d.SetConfigItem(s.Category, s.EnableItem, value); err != nil {
		return err
	}
	s.Enabled = on
	return nil
}

// SetBusID changes the IEC device number SoftIEC answers on.
func SetBusID(d Device, s *Settings, id int) error {
	if s.BusIDItem == "" {
		return fmt.Errorf("this device has no configurable SoftIEC bus ID")
	}
	if id < s.MinBusID || id > s.MaxBusID {
		return fmt.Errorf("bus ID %d out of range (%d-%d)", id, s.MinBusID, s.MaxBusID)
	}
	if err := d.SetConfigItem(s.Category, s.BusIDItem, fmt.Sprintf("%d", id)); err != nil {
		return err
	}
	s.BusID = id
	return nil
}

// ReadStatus reports the drive's live state from the drives list.
func ReadStatus(d Device) (Status, error) {
	resp, err := d.DrivesList()
	if err != nil {
		return Status{}, err
	}
	if resp.HasErrors() {
		return Status{}, fmt.Errorf("%s", strings.Join(resp.Errors, "; "))
	}
	drives, _ := resp.Data["drives"].([]interface{})
	for _, entry := range drives {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		for name, info := range m {
			if !strings.Contains(strings.ToLower(name), "iec") {
				continue
			}
			fields, ok := info.(map[string]interface{})
			if !ok {
				continue
			}
			st := Status{Present: true}
			st.Enabled, _ = fields["enabled"].(bool)
			st.LastError, _ = fields["last_error"].(string)
			if id, ok := fields["bus_id"].(float64); ok {
				st.BusID = int(id)
			}
			st.Path = firstPartitionPath(fields)
			return st, nil
		}
	}
	return Status{}, nil
}

func firstPartitionPath(fields map[string]interface{}) string {
	parts, _ := fields["partitions"].([]interface{})
	if len(parts) == 0 {
		return ""
	}
	p, ok := parts[0].(map[string]interface{})
	if !ok {
		return ""
	}
	path, _ := p["path"].(string)
	return path
}

// CDCommand builds the BASIC line that points SoftIEC at path. An absolute
// path needs the "CD:" form; "CD//path/" is rejected by the drive.
func CDCommand(busID int, path string) string {
	return fmt.Sprintf(`OPEN1,%d,15,"CD:%s":CLOSE1`+"\n", busID, path)
}

// SamePath compares two device paths. The drive reports the name as it exists
// on disk, while the C64 types in PETSCII uppercase, and a trailing slash is
// optional.
func SamePath(a, b string) bool {
	norm := func(s string) string {
		return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "/"))
	}
	return norm(a) == norm(b)
}

// ValidatePath rejects paths the CD command cannot carry.
func ValidatePath(path string) error {
	switch {
	case strings.TrimSpace(path) == "":
		return fmt.Errorf("path is empty")
	case !strings.HasPrefix(path, "/"):
		return fmt.Errorf("path must be absolute, starting with /")
	case strings.ContainsAny(path, `"`):
		return fmt.Errorf(`path must not contain a quote`)
	}
	return nil
}

// SetRoot points SoftIEC at path by typing a CD command on the C64, then reads
// the drive back to confirm it took effect. The C64 has to be at the BASIC
// prompt; if it is not, the command goes nowhere and this reports that.
func SetRoot(d Device, busID int, path string) (Status, error) {
	if err := ValidatePath(path); err != nil {
		return Status{}, err
	}
	if err := d.SendKeys(CDCommand(busID, path), 40*time.Millisecond); err != nil {
		return Status{}, fmt.Errorf("send CD command: %w", err)
	}
	return awaitPath(d, path, pollAttempts, pollInterval)
}

// How long SetRoot waits for the C64 to work through the typed command.
var (
	pollAttempts = 8
	pollInterval = 400 * time.Millisecond
)

// awaitPath polls the drive until it serves path, or gives up.
func awaitPath(d Device, path string, attempts int, interval time.Duration) (Status, error) {
	var last Status
	for i := range attempts {
		if i > 0 {
			time.Sleep(interval)
		}
		st, err := ReadStatus(d)
		if err != nil {
			return st, err
		}
		last = st
		if SamePath(st.Path, path) {
			return st, nil
		}
	}
	if last.LastError != "" && !strings.HasPrefix(last.LastError, "00") {
		return last, fmt.Errorf("drive reported %s", last.LastError)
	}
	return last, fmt.Errorf("drive still serves %s — is the C64 at the BASIC prompt?", last.Path)
}
