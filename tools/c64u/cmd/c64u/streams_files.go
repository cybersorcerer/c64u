package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ============================================================================
// STREAMS COMMANDS (U64 Only)
// ============================================================================

var streamsCmd = &cobra.Command{
	Use:   "streams",
	Short: "Data streams control (U64 only)",
	Long: `Start and stop video, audio, and debug streams on Ultimate 64.

Streams are sent to the specified IP address on default ports:
- video: port 11000
- audio: port 11001
- debug: port 11002

This feature is only available on Ultimate 64 hardware.`,
}

var streamsStartCmd = &cobra.Command{
	Use:   "start <stream> <ip>",
	Short: "Start a stream",
	Long: `Start a video, audio, or debug stream to the specified IP address.

Streams: video, audio, debug

Example:
  c64u streams start video 192.168.1.100`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		stream := args[0]
		ip := args[1]

		// Validate stream type
		validStreams := map[string]bool{"video": true, "audio": true, "debug": true}
		if !validStreams[stream] {
			formatter.Error("Invalid stream type", []string{
				fmt.Sprintf("Stream '%s' is not valid", stream),
				"Valid streams: video, audio, debug",
			})
			return
		}

		resp, err := apiClient.StreamsStart(stream, ip)
		if err != nil {
			formatter.Error("Failed to start stream", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		// Get default port for stream type
		ports := map[string]int{"video": 11000, "audio": 11001, "debug": 11002}
		data := map[string]interface{}{
			"stream":      stream,
			"destination": fmt.Sprintf("%s:%d", ip, ports[stream]),
		}
		formatter.Success("Stream started", data)
	},
}

var streamsStopCmd = &cobra.Command{
	Use:   "stop <stream>",
	Short: "Stop a stream",
	Long: `Stop the specified video, audio, or debug stream.

Streams: video, audio, debug

Example:
  c64u streams stop video`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		stream := args[0]

		// Validate stream type
		validStreams := map[string]bool{"video": true, "audio": true, "debug": true}
		if !validStreams[stream] {
			formatter.Error("Invalid stream type", []string{
				fmt.Sprintf("Stream '%s' is not valid", stream),
				"Valid streams: video, audio, debug",
			})
			return
		}

		resp, err := apiClient.StreamsStop(stream)
		if err != nil {
			formatter.Error("Failed to stop stream", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Stream '%s' stopped", stream), nil)
	},
}

// ============================================================================
// FILES COMMANDS
// ============================================================================

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "File operations",
	Long: `File manipulation on the C64 Ultimate filesystem.

Commands include getting file info and creating disk images (D64, D71, D81, DNP).`,
}

var filesInfoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Get file information",
	Long: `Returns file size and extension. Supports wildcards.

Example:
  c64u files info /usb0/games/*.d64`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

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
			// Parse file info data
			files, ok := resp.Data["files"].([]interface{})
			if !ok || len(files) == 0 {
				formatter.Info("No files found")
				return
			}

			formatter.PrintHeader(fmt.Sprintf("File Information: %s", path))
			fmt.Println()

			for _, fileData := range files {
				fileMap, ok := fileData.(map[string]interface{})
				if !ok {
					continue
				}

				// File name is the key
				for fileName, fileInfo := range fileMap {
					info, ok := fileInfo.(map[string]interface{})
					if !ok {
						continue
					}

					formatter.PrintHeader(fileName)
					fmt.Println()

					if size, ok := info["size"].(float64); ok {
						formatter.PrintKeyValue("Size", fmt.Sprintf("%d bytes", int(size)))
					}

					if ext, ok := info["extension"].(string); ok && ext != "" {
						formatter.PrintKeyValue("Type", ext)
					}

					fmt.Println()
				}
			}
		}
	},
}

