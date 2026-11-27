package softiec

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// fakeFiles stands in for the device's FTP server. Like the real one, listing
// a path that does not exist returns an empty list rather than an error.
type fakeFiles struct {
	dirs     map[string][]api.FileEntry // directory -> entries
	files    map[string][]byte
	uploaded map[string][]byte
	deleted  []string
}

func newFakeFiles() *fakeFiles {
	return &fakeFiles{
		dirs: map[string][]api.FileEntry{
			"/":             {{Name: "Flash", IsDir: true}, {Name: "Temp", IsDir: true}, {Name: "USB0", IsDir: true}},
			"/USB0":         {{Name: "games", IsDir: true}, {Name: "notes.txt"}},
			"/Flash/config": {},
		},
		files:    map[string][]byte{},
		uploaded: map[string][]byte{},
	}
}

func (f *fakeFiles) FTPList(p string) ([]api.FileEntry, error) {
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return f.dirs[p], nil
}

func (f *fakeFiles) FTPReadFile(p string) ([]byte, error) {
	data, ok := f.files[p]
	if !ok {
		return nil, errors.New("FTP read failed: 550")
	}
	return data, nil
}

func (f *fakeFiles) FTPUploadBytes(p string, data []byte) error {
	f.uploaded[p] = data
	return nil
}

func (f *fakeFiles) FTPDelete(p string) error {
	f.deleted = append(f.deleted, p)
	return nil
}

// withSaved puts a partition file on the fake device.
func (f *fakeFiles) withSaved(data string) *fakeFiles {
	f.files[PartitionFile] = []byte(data)
	dir := path.Dir(PartitionFile)
	f.dirs[dir] = append(f.dirs[dir], api.FileEntry{Name: path.Base(PartitionFile)})
	return f
}

func TestParsePartition(t *testing.T) {
	cases := []struct {
		arg  string
		want Partition
	}{
		{"2=USB:/USB0/", Partition{Number: 2, Name: "USB", Path: "/USB0/"}},
		{"3=/USB0/games", Partition{Number: 3, Path: "/USB0/games"}},
		{"255=/x", Partition{Number: 255, Path: "/x"}},
	}
	for _, c := range cases {
		got, err := ParsePartition(c.arg)
		if err != nil || got != c.want {
			t.Errorf("ParsePartition(%q) = %+v, %v; want %+v", c.arg, got, err, c.want)
		}
	}
}

func TestParsePartitionRejects(t *testing.T) {
	for _, arg := range []string{
		"", "2", "=/USB0/", "x=/USB0/", "0=/USB0/", "256=/USB0/",
		"2=USB0/", "2=USB:", "2=:/USB0/", "2=SEVENTEEN-CHARSXX:/USB0/", `2=A"B:/USB0/`, `2=/US"B0/`, "2=A:B:/x",
	} {
		if _, err := ParsePartition(arg); err == nil {
			t.Errorf("ParsePartition(%q) accepted it", arg)
		}
	}
}

func TestEncodePartitionsSortsByNumber(t *testing.T) {
	data, err := EncodePartitions([]Partition{
		{Number: 3, Path: "/USB0/games"},
		{Number: 2, Name: "USB", Path: "/USB0/"},
	})
	if err != nil {
		t.Fatalf("EncodePartitions: %v", err)
	}
	want := `{"version":1,"partitions":[{"number":2,"path":"/USB0/","name":"USB"},{"number":3,"path":"/USB0/games"}]}`
	if string(data) != want {
		t.Errorf("EncodePartitions = %s\nwant %s", data, want)
	}
}

func TestEncodePartitionsRejectsDuplicates(t *testing.T) {
	_, err := EncodePartitions([]Partition{{Number: 2, Path: "/a"}, {Number: 2, Path: "/b"}})
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Errorf("err = %v, want a duplicate error naming 2", err)
	}
	if _, err := EncodePartitions(nil); err == nil {
		t.Error("an empty list was accepted; clear is the way to remove the file")
	}
}

func TestSavePartitionsWritesTheFile(t *testing.T) {
	f := newFakeFiles()
	err := SavePartitions(f, []Partition{{Number: 2, Name: "USB", Path: "/usb0/"}, {Number: 3, Path: "/USB0/GAMES"}})
	if err != nil {
		t.Fatalf("SavePartitions: %v", err)
	}
	got := string(f.uploaded[PartitionFile])
	if !strings.Contains(got, `"path":"/usb0/"`) || !strings.Contains(got, `"number":3`) {
		t.Errorf("uploaded %q", got)
	}
}

func TestSavePartitionsRejectsMissingDirectory(t *testing.T) {
	for _, p := range []string{"/USB0/nope", "/USB0/notes.txt", "/SD/games"} {
		f := newFakeFiles()
		err := SavePartitions(f, []Partition{{Number: 2, Path: p}})
		if err == nil || !strings.Contains(err.Error(), p) {
			t.Errorf("SavePartitions(%s) err = %v, want one naming the path", p, err)
		}
		if len(f.uploaded) != 0 {
			t.Errorf("SavePartitions(%s) wrote the file anyway", p)
		}
	}
}

func TestSavedPartitions(t *testing.T) {
	parts, ok, err := SavedPartitions(newFakeFiles())
	if err != nil || ok || parts != nil {
		t.Errorf("without a file = %v, %v, %v; want nil, false, nil", parts, ok, err)
	}
	f := newFakeFiles().withSaved(`{"version":1,"partitions":[{"number":2,"path":"/USB0/","name":"USB"}]}`)
	parts, ok, err = SavedPartitions(f)
	if err != nil || !ok || len(parts) != 1 || parts[0] != (Partition{Number: 2, Name: "USB", Path: "/USB0/"}) {
		t.Errorf("with a file = %+v, %v, %v", parts, ok, err)
	}
	f = newFakeFiles().withSaved(`not json`)
	if _, _, err := SavedPartitions(f); err == nil {
		t.Error("a broken file was read without an error")
	}
}

func TestClearPartitions(t *testing.T) {
	f := newFakeFiles()
	existed, err := ClearPartitions(f)
	if err != nil || existed || len(f.deleted) != 0 {
		t.Errorf("without a file = %v, %v, deleted %v", existed, err, f.deleted)
	}
	f = newFakeFiles().withSaved(`{}`)
	existed, err = ClearPartitions(f)
	if err != nil || !existed || len(f.deleted) != 1 || f.deleted[0] != PartitionFile {
		t.Errorf("with a file = %v, %v, deleted %v", existed, err, f.deleted)
	}
}

func TestActivePartitions(t *testing.T) {
	d := newFakeDevice()
	parts, err := ActivePartitions(d)
	if err != nil || len(parts) != 1 || parts[0] != (Partition{Number: 0, Path: "/USB0/"}) {
		t.Errorf("ActivePartitions = %+v, %v", parts, err)
	}
}
