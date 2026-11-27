package output

import (
	"strings"
	"testing"
)

// Files on the C64 Ultimate filesystem often have no extension at all. Slicing
// from an unguarded strings.LastIndex panicked on those, which took down every
// directory listing that contained one.
func TestFileExtWithoutDot(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"README", ""},
		{"", ""},
		{"noextension", ""},
		{"game.prg", ".prg"},
		{"GAME.PRG", ".prg"},
		{"archive.tar.gz", ".gz"},
		{".hidden", ".hidden"},
		{"trailing.", "."},
	}

	for _, c := range cases {
		if got := fileExt(c.filename); got != c.want {
			t.Errorf("fileExt(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

// The three exported helpers must survive a name without an extension.
func TestFileHelpersSurviveMissingExtension(t *testing.T) {
	for _, name := range []string{"README", "", "noextension"} {
		GetFileIcon(name, false)
		GetFileTypeStyle(name, false)
		if label := GetFileTypeLabel(name, false); label == "" {
			t.Errorf("GetFileTypeLabel(%q) returned an empty label", name)
		}
	}
}

func TestFileHelpersKnownTypes(t *testing.T) {
	if got := GetFileTypeLabel("game.d64", false); got != "D64" {
		t.Errorf("GetFileTypeLabel(game.d64) = %q, want D64", got)
	}
	if got := GetFileTypeLabel("anything", true); got != "DIR" {
		t.Errorf("GetFileTypeLabel(dir) = %q, want DIR", got)
	}
}

// Nested response data used to print as Go syntax: "map[inputs:[] port:1]".
func TestFieldLines_NestedValues(t *testing.T) {
	data := map[string]interface{}{
		"joysticks": []interface{}{
			map[string]interface{}{"port": float64(1), "inputs": []interface{}{}},
		},
		"mouse": map[string]interface{}{"pending": float64(0), "attached": false},
	}
	var got []string
	for _, k := range []string{"joysticks", "mouse"} {
		got = append(got, fieldLines("  ", k, data[k])...)
	}
	want := []string{
		"  joysticks:",
		"    - inputs: none",
		"      port: 1",
		"  mouse:",
		"    attached: false",
		"    pending: 0",
	}
	assertLines(t, got, want)
}

// JSON numbers arrive as float64; %v printed 1583280 as 1.58328e+06.
func TestFieldLines_Scalars(t *testing.T) {
	cases := []struct {
		key   string
		value interface{}
		want  string
	}{
		{"free", float64(1583280), "  free: 1583280"},
		{"ratio", 0.5, "  ratio: 0.5"},
		{"written", []interface{}{"A", "B"}, "  written: A, B"},
		{"empty", map[string]interface{}{}, "  empty: none"},
		{"name", "x", "  name: x"},
	}
	for _, c := range cases {
		assertLines(t, fieldLines("  ", c.key, c.value), []string{c.want})
	}
}

// Map iteration order is random; the output must not be.
func TestSortedFields(t *testing.T) {
	m := map[string]interface{}{"b": "2", "a": "1", "c": "3"}
	for i := 0; i < 20; i++ {
		assertLines(t, mapLines("  ", m), []string{"  a: 1", "  b: 2", "  c: 3"})
	}
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestFieldLines_ListOfLists(t *testing.T) {
	v := []interface{}{[]interface{}{float64(1), float64(2)}, []interface{}{}}
	assertLines(t, fieldLines("  ", "rows", v), []string{"  rows:", "    - 1, 2", "    - none"})
}

func TestFieldLines_NullReadsNone(t *testing.T) {
	assertLines(t, fieldLines("  ", "image", nil), []string{"  image: none"})
}

// The colour split: a label and the value after it; a list item's text is a
// value, not a label.
func TestSplitLine(t *testing.T) {
	cases := []struct{ line, label, value string }{
		{"  free: 1583280", "  free:", "1583280"},
		{"  mouse:", "  mouse:", ""},
		{"    - inputs: none", "    - inputs:", "none"},
		{"    - 1, 2", "    -", "1, 2"},
	}
	for _, c := range cases {
		if l, v := splitLine(c.line); l != c.label || v != c.value {
			t.Errorf("splitLine(%q) = %q, %q; want %q, %q", c.line, l, v, c.label, c.value)
		}
	}
}
