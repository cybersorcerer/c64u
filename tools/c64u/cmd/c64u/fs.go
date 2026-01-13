package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ============================================================================
// FILESYSTEM COMMANDS (FTP-based)
// ============================================================================

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Filesystem operations via FTP",
	Long: `Complete filesystem access to C64 Ultimate via FTP.

Upload and download files and directories, create directories,
delete, copy, move files, and list directory contents including C64 disk images.

All operations use FTP (port 21) with anonymous login.`,
}

// ============================================================================
// FS LS - List directory contents
// ============================================================================

var fsLsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List directory contents",
	Long: `List files and directories on the C64 Ultimate filesystem.

Examples:
  c64u fs ls /
  c64u fs ls /SD/games
  c64u fs ls /USB0`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		entries, err := apiClient.FTPList(path)
		if err != nil {
			formatter.Error("Failed to list directory", []string{err.Error()})
			return
		}

		if len(entries) == 0 {
			formatter.Info(fmt.Sprintf("Directory is empty: %s", path))
			return
		}

		if jsonOut {
			formatter.PrintData(entries)
		} else {
			formatter.PrintHeader(fmt.Sprintf("📁 %s", path))
			fmt.Println()

			// Prepare table data
			var rows [][]string
			for _, entry := range entries {
				icon := "📄"
				typeStr := "file"
				size := fmt.Sprintf("%d", entry.Size)

				if entry.IsDir {
					icon = "📁"
					typeStr = "dir"
					size = "-"
				}

				rows = append(rows, []string{
					icon,
					entry.Name,
					typeStr,
					size,
				})
			}

			formatter.PrintTable([]string{"", "Name", "Type", "Size"}, rows)
		}
	},
}

// ============================================================================
// FS UPLOAD - Upload file or directory
// ============================================================================

var fsUploadCmd = &cobra.Command{
	Use:   "upload <local-path> <remote-path>",
	Short: "Upload file to C64 Ultimate",
	Long: `Upload a local file to the C64 Ultimate filesystem via FTP.

Examples:
  c64u fs upload game.prg /USB0/games/game.prg
  c64u fs upload disk.d64 /SD/disks/disk.d64`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		localPath := args[0]
		remotePath := args[1]

		// Check if local file exists
		info, err := os.Stat(localPath)
		if err != nil {
			formatter.Error("Local file not found", []string{err.Error()})
			return
		}

		if info.IsDir() {
			formatter.Error("Directory upload not yet supported", []string{
				"Please upload files individually",
			})
			return
		}

		formatter.Info(fmt.Sprintf("Uploading %s to %s...", localPath, remotePath))

		if err := apiClient.FTPUpload(localPath, remotePath); err != nil {
			formatter.Error("Upload failed", []string{err.Error()})
			return
		}

		formatter.Success(fmt.Sprintf("Uploaded %s", filepath.Base(localPath)), map[string]interface{}{
			"local":  localPath,
			"remote": remotePath,
			"size":   fmt.Sprintf("%d bytes", info.Size()),
		})
	},
}

// ============================================================================
// FS DOWNLOAD - Download file or directory
// ============================================================================

var fsDownloadCmd = &cobra.Command{
	Use:   "download <remote-path> <local-path>",
	Short: "Download file from C64 Ultimate",
	Long: `Download a file from the C64 Ultimate filesystem via FTP.

Examples:
  c64u fs download /USB0/games/game.prg ./game.prg
  c64u fs download /SD/disks/disk.d64 ./disk.d64`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		remotePath := args[0]
		localPath := args[1]

		formatter.Info(fmt.Sprintf("Downloading %s to %s...", remotePath, localPath))

		if err := apiClient.FTPDownload(remotePath, localPath); err != nil {
			formatter.Error("Download failed", []string{err.Error()})
			return
		}

		info, _ := os.Stat(localPath)
		formatter.Success(fmt.Sprintf("Downloaded %s", filepath.Base(remotePath)), map[string]interface{}{
			"remote": remotePath,
			"local":  localPath,
			"size":   fmt.Sprintf("%d bytes", info.Size()),
		})
	},
}

// ============================================================================
// FS MKDIR - Create directory
// ============================================================================

var fsMkdirCmd = &cobra.Command{
	Use:   "mkdir <path>",
	Short: "Create directory",
	Long: `Create a new directory on the C64 Ultimate filesystem.

Examples:
  c64u fs mkdir /USB0/newgames
  c64u fs mkdir /SD/backups`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		if err := apiClient.FTPMkdir(path); err != nil {
			formatter.Error("Failed to create directory", []string{err.Error()})
			return
		}

		formatter.Success("Directory created", map[string]interface{}{
			"path": path,
		})
	},
}

