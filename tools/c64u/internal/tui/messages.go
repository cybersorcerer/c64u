package tui

import "github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"

// BackMsg is sent when a component wants to navigate back
// BackMsg is sent when a component wants to navigate back
type BackMsg struct{}

// FilePickerRequestMsg is sent to request a file selection
type FilePickerRequestMsg struct {
	Drive string // The drive requesting the file (e.g. "a", "b")
}

// FilePickedMsg is sent when a file is selected
type FilePickedMsg struct {
	Drive    string
	File     string // Full path
	Filename string
}

// DirPickerRequestMsg requests the file browser to pick a directory
type DirPickerRequestMsg struct{}

// FileDirPickedMsg is sent when a directory is selected
type FileDirPickedMsg struct {
	Path string
}

// GlobalStatusMsg is sent to display a status message in the global status line
type GlobalStatusMsg struct {
	Message string
	IsError bool
}

// ClearStatusMsg is sent to clear the status message after a timeout
type ClearStatusMsg struct {
	ID int // matches statusID to avoid clearing newer messages
}

// FileContentMsg is sent when a file has been read from FTP for viewing
type FileContentMsg struct {
	Filename string
	Content  []byte
	Err      error
}

// deviceInfoMsg carries device info fetched from GetInfo()
type deviceInfoMsg struct {
	Hostname string
	Firmware string
}

// diskDirMsg carries parsed D64 directory entries
type diskDirMsg struct {
	DiskName   string
	DiskPath   string
	Entries    []diskimage.DirEntry
	BlocksUsed int
	Err        error
}
