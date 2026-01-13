package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jlaffaye/ftp"
)

// File Manipulation API

// FilesInfo returns file size and extension (supports wildcards)
func (c *Client) FilesInfo(path string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/files/%s:info", path)
	return c.Get(endpoint, nil)
}

// FilesCreateD64 creates a D64 image
// path: destination path on C64U filesystem
// tracks: 35 or 40
// diskName: optional disk name
func (c *Client) FilesCreateD64(path string, tracks int, diskName string) (*Response, error) {
	params := make(map[string]string)

	if tracks > 0 {
		params["tracks"] = strconv.Itoa(tracks)
	}

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d64", path)
	return c.Put(endpoint, params)
}

// FilesCreateD71 creates a D71 image (70 tracks fixed)
// path: destination path on C64U filesystem
// diskName: optional disk name
func (c *Client) FilesCreateD71(path string, diskName string) (*Response, error) {
	params := make(map[string]string)

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d71", path)
	return c.Put(endpoint, params)
}

// FilesCreateD81 creates a D81 image (160 tracks fixed)
// path: destination path on C64U filesystem
// diskName: optional disk name
func (c *Client) FilesCreateD81(path string, diskName string) (*Response, error) {
	params := make(map[string]string)

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d81", path)
	return c.Put(endpoint, params)
}

// FilesCreateDNP creates a DNP image (max 255 tracks)
// path: destination path on C64U filesystem
// tracks: number of tracks (max 255, ~16MB)
// diskName: optional disk name
func (c *Client) FilesCreateDNP(path string, tracks int, diskName string) (*Response, error) {
	params := map[string]string{
		"tracks": strconv.Itoa(tracks),
	}

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_dnp", path)
	return c.Put(endpoint, params)
}

// FTP-based Filesystem Operations

// FileEntry represents a file or directory entry
type FileEntry struct {
	Name  string
	Size  uint64
	IsDir bool
	Type  string // "file" or "dir"
}

// getFTPConn creates an FTP connection to the C64 Ultimate
// C64 Ultimate FTP is on port 21, anonymous login
func (c *Client) getFTPConn() (*ftp.ServerConn, error) {
	// Extract host from BaseURL (remove http:// and port)
	host := strings.TrimPrefix(c.BaseURL, "http://")
	host = strings.TrimPrefix(host, "https://")
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}

	conn, err := ftp.Dial(fmt.Sprintf("%s:21", host))
	if err != nil {
		return nil, fmt.Errorf("FTP dial failed: %w", err)
	}

	// C64 Ultimate uses anonymous FTP
	if err := conn.Login("anonymous", "anonymous"); err != nil {
		conn.Quit()
		return nil, fmt.Errorf("FTP login failed: %w", err)
	}

	return conn, nil
}

// FTPList lists directory contents via FTP
func (c *Client) FTPList(path string) ([]FileEntry, error) {
	conn, err := c.getFTPConn()
	if err != nil {
		return nil, err
	}
	defer conn.Quit()

	entries, err := conn.List(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var result []FileEntry
	for _, entry := range entries {
		fileEntry := FileEntry{
			Name:  entry.Name,
			Size:  entry.Size,
			IsDir: entry.Type == ftp.EntryTypeFolder,
		}
		if fileEntry.IsDir {
			fileEntry.Type = "dir"
		} else {
			fileEntry.Type = "file"
		}
		result = append(result, fileEntry)
	}

	return result, nil
}

// FTPUpload uploads a local file to C64 Ultimate via FTP
func (c *Client) FTPUpload(localPath, remotePath string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	// Create remote directories if needed
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		c.ftpMkdirAll(conn, remoteDir)
	}

	if err := conn.Stor(remotePath, file); err != nil {
		return fmt.Errorf("FTP upload failed: %w", err)
	}

	return nil
}

// FTPDownload downloads a file from C64 Ultimate via FTP
func (c *Client) FTPDownload(remotePath, localPath string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	resp, err := conn.Retr(remotePath)
	if err != nil {
		return fmt.Errorf("FTP download failed: %w", err)
	}
	defer resp.Close()

	// Create local directory if needed
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// FTPMkdir creates a directory on C64 Ultimate via FTP
func (c *Client) FTPMkdir(path string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	if err := conn.MakeDir(path); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// ftpMkdirAll creates all directories in path (like mkdir -p)
func (c *Client) ftpMkdirAll(conn *ftp.ServerConn, path string) error {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		conn.MakeDir("/" + current) // Ignore errors, dir might exist
	}
	return nil
}

// FTPDelete deletes a file on C64 Ultimate via FTP
func (c *Client) FTPDelete(path string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	if err := conn.Delete(path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// FTPDeleteDir deletes a directory on C64 Ultimate via FTP
func (c *Client) FTPDeleteDir(path string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	if err := conn.RemoveDir(path); err != nil {
		return fmt.Errorf("failed to delete directory: %w", err)
	}

	return nil
}

// FTPRename renames/moves a file on C64 Ultimate via FTP
func (c *Client) FTPRename(oldPath, newPath string) error {
	conn, err := c.getFTPConn()
	if err != nil {
		return err
	}
	defer conn.Quit()

	if err := conn.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	return nil
}
