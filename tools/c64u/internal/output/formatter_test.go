package output

import "testing"

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
