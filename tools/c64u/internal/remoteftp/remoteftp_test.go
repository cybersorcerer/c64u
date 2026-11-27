package remoteftp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// fakeFiles stands in for the device's FTP server.
type fakeFiles struct {
	files    map[string][]byte
	uploaded map[string][]byte
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{files: map[string][]byte{}, uploaded: map[string][]byte{}}
}

func (f *fakeFiles) FTPList(p string) ([]api.FileEntry, error) {
	var entries []api.FileEntry
	if p == "/Flash/config" {
		for name := range f.files {
			entries = append(entries, api.FileEntry{Name: strings.TrimPrefix(name, p+"/")})
		}
	}
	return entries, nil
}

func (f *fakeFiles) FTPReadFile(p string) ([]byte, error) {
	if data, ok := f.files[p]; ok {
		return data, nil
	}
	return nil, errors.New("550")
}

func (f *fakeFiles) FTPUploadBytes(p string, data []byte) error {
	f.uploaded[p] = data
	return nil
}

// What the menu wrote on firmware 1.2.1 for a server added there.
const menuFile = `{
  "version" : 1,
  "servers" : [ {
  "alias" : "mac",
  "host" : "192.168.1.155",
  "port" : 2121,
  "user" : "anonymous",
  "password" : "",
  "folder" : "/"
} ]
}`

func TestEncodePassword(t *testing.T) {
	// The firmware's own url_encode keeps letters, digits and _-.* and writes
	// everything else as lowercase %xx; "pa ss&w=rd" logged in as written.
	if got := EncodePassword("pa ss&w=rd"); got != "pa%20ss%26w%3drd" {
		t.Errorf("EncodePassword = %q", got)
	}
	if got := EncodePassword("Ab9_-.*"); got != "Ab9_-.*" {
		t.Errorf("unreserved characters changed: %q", got)
	}
}

func TestNewServer(t *testing.T) {
	s, err := NewServer("nas", "192.168.1.20", 0, "", "pa ss", "")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	want := Server{Alias: "nas", Host: "192.168.1.20", Port: 21, User: "anonymous", Password: "pa%20ss", Folder: "/"}
	if s != want {
		t.Errorf("NewServer = %+v, want %+v", s, want)
	}
	for _, c := range []struct {
		alias, host, folder string
		port                int
	}{
		{"", "h", "/", 21}, {"a/b", "h", "/", 21}, {"nas", "", "/", 21},
		{"nas", "h", "games", 21}, {"nas", "h", "/", 70000}, {"nas", "h", "/", -1},
	} {
		if _, err := NewServer(c.alias, c.host, c.port, "", "", c.folder); err == nil {
			t.Errorf("NewServer(%q, %q, %d, %q) accepted it", c.alias, c.host, c.port, c.folder)
		}
	}
}

func TestLoadReadsTheMenuFile(t *testing.T) {
	f := newFakeFiles()
	if servers, err := Load(f); err != nil || servers != nil {
		t.Errorf("without a file = %v, %v", servers, err)
	}
	f.files[ServerFile] = []byte(menuFile)
	servers, err := Load(f)
	want := Server{Alias: "mac", Host: "192.168.1.155", Port: 2121, User: "anonymous", Folder: "/"}
	if err != nil || len(servers) != 1 || servers[0] != want {
		t.Errorf("Load = %+v, %v", servers, err)
	}
	f.files[ServerFile] = []byte("not json")
	if _, err := Load(f); err == nil {
		t.Error("a broken file was read without an error")
	}
}

func TestPutReplacesByAliasWithoutCase(t *testing.T) {
	list := []Server{{Alias: "mac", Host: "a"}, {Alias: "nas", Host: "b"}}
	list, replaced := Put(list, Server{Alias: "MAC", Host: "c"})
	if !replaced || len(list) != 2 || list[0].Host != "c" || list[0].Alias != "MAC" {
		t.Errorf("replace = %+v, %v", list, replaced)
	}
	list, replaced = Put(list, Server{Alias: "new", Host: "d"})
	if replaced || len(list) != 3 || list[2].Alias != "new" {
		t.Errorf("add = %+v, %v", list, replaced)
	}
}

func TestRemove(t *testing.T) {
	list := []Server{{Alias: "mac"}, {Alias: "nas"}}
	list, ok := Remove(list, "NAS")
	if !ok || len(list) != 1 || list[0].Alias != "mac" {
		t.Errorf("Remove = %+v, %v", list, ok)
	}
	if _, ok := Remove(list, "gone"); ok {
		t.Error("removed an alias that is not there")
	}
}

func TestSaveWritesTheFirmwareFormat(t *testing.T) {
	f := newFakeFiles()
	s, _ := NewServer("nas", "192.168.1.20", 2121, "c64", "pa ss&w=rd", "/games")
	if err := Save(f, []Server{s}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var got struct {
		Version int                      `json:"version"`
		Servers []map[string]interface{} `json:"servers"`
	}
	if err := json.Unmarshal(f.uploaded[ServerFile], &got); err != nil {
		t.Fatalf("written file: %v", err)
	}
	if got.Version != 1 || len(got.Servers) != 1 {
		t.Fatalf("written %s", f.uploaded[ServerFile])
	}
	e := got.Servers[0]
	if e["alias"] != "nas" || e["host"] != "192.168.1.20" || e["port"] != float64(2121) ||
		e["user"] != "c64" || e["password"] != "pa%20ss%26w%3drd" || e["folder"] != "/games" {
		t.Errorf("entry = %v", e)
	}
	// Entries already in the file came from the menu or the firmware; one that
	// would not pass NewServer must not stop another from being saved.
	if err := Save(f, []Server{{Alias: "menu", Host: "h", Port: 21, Folder: ""}, s}); err != nil {
		t.Errorf("an entry from the menu blocked saving: %v", err)
	}
	// An empty list is still a valid file: the firmware then mounts no server.
	if err := Save(f, nil); err != nil || !strings.Contains(string(f.uploaded[ServerFile]), `"servers":[]`) {
		t.Errorf("empty list: %v, %s", err, f.uploaded[ServerFile])
	}
}
