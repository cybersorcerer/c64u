package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeRemoteSource is an in-memory stand-in for the C64 Ultimate so the browser's
// key routing can be tested without a device or an api.Client.
type fakeRemoteSource struct {
	items []fileItem
	files map[string][]byte
}

func (s *fakeRemoteSource) List(string) ([]fileItem, error)    { return s.items, nil }
func (s *fakeRemoteSource) Delete(string, bool) error          { return nil }
func (s *fakeRemoteSource) Rename(string, string) error        { return nil }
func (s *fakeRemoteSource) Mkdir(string) error                 { return nil }
func (s *fakeRemoteSource) ReadFile(p string) ([]byte, error)  { return s.files[p], nil }
func (s *fakeRemoteSource) WriteFile(p string, d []byte) error { s.files[p] = d; return nil }
func (s *fakeRemoteSource) IsLocal() bool                      { return false }

// newTestBrowser builds a browser whose local pane lists dir and whose remote
// pane lists names. The api.Client stays nil: these tests only inspect state
// transitions, they never execute the returned tea.Cmd.
func newTestBrowser(t *testing.T, dir string, remoteNames ...string) *BrowserModel {
	t.Helper()
	m := NewBrowserModel(nil)

	m.local = newPaneModel(&localSource{}, "LOCAL", dir)
	if err := m.local.reload(); err != nil {
		t.Fatalf("local reload: %v", err)
	}

	remote := &fakeRemoteSource{files: map[string][]byte{}}
	for _, n := range remoteNames {
		remote.items = append(remote.items, fileItem{Name: n})
	}
	m.remote = newPaneModel(remote, "REMOTE", "/")
	if err := m.remote.reload(); err != nil {
		t.Fatalf("remote reload: %v", err)
	}

	m.setActive(m.local)
	return m
}

// cursorTo moves the active pane's cursor onto the entry called name.
func cursorTo(t *testing.T, p *PaneModel, name string) {
	t.Helper()
	for i, it := range p.items {
		if it.Name == name {
			p.cursor = i
			return
		}
	}
	t.Fatalf("entry %q not found in pane", name)
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"/usb0/games.d64": "games.d64",
		"games.d64":       "games.d64",
		`C:\tmp\demo.prg`: "demo.prg",
		"/usb0/sub/":      "sub",
		"/":               "",
		"":                "",
	}
	for in, want := range cases {
		if got := baseName(in); got != want {
			t.Errorf("baseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUploadVariant(t *testing.T) {
	if got := uploadVariant("run-prg", true); got != "run-prg-upload" {
		t.Errorf("local variant = %q, want run-prg-upload", got)
	}
	if got := uploadVariant("run-prg", false); got != "run-prg" {
		t.Errorf("remote variant = %q, want run-prg", got)
	}
}

func TestIsDiskImageExt(t *testing.T) {
	for _, ext := range []string{".d64", ".d71", ".d81", ".g64", ".g71"} {
		if !isDiskImageExt(ext) {
			t.Errorf("%s should be a disk image", ext)
		}
	}
	for _, ext := range []string{".prg", ".crt", ".rom", ".txt", ""} {
		if isDiskImageExt(ext) {
			t.Errorf("%s should not be a disk image", ext)
		}
	}
}

func TestDiskActionItems_LocalLabelsMountAsUpload(t *testing.T) {
	remote := diskActionItems(false, false)
	local := diskActionItems(false, true)
	if len(remote) != len(local) {
		t.Fatalf("local and remote action lists should have the same length, got %d and %d", len(remote), len(local))
	}
	if remote[0].Description == local[0].Description {
		t.Error("local mount entry should describe the upload, remote should not")
	}
	if remote[0].Value != local[0].Value {
		t.Errorf("action values must match: %q vs %q", remote[0].Value, local[0].Value)
	}
}

func TestDiskActionItems_RunPRGOnlyWhenPresent(t *testing.T) {
	if diskActionItems(true, false)[0].Value != "run-prg" {
		t.Error("run-prg should be the first entry when the image contains a PRG")
	}
	for _, it := range diskActionItems(false, false) {
		if it.Value == "run-prg" {
			t.Error("run-prg must be absent when the image has no PRG")
		}
	}
}

func TestHandleEnter_LocalPRGOpensRunLoadSelector(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "demo.prg"), []byte("x"), 0o644) //nolint:errcheck

	m := newTestBrowser(t, dir)
	cursorTo(t, m.local, "demo.prg")
	m.handleEnter() //nolint:errcheck

	if m.state != browserPRGAction {
		t.Fatalf("state = %v, want browserPRGAction", m.state)
	}
	if !m.prgIsLocal {
		t.Error("prgIsLocal should be true for a PRG on the local pane")
	}
}

func TestHandleEnter_RemotePRGOpensRunLoadSelector(t *testing.T) {
	m := newTestBrowser(t, t.TempDir(), "demo.prg")
	m.setActive(m.remote)
	cursorTo(t, m.remote, "demo.prg")
	m.handleEnter() //nolint:errcheck

	if m.state != browserPRGAction {
		t.Fatalf("state = %v, want browserPRGAction", m.state)
	}
	if m.prgIsLocal {
		t.Error("prgIsLocal should be false for a PRG on the remote pane")
	}
}

func TestHandleUploadKey_RemoteROMUsesLoadROMByPath(t *testing.T) {
	m := newTestBrowser(t, t.TempDir(), "kernal.rom")
	m.setActive(m.remote)
	cursorTo(t, m.remote, "kernal.rom")
	m.handleKey(key("u")) //nolint:errcheck

	if m.state != browserUploadDrive {
		t.Fatalf("state = %v, want browserUploadDrive", m.state)
	}
	if !m.uploadIsROM || !m.uploadIsRemote {
		t.Errorf("uploadIsROM=%v uploadIsRemote=%v, want both true", m.uploadIsROM, m.uploadIsRemote)
	}
}

func TestHandleUploadKey_LocalDiskImageUploadsAndMounts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "games.d64"), []byte("x"), 0o644) //nolint:errcheck

	m := newTestBrowser(t, dir)
	cursorTo(t, m.local, "games.d64")
	m.handleKey(key("u")) //nolint:errcheck

	if m.state != browserUploadDrive {
		t.Fatalf("state = %v, want browserUploadDrive", m.state)
	}
	if m.uploadIsROM || m.uploadIsRemote {
		t.Errorf("uploadIsROM=%v uploadIsRemote=%v, want both false", m.uploadIsROM, m.uploadIsRemote)
	}
}

func TestHandleUploadKey_RemoteDiskImageRefersToEnter(t *testing.T) {
	m := newTestBrowser(t, t.TempDir(), "games.d64")
	m.setActive(m.remote)
	cursorTo(t, m.remote, "games.d64")
	m.handleKey(key("u")) //nolint:errcheck

	if m.state != browserBrowsing {
		t.Fatalf("state = %v, want browserBrowsing", m.state)
	}
	if m.message == "" {
		t.Error("expected a hint pointing at Enter for remote disk images")
	}
}

func TestHandleUploadKey_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644) //nolint:errcheck

	m := newTestBrowser(t, dir)
	cursorTo(t, m.local, "notes.txt")
	m.handleKey(key("u")) //nolint:errcheck

	if m.state != browserBrowsing {
		t.Fatalf("state = %v, want browserBrowsing", m.state)
	}
	if m.message == "" {
		t.Error("expected a message explaining which extensions are supported")
	}
}
