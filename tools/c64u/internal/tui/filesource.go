package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// fileItem is a source-agnostic directory entry.
type fileItem struct {
	Name  string
	IsDir bool
	Size  int64
}

// fileSource abstracts a filesystem (local disk or remote C64 Ultimate).
type fileSource interface {
	List(path string) ([]fileItem, error)
	Delete(path string, isDir bool) error
	Rename(oldPath, newPath string) error
	Mkdir(path string) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	IsLocal() bool
}

// sortItems sorts directories first, then alphabetically (case-insensitive).
func sortItems(items []fileItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

// ─── localSource ─────────────────────────────────────────────────────────

type localSource struct{}

func (s *localSource) List(path string) ([]fileItem, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var items []fileItem
	for _, e := range entries {
		info, err := e.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		items = append(items, fileItem{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	sortItems(items)
	return items, nil
}

func (s *localSource) Delete(path string, isDir bool) error {
	if isDir {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func (s *localSource) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (s *localSource) Mkdir(path string) error {
	return os.Mkdir(path, 0o755)
}

func (s *localSource) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (s *localSource) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func (s *localSource) IsLocal() bool { return true }

// ─── remoteSource ────────────────────────────────────────────────────────

type remoteSource struct {
	client *api.Client
}

func (s *remoteSource) List(path string) ([]fileItem, error) {
	entries, err := s.client.FTPList(path)
	if err != nil {
		return nil, err
	}
	var items []fileItem
	for _, e := range entries {
		items = append(items, fileItem{
			Name:  e.Name,
			IsDir: e.IsDir,
			Size:  int64(e.Size),
		})
	}
	sortItems(items)
	return items, nil
}

func (s *remoteSource) Delete(path string, isDir bool) error {
	if isDir {
		return s.client.FTPDeleteDir(path)
	}
	return s.client.FTPDelete(path)
}

func (s *remoteSource) Rename(oldPath, newPath string) error {
	return s.client.FTPRename(oldPath, newPath)
}

func (s *remoteSource) Mkdir(path string) error {
	return s.client.FTPMkdir(path)
}

func (s *remoteSource) ReadFile(path string) ([]byte, error) {
	return s.client.FTPReadFile(path)
}

func (s *remoteSource) WriteFile(path string, data []byte) error {
	return s.client.FTPUploadBytes(path, data)
}

func (s *remoteSource) IsLocal() bool { return false }

// joinPath joins a directory and name using forward slashes (remote/POSIX paths).
func joinPath(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return strings.TrimRight(dir, "/") + "/" + name
}

// localJoin joins using the OS path separator for the local pane.
func localJoin(dir, name string) string {
	return filepath.Join(dir, name)
}
