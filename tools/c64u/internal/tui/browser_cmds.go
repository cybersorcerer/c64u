package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
)

// joinColumns places rendered column strings side by side with single-space gaps.
func joinColumns(cols ...string) string {
	withGaps := make([]string, 0, len(cols)*2)
	for i, c := range cols {
		if i > 0 {
			withGaps = append(withGaps, " ")
		}
		withGaps = append(withGaps, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, withGaps...)
}

// diskActionItems builds the selector items for a disk image. A local image is
// mounted by uploading it, so its mount entries are labelled accordingly.
func diskActionItems(hasPRG, isLocal bool) []SelectorItem {
	mountA, mountB := "Drive A (8)", "Drive B (9)"
	if isLocal {
		mountA, mountB = "Upload and mount to Drive A (8)", "Upload and mount to Drive B (9)"
	}
	items := []SelectorItem{
		{Label: "Mount to Drive A", Value: "mount-a", Description: mountA},
		{Label: "Mount to Drive B", Value: "mount-b", Description: mountB},
		{Label: "Unmount Drive A", Value: "unmount-a", Description: "Eject A"},
		{Label: "Unmount Drive B", Value: "unmount-b", Description: "Eject B"},
	}
	if hasPRG {
		items = append([]SelectorItem{
			{Label: "Run first PRG", Value: "run-prg", Description: "Extract and run"},
		}, items...)
	}
	return items
}

// refreshPreviewCmd loads content for the item under the active cursor.
func (m *BrowserModel) refreshPreviewCmd() tea.Cmd {
	if !m.showPreview {
		return nil
	}
	cur, ok := m.activeP.currentItem()
	if !ok || cur.IsDir {
		m.previewData = nil
		m.previewName = ""
		return nil
	}
	if cur.Name == m.previewName {
		return nil
	}
	src := m.activeP.src
	path := m.activeP.join(cur.Name)
	name := cur.Name
	return func() tea.Msg {
		data, err := src.ReadFile(path)
		if err != nil {
			return previewLoadedMsg{name: name, data: nil}
		}
		return previewLoadedMsg{name: name, data: data}
	}
}

// copyCmd copies selected (or current) files from the active pane to the other.
func (m *BrowserModel) copyCmd() tea.Cmd {
	srcPane := m.activeP
	dstPane := m.other()

	items := srcPane.selected()
	if len(items) == 0 {
		if cur, ok := srcPane.currentItem(); ok {
			items = []fileItem{cur}
		}
	}
	if len(items) == 0 {
		return nil
	}

	srcSource := srcPane.src
	dstSource := dstPane.src
	srcDir := srcPane.curDir
	dstDir := dstPane.curDir
	srcLocal := srcSource.IsLocal()
	dstLocal := dstSource.IsLocal()

	return func() tea.Msg {
		count := 0
		for _, it := range items {
			if it.IsDir {
				continue
			}
			var sp string
			if srcLocal {
				sp = localJoin(srcDir, it.Name)
			} else {
				sp = joinPath(srcDir, it.Name)
			}
			data, err := srcSource.ReadFile(sp)
			if err != nil {
				return transferDoneMsg{count: count, err: fmt.Errorf("%s: %w", it.Name, err)}
			}
			var dp string
			if dstLocal {
				dp = localJoin(dstDir, it.Name)
			} else {
				dp = joinPath(dstDir, it.Name)
			}
			if err := dstSource.WriteFile(dp, data); err != nil {
				return transferDoneMsg{count: count, err: fmt.Errorf("%s: %w", it.Name, err)}
			}
			count++
		}
		return transferDoneMsg{count: count}
	}
}

func (m *BrowserModel) deleteCmd(it fileItem) tea.Cmd {
	src := m.activeP.src
	path := m.activeP.join(it.Name)
	isDir := it.IsDir
	pane := m.activeP
	return func() tea.Msg {
		if err := src.Delete(path, isDir); err != nil {
			return paneReloadMsg{side: "active", err: err}
		}
		pane.reload() //nolint:errcheck
		return paneReloadMsg{side: "active"}
	}
}

func (m *BrowserModel) renameCmd(oldName, newName string) tea.Cmd {
	src := m.activeP.src
	oldPath := m.activeP.join(oldName)
	newPath := m.activeP.join(newName)
	pane := m.activeP
	return func() tea.Msg {
		if err := src.Rename(oldPath, newPath); err != nil {
			return paneReloadMsg{side: "active", err: err}
		}
		pane.reload() //nolint:errcheck
		return paneReloadMsg{side: "active"}
	}
}

func (m *BrowserModel) mkdirCmd(name string) tea.Cmd {
	src := m.activeP.src
	path := m.activeP.join(name)
	pane := m.activeP
	return func() tea.Msg {
		if err := src.Mkdir(path); err != nil {
			return paneReloadMsg{side: "active", err: err}
		}
		pane.reload() //nolint:errcheck
		return paneReloadMsg{side: "active"}
	}
}

func (m *BrowserModel) createDiskCmd(format, name string) tea.Cmd {
	client := m.client
	path := m.remote.join(name)
	remote := m.remote
	return func() tea.Msg {
		var err error
		switch format {
		case "d64":
			_, err = client.FilesCreateD64(path, 35, "")
		case "d71":
			_, err = client.FilesCreateD71(path, "")
		case "d81":
			_, err = client.FilesCreateD81(path, "")
		}
		if err != nil {
			return paneReloadMsg{side: "remote", err: err}
		}
		remote.reload() //nolint:errcheck
		return paneReloadMsg{side: "remote"}
	}
}

func (m *BrowserModel) loadDiskDirCmd(path string) tea.Cmd {
	src := m.activeP.src
	isLocal := src.IsLocal()
	return func() tea.Msg {
		data, err := src.ReadFile(path)
		if err != nil {
			return diskDirLoadedMsg{path: path, isLocal: isLocal, err: err}
		}
		entries, name, _, err := diskimage.ReadD64Directory(data)
		if err != nil {
			return diskDirLoadedMsg{path: path, isLocal: isLocal, err: err}
		}
		hasPRG := false
		for _, e := range entries {
			if e.FileType == "PRG" {
				hasPRG = true
				break
			}
		}
		return diskDirLoadedMsg{path: path, name: name, hasPRG: hasPRG, isLocal: isLocal}
	}
}

func (m *BrowserModel) diskActionCmd(action string) tea.Cmd {
	client := m.client
	src := m.diskSrc
	isLocal := m.diskIsLocal
	diskPath := m.diskPath
	curDir := m.remote.curDir
	return func() tea.Msg {
		switch action {
		case "mount-a", "mount-b":
			drive := strings.TrimPrefix(action, "mount-")
			if isLocal {
				client.DrivesMountUpload(drive, diskPath, "", "readwrite") //nolint:errcheck
			} else {
				client.DrivesMount(drive, diskPath, "", "") //nolint:errcheck
			}
		case "unmount-a":
			client.DrivesRemove("a") //nolint:errcheck
		case "unmount-b":
			client.DrivesRemove("b") //nolint:errcheck
		case "run-prg":
			// The PRG is extracted from the image and staged on the C64U before
			// running, regardless of which pane the image came from.
			data, err := src.ReadFile(diskPath)
			if err != nil {
				return paneReloadMsg{side: "remote", err: err}
			}
			entries, _, _, _ := diskimage.ReadD64Directory(data)
			var prgName string
			for _, e := range entries {
				if e.FileType == "PRG" {
					prgName = e.Name
					break
				}
			}
			if prgName == "" {
				return paneReloadMsg{side: "remote", err: fmt.Errorf("no PRG in disk")}
			}
			prgData, err := diskimage.ExtractPRG(data, prgName)
			if err != nil {
				return paneReloadMsg{side: "remote", err: err}
			}
			tmp := joinPath(curDir, "_tmp_"+strings.ReplaceAll(prgName, " ", "_")+".prg")
			if err := client.FTPUploadBytes(tmp, prgData); err != nil {
				return paneReloadMsg{side: "remote", err: err}
			}
			client.RunPRG(tmp)    //nolint:errcheck
			client.FTPDelete(tmp) //nolint:errcheck
		}
		return paneReloadMsg{side: "remote"}
	}
}

// uploadCmd applies a file to a drive. A local disk image goes via mount-upload
// (readwrite); a ROM goes via load-rom-upload from the local pane, or via
// load-rom by path when it already lives on the C64U.
func (m *BrowserModel) uploadCmd(drive, path string, isROM, isRemote bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var resp *api.Response
		var err error
		switch {
		case isROM && isRemote:
			resp, err = client.DrivesLoadROM(drive, path)
		case isROM:
			resp, err = client.DrivesLoadROMUpload(drive, path)
		default:
			resp, err = client.DrivesMountUpload(drive, path, "", "readwrite")
		}
		if err != nil {
			return transferDoneMsg{err: err}
		}
		if resp.HasErrors() {
			return transferDoneMsg{err: fmt.Errorf("%s", resp.Errors[0])}
		}
		action := "Mounted"
		if isROM {
			action = "Loaded ROM"
		}
		return transferDoneMsg{count: 1, name: action + " to Drive " + strings.ToUpper(drive)}
	}
}

