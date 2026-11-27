package main

import (
	"reflect"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// The config API nests category → item → attributes. Rendering a nested map
// with %v printed a raw Go map literal, so the value formatter is checked for
// every type the API actually returns.
func TestFormatConfigValue(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"string", "1541", "1541"},
		{"empty string", "", "─ (empty)"},
		{"integer as float64", float64(9), "9"},
		{"fractional float64", 1.5, "1.50"},
		{"true", true, "● true"},
		{"false", false, "○ false"},
		{"nil", nil, "─ (not set)"},
		{"empty list", []interface{}{}, "[]"},
		{"list", []interface{}{"a.rom", "b.rom"}, "a.rom, b.rom"},
	}

	for _, c := range cases {
		if got := formatConfigValue(c.value); got != c.want {
			t.Errorf("%s: formatConfigValue(%#v) = %q, want %q", c.name, c.value, got, c.want)
		}
	}
}

// Go randomises map iteration, so keys are sorted to keep output stable.
func TestSortedKeys(t *testing.T) {
	m := map[string]interface{}{"min": 8, "current": 9, "default": 8, "max": 11}
	want := []string{"current", "default", "max", "min"}
	if got := sortedKeys(m); !reflect.DeepEqual(got, want) {
		t.Errorf("sortedKeys() = %v, want %v", got, want)
	}

	if got := sortedKeys(map[string]interface{}{}); len(got) != 0 {
		t.Errorf("sortedKeys(empty) = %v, want empty", got)
	}
}

func TestCategoryStoreOutcome(t *testing.T) {
	cases := []struct {
		action string
		names  []string
		msg    string
		ok     bool
	}{
		{"save_to_flash", []string{"Drive A Settings"}, "Saved to flash", true},
		{"save_to_flash", []string{}, `Nothing to save: no unsaved changes match "Drive*"`, true},
		{"load_from_flash", []string{"Drive A Settings"}, "Reloaded from flash", true},
		{"load_from_flash", []string{}, `No category matches "Drive*"`, false},
		{"reset_to_default", []string{"Drive A Settings"}, "Reset to defaults", true},
		{"reset_to_default", []string{}, `No category matches "Drive*"`, false},
	}
	for _, c := range cases {
		msg, data, ok := categoryStoreOutcome(c.action, "Drive*", c.names)
		if msg != c.msg || ok != c.ok {
			t.Errorf("%s %v: got %q, %v; want %q, %v", c.action, c.names, msg, ok, c.msg, c.ok)
		}
		if ok && data["category"] != "Drive*" {
			t.Errorf("%s: data = %v", c.action, data)
		}
	}
}

func TestSplitIdentity(t *testing.T) {
	changes := []api.ConfigChange{
		{Category: "Drive A Settings", Item: "Drive Type", New: "1581"},
		{Category: "Ethernet Settings", Item: "Static IP", New: "192.168.2.64"},
		{Category: "Network Settings", Item: "Host Name", New: "C64U"},
		{Category: "Network Settings", Item: "TimeZone", New: "Western Europe"},
	}
	rest, identity := splitIdentity(changes)
	if len(rest) != 2 || rest[1].Item != "TimeZone" || len(identity) != 2 || identity[0].Item != "Static IP" {
		t.Errorf("rest = %+v, identity = %+v", rest, identity)
	}
}

// After a restore changed the network settings, the hint names where the
// device will be: its static address, unless the backup uses DHCP.
func TestStaticAddress(t *testing.T) {
	static := api.ConfigSettings{"Ethernet Settings": {"Use DHCP": "Disabled", "Static IP": "192.168.2.64"}}
	if got := staticAddress(static); got != "192.168.2.64" {
		t.Errorf("static = %q", got)
	}
	dhcp := api.ConfigSettings{"Ethernet Settings": {"Use DHCP": "Enabled", "Static IP": "192.168.2.64"}}
	if got := staticAddress(dhcp); got != "" {
		t.Errorf("dhcp = %q, want none", got)
	}
}

func TestChangeValue(t *testing.T) {
	cases := map[string]api.ConfigChange{
		"1541 -> 1581":                 {Category: "Drive A Settings", Item: "Drive Type", Old: "1541", New: "1581"},
		"(not on this device) -> 0 dB": {Category: "Audio Mixer", Item: "Vol Ultimate", New: "0 dB"},
		"8 -> 9":                       {Category: "Drive A Settings", Item: "Drive Bus ID", Old: float64(8), New: float64(9)},
	}
	for want, ch := range cases {
		if got := changeValue(ch); got != want {
			t.Errorf("changeValue = %q, want %q", got, want)
		}
	}
}
