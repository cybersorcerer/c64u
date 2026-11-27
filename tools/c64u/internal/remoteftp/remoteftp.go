// Package remoteftp manages the remote FTP servers a C64 Ultimate mounts under
// /ftp/<alias>/, so that drives mount and runners run-prg reach files on a NAS.
//
// Firmware 1.2.1 keeps the list in ServerFile and reads it only at power-on;
// servers added in the menu (Disk File Browser > FTP > New Host) work at once
// and rewrite the file. There is no REST endpoint for it (checked 2026-11-27).
package remoteftp

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// ServerFile is where the firmware keeps the remote FTP servers.
const ServerFile = "/Flash/config/ftp_servers"

const (
	defaultPort   = 21
	defaultUser   = "anonymous"
	defaultFolder = "/"
)

// Server is one remote FTP server, in the file's own field names. Password is
// stored URL-encoded, as the firmware writes it.
type Server struct {
	Alias    string `json:"alias"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Folder   string `json:"folder"`
}

// Files is the part of the device's FTP access the server file needs.
type Files interface {
	FTPList(path string) ([]api.FileEntry, error)
	FTPReadFile(path string) ([]byte, error)
	FTPUploadBytes(path string, data []byte) error
}

type serverFile struct {
	Version int      `json:"version"`
	Servers []Server `json:"servers"`
}

// EncodePassword encodes a password the way the firmware's url_encode does:
// letters, digits and _-.* stay, everything else becomes lowercase %xx.
func EncodePassword(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '-', c == '.', c == '*':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02x", c)
		}
	}
	return b.String()
}

// NewServer builds a server entry, filling in the firmware's defaults for a
// port of 0, an empty user and an empty folder.
func NewServer(alias, host string, port int, user, password, folder string) (Server, error) {
	if port == 0 {
		port = defaultPort
	}
	if user == "" {
		user = defaultUser
	}
	if folder == "" {
		folder = defaultFolder
	}
	s := Server{Alias: alias, Host: host, Port: port, User: user, Password: EncodePassword(password), Folder: folder}
	return s, s.validate()
}

func (s Server) validate() error {
	switch {
	case strings.TrimSpace(s.Alias) == "":
		return fmt.Errorf("alias is empty")
	case strings.Contains(s.Alias, "/"):
		return fmt.Errorf("alias %q must not contain a slash: it becomes the directory /ftp/%s", s.Alias, s.Alias)
	case strings.TrimSpace(s.Host) == "":
		return fmt.Errorf("host is empty")
	case s.Port < 1 || s.Port > 65535:
		return fmt.Errorf("port must be 1 to 65535")
	case !strings.HasPrefix(s.Folder, "/"):
		return fmt.Errorf("folder must be absolute, starting with /")
	}
	return nil
}

// Load reads the server list; nil when the device has no file.
func Load(f Files) ([]Server, error) {
	entries, err := f.FTPList(path.Dir(ServerFile))
	if err != nil {
		return nil, err
	}
	found := false
	for _, e := range entries {
		if strings.EqualFold(e.Name, path.Base(ServerFile)) && !e.IsDir {
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	data, err := f.FTPReadFile(ServerFile)
	if err != nil {
		return nil, err
	}
	var file serverFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%s is not a server list: %w", ServerFile, err)
	}
	return file.Servers, nil
}

// Save writes the server list. The firmware reads it at the next power-on.
// Entries are written as they are: new ones pass NewServer, the rest came
// from the device itself.
func Save(f Files, servers []Server) error {
	if servers == nil {
		servers = []Server{}
	}
	data, err := json.Marshal(serverFile{Version: 1, Servers: servers})
	if err != nil {
		return err
	}
	return f.FTPUploadBytes(ServerFile, data)
}

// Put adds s, or replaces the server with the same alias (compared without
// case, as two directories under /ftp could not be told apart otherwise).
func Put(servers []Server, s Server) ([]Server, bool) {
	for i := range servers {
		if strings.EqualFold(servers[i].Alias, s.Alias) {
			servers[i] = s
			return servers, true
		}
	}
	return append(servers, s), false
}

// Remove drops the server with alias; false when there is none.
func Remove(servers []Server, alias string) ([]Server, bool) {
	for i := range servers {
		if strings.EqualFold(servers[i].Alias, alias) {
			return append(servers[:i], servers[i+1:]...), true
		}
	}
	return servers, false
}
