package main

import (
	"reflect"
	"testing"
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