// runnerCmd executes a media/program runner. action selects the API call;
// path is a remote path for the non-upload variants, or a local path for the
// "-upload" variants. Status is reported via transferDoneMsg.
func (m *BrowserModel) runnerCmd(action, path string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var resp *api.Response
		var err error
		var label string
		switch action {
		case "run-prg":
			resp, err = client.RunPRG(path)
			label = "Running"
		case "load-prg":
			resp, err = client.LoadPRG(path)
			label = "Loaded"
		case "run-prg-upload":
			resp, err = client.RunPRGUpload(path)
			label = "Running"
		case "load-prg-upload":
			resp, err = client.LoadPRGUpload(path)
			label = "Loaded"
		case "run-crt":
			resp, err = client.RunCRT(path)
			label = "Running cartridge"
		case "run-crt-upload":
			resp, err = client.RunCRTUpload(path)
			label = "Running cartridge"
		case "sid":
			resp, err = client.SidPlay(path, 0)
			label = "Playing SID"
		case "sid-upload":
			resp, err = client.SidPlayUpload(path, 0)
			label = "Playing SID"
		case "mod":
			resp, err = client.ModPlay(path)
			label = "Playing MOD"
		case "mod-upload":
			resp, err = client.ModPlayUpload(path)
			label = "Playing MOD"
		default:
			return transferDoneMsg{err: fmt.Errorf("unknown runner %q", action)}
		}
		if err != nil {
			return transferDoneMsg{err: err}
		}
		if resp != nil && resp.HasErrors() {
			return transferDoneMsg{err: fmt.Errorf("%s", resp.Errors[0])}
		}
		return transferDoneMsg{count: 1, name: label + ": " + baseName(path)}
	}
}

// baseName returns the last path component (forward-slash or OS separator).
func baseName(path string) string {
	p := strings.TrimRight(path, "/")
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