// ============================================================================
// FS RM - Remove file or directory
// ============================================================================

var fsRmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Remove file or directory",
	Long: `Delete a file or empty directory on the C64 Ultimate filesystem.

Examples:
  c64u fs rm /USB0/old-game.prg
  c64u fs rm /SD/empty-dir`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		// Try deleting as file first
		err := apiClient.FTPDelete(path)
		if err != nil {
			// If file delete fails, try as directory
			err = apiClient.FTPDeleteDir(path)
			if err != nil {
				formatter.Error("Failed to delete", []string{err.Error()})
				return
			}
		}

		formatter.Success("Deleted", map[string]interface{}{
			"path": path,
		})
	},
}

// ============================================================================
// FS MV - Move/rename file or directory
// ============================================================================

var fsMvCmd = &cobra.Command{
	Use:   "mv <source> <destination>",
	Short: "Move or rename file/directory",
	Long: `Move or rename a file or directory on the C64 Ultimate filesystem.

Examples:
  c64u fs mv /USB0/old-name.prg /USB0/new-name.prg
  c64u fs mv /SD/games /SD/c64-games`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		oldPath := args[0]
		newPath := args[1]

		if err := apiClient.FTPRename(oldPath, newPath); err != nil {
			formatter.Error("Failed to move/rename", []string{err.Error()})
			return
		}

		formatter.Success("Moved/renamed", map[string]interface{}{
			"from": oldPath,
			"to":   newPath,
		})
	},
}

// ============================================================================
// FS CP - Copy file (via download + upload)
// ============================================================================

var fsCpCmd = &cobra.Command{
	Use:   "cp <source> <destination>",
	Short: "Copy file",
	Long: `Copy a file on the C64 Ultimate filesystem.

Note: This downloads the file and re-uploads it to the new location.

Examples:
  c64u fs cp /USB0/game.prg /SD/backup/game.prg`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]
		dest := args[1]

		// Create temp file for download
		tmpFile, err := os.CreateTemp("", "c64u-cp-*")
		if err != nil {
			formatter.Error("Failed to create temp file", []string{err.Error()})
			return
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		formatter.Info("Copying file...")

		// Download source
		if err := apiClient.FTPDownload(source, tmpPath); err != nil {
			formatter.Error("Failed to download source", []string{err.Error()})
			return
		}

		// Upload to destination
		if err := apiClient.FTPUpload(tmpPath, dest); err != nil {
			formatter.Error("Failed to upload to destination", []string{err.Error()})
			return
		}

		formatter.Success("File copied", map[string]interface{}{
			"from": source,
			"to":   dest,
		})
	},
}

// ============================================================================
// FS CAT - Show file info (C64 directories, etc.)
// ============================================================================

var fsCatCmd = &cobra.Command{
	Use:   "cat <path>",
	Short: "Show file information",
	Long: `Display information about a file.

For disk images (.d64, .d71, .d81), this could be extended to show directory contents.

Examples:
  c64u fs cat /USB0/game.prg`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		// For now, just show file info from the files API
		resp, err := apiClient.FilesInfo(path)
		if err != nil {
			formatter.Error("Failed to get file info", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		if jsonOut {
			formatter.PrintData(resp.Data)
		} else {
			files, ok := resp.Data["files"].([]interface{})
			if !ok || len(files) == 0 {
				formatter.Info("No file information available")
				return
			}

			formatter.PrintHeader(fmt.Sprintf("File Information: %s", path))
			fmt.Println()

			for _, fileData := range files {
				fileMap, ok := fileData.(map[string]interface{})
				if !ok {
					continue
				}

				for fileName, fileInfo := range fileMap {
					info, ok := fileInfo.(map[string]interface{})
					if !ok {
						continue
					}

					formatter.PrintKeyValue("Name", fileName)

					if size, ok := info["size"].(float64); ok {
						formatter.PrintKeyValue("Size", fmt.Sprintf("%d bytes", int(size)))
					}

					if ext, ok := info["extension"].(string); ok && ext != "" {
						formatter.PrintKeyValue("Type", strings.ToUpper(ext))
					}

					fmt.Println()
				}
			}
		}
	},
}

func init() {
	// Add subcommands to fs
	fsCmd.AddCommand(fsLsCmd)
	fsCmd.AddCommand(fsUploadCmd)
	fsCmd.AddCommand(fsDownloadCmd)
	fsCmd.AddCommand(fsMkdirCmd)
	fsCmd.AddCommand(fsRmCmd)
	fsCmd.AddCommand(fsMvCmd)
	fsCmd.AddCommand(fsCpCmd)
	fsCmd.AddCommand(fsCatCmd)
}