var filesCreateD64Cmd = &cobra.Command{
	Use:   "create-d64 <path> [--tracks N] [--name NAME]",
	Short: "Create D64 disk image",
	Long: `Create a new D64 disk image on the C64 Ultimate filesystem.

Tracks: 35 (standard) or 40 (extended)

Example:
  c64u files create-d64 /usb0/newdisk.d64 --tracks 35 --name "MY DISK"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		tracks, _ := cmd.Flags().GetInt("tracks")
		name, _ := cmd.Flags().GetString("name")

		resp, err := apiClient.FilesCreateD64(path, tracks, name)
		if err != nil {
			formatter.Error("Failed to create D64 image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"path": path,
		}
		if tracks > 0 {
			data["tracks"] = tracks
		}
		if name != "" {
			data["name"] = name
		}
		formatter.Success("D64 image created", data)
	},
}

var filesCreateD71Cmd = &cobra.Command{
	Use:   "create-d71 <path> [--name NAME]",
	Short: "Create D71 disk image",
	Long: `Create a new D71 disk image (70 tracks) on the C64 Ultimate filesystem.

Example:
  c64u files create-d71 /usb0/newdisk.d71 --name "MY DISK"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		name, _ := cmd.Flags().GetString("name")

		resp, err := apiClient.FilesCreateD71(path, name)
		if err != nil {
			formatter.Error("Failed to create D71 image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"path":   path,
			"tracks": 70,
		}
		if name != "" {
			data["name"] = name
		}
		formatter.Success("D71 image created", data)
	},
}

var filesCreateD81Cmd = &cobra.Command{
	Use:   "create-d81 <path> [--name NAME]",
	Short: "Create D81 disk image",
	Long: `Create a new D81 disk image (160 tracks) on the C64 Ultimate filesystem.

Example:
  c64u files create-d81 /usb0/newdisk.d81 --name "MY DISK"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		name, _ := cmd.Flags().GetString("name")

		resp, err := apiClient.FilesCreateD81(path, name)
		if err != nil {
			formatter.Error("Failed to create D81 image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"path":   path,
			"tracks": 160,
		}
		if name != "" {
			data["name"] = name
		}
		formatter.Success("D81 image created", data)
	},
}

var filesCreateDNPCmd = &cobra.Command{
	Use:   "create-dnp <path> --tracks N [--name NAME]",
	Short: "Create DNP disk image",
	Long: `Create a new DNP disk image (max 255 tracks, ~16MB) on the C64 Ultimate filesystem.

Example:
  c64u files create-dnp /usb0/bigdisk.dnp --tracks 200 --name "BIG DISK"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		tracks, _ := cmd.Flags().GetInt("tracks")
		name, _ := cmd.Flags().GetString("name")

		if tracks == 0 {
			formatter.Error("Missing required flag", []string{"--tracks is required for DNP images"})
			return
		}

		if tracks > 255 {
			formatter.Error("Invalid tracks value", []string{"DNP images support max 255 tracks"})
			return
		}

		resp, err := apiClient.FilesCreateDNP(path, tracks, name)
		if err != nil {
			formatter.Error("Failed to create DNP image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"path":   path,
			"tracks": tracks,
		}
		if name != "" {
			data["name"] = name
		}
		formatter.Success("DNP image created", data)
	},
}

func init() {
	// Streams commands
	streamsCmd.AddCommand(streamsStartCmd)
	streamsCmd.AddCommand(streamsStopCmd)

	// Files commands
	filesCmd.AddCommand(filesInfoCmd)
	filesCmd.AddCommand(filesCreateD64Cmd)
	filesCmd.AddCommand(filesCreateD71Cmd)
	filesCmd.AddCommand(filesCreateD81Cmd)
	filesCmd.AddCommand(filesCreateDNPCmd)

	// Flags for file creation commands
	filesCreateD64Cmd.Flags().Int("tracks", 35, "Number of tracks (35 or 40)")
	filesCreateD64Cmd.Flags().String("name", "", "Disk name")
	filesCreateD71Cmd.Flags().String("name", "", "Disk name")
	filesCreateD81Cmd.Flags().String("name", "", "Disk name")
	filesCreateDNPCmd.Flags().Int("tracks", 0, "Number of tracks (max 255)")
	filesCreateDNPCmd.Flags().String("name", "", "Disk name")
	filesCreateDNPCmd.MarkFlagRequired("tracks")
}
