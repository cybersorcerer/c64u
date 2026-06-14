package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// diskActionItems builds the selector items for a remote disk image.
func diskActionItems(hasPRG bool) []SelectorItem {
	items := []SelectorItem{
		{Label: "Mount to Drive A", Value: "mount-a", Description: "Drive A (8)"},
		{Label: "Mount to Drive B", Value: "mount-b", Description: "Drive B (9)"},
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
	client := m.client
	return func() tea.Msg {
		data, err := client.FTPReadFile(path)
		if err != nil {
			return diskDirLoadedMsg{path: path, err: err}
		}
		entries, name, _, err := diskimage.ReadD64Directory(data)
		if err != nil {
			return diskDirLoadedMsg{path: path, err: err}
		}
		hasPRG := false
		for _, e := range entries {
			if e.FileType == "PRG" {
				hasPRG = true
				break
			}
		}
		return diskDirLoadedMsg{path: path, name: name, hasPRG: hasPRG}
	}
}

func (m *BrowserModel) runPRGCmd(path string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_, err := client.RunPRG(path)
		if err != nil {
			return paneReloadMsg{side: "remote", err: err}
		}
		return paneReloadMsg{side: "remote"}
	}
}

func (m *BrowserModel) diskActionCmd(action string) tea.Cmd {
	client := m.client
	diskPath := m.diskPath
	curDir := m.remote.curDir
	return func() tea.Msg {
		switch action {
		case "mount-a":
			client.DrivesMount("a", diskPath, "", "") //nolint:errcheck
		case "mount-b":
			client.DrivesMount("b", diskPath, "", "") //nolint:errcheck
		case "unmount-a":
			client.DrivesRemove("a") //nolint:errcheck
		case "unmount-b":
			client.DrivesRemove("b") //nolint:errcheck
		case "run-prg":
			data, err := client.FTPReadFile(diskPath)
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
