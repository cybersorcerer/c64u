package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/audio"
	dbg "github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debugger"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/diskimage"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/network"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/video"
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

var filesPackD64Cmd = &cobra.Command{
	Use:   "pack-d64 <source-dir> <output-file> [--name NAME] [--id ID] [--tracks N]",
	Short: "Pack directory into D64 disk image",
	Long: `Create a D64 disk image from all files in a directory.

Reads all files from the source directory and packs them into a D64 disk image.
Supports .prg, .seq, .usr, and .rel files.

To upload the created D64 to C64 Ultimate, use: c64u fs upload <output-file> <remote-path>

Examples:
  c64u files pack-d64 ./myfiles game.d64 --name "MY GAME" --id "01"
  c64u files pack-d64 ./build output.d64 --tracks 40

  # Upload to C64 Ultimate after creation
  c64u files pack-d64 ./myfiles game.d64 && c64u fs upload game.d64 /SD/game.d64`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runPackD64(cmd, args)
	},
}

func runPackD64(cmd *cobra.Command, args []string) {
	sourceDir := args[0]
	outputPath := args[1]

	diskName, _ := cmd.Flags().GetString("name")
	diskID, _ := cmd.Flags().GetString("id")
	tracks, _ := cmd.Flags().GetInt("tracks")

	// Use directory name as disk name if not specified
	if diskName == "" {
		diskName = filepath.Base(sourceDir)
		if len(diskName) > 16 {
			diskName = diskName[:16]
		}
	}

	// Default disk ID
	if diskID == "" {
		diskID = "2A"
	}

	// Default tracks
	if tracks == 0 {
		tracks = 35
	}

	// Validate tracks
	if tracks != 35 && tracks != 40 {
		formatter.Error("Invalid track count", []string{
			fmt.Sprintf("Track count must be 35 or 40, got %d", tracks),
		})
		return
	}

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		formatter.Error("Source directory not found", []string{sourceDir})
		return
	}

	// Create pack options
	opts := &diskimage.PackOptions{
		DiskName: diskName,
		DiskID:   diskID,
		Tracks:   tracks,
	}

	// Pack directory into D64
	if err := diskimage.PackDirectory(sourceDir, outputPath, opts); err != nil {
		formatter.Error("Failed to create D64 image", []string{err.Error()})
		return
	}

	// Get file info
	files, _ := diskimage.GetDiskInfo(sourceDir, nil)

	data := map[string]interface{}{
		"output":     outputPath,
		"disk_name":  diskName,
		"disk_id":    diskID,
		"tracks":     tracks,
		"file_count": len(files),
	}

	formatter.Success("D64 image created", data)

	// Show files included
	if !jsonOut {
		fmt.Println()
		formatter.PrintHeader("Files Included")
		fmt.Println()
		for i, file := range files {
			// Get icon for file type
			icon := "·"
			ext := strings.ToLower(filepath.Ext(file.Name))
			switch ext {
			case ".prg", ".p00":
				icon = "▶"
			case ".seq", ".s00", ".txt", ".asm", ".sym", ".bas", ".md", ".log":
				icon = "―"
			case ".usr", ".u00":
				icon = "◆"
			}
			fmt.Printf("  %s  %2d. %s\n", icon, i+1, file.Name)
		}
		fmt.Println()
	}
}

const debugStreamPort = 11002

var streamsListenDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Listen to the debug stream",
	Long: `Start the C64 Ultimate debug stream and print incoming data to stdout.

The local IP address is detected automatically. Use --ip to override.
Use --raw to get plain output suitable for piping to other tools.

Examples:
  c64u streams listen debug
  c64u streams listen debug --ip 192.168.1.50
  c64u streams listen debug --raw | grep SID`,
	Run: func(cmd *cobra.Command, args []string) {
		listenIP, _ := cmd.Flags().GetString("ip")
		raw, _ := cmd.Flags().GetBool("raw")
		tuiMode, _ := cmd.Flags().GetBool("tui")

		// Determine local IP
		if listenIP == "" {
			detected, err := network.LocalIP(host)
			if err != nil {
				formatter.Error("Cannot detect local IP address", []string{
					err.Error(),
					"Use --ip to specify your IP manually",
				})
				return
			}
			listenIP = detected
		}

		listenAddr := fmt.Sprintf("%s:%d", listenIP, debugStreamPort)

		// Update the "Stream Debug to" config so the C64 knows where to send data
		if err := apiClient.SetConfigItem("Data Streams", "Stream Debug to", listenAddr); err != nil {
			formatter.Error("Failed to set debug stream destination", []string{err.Error()})
			return
		}

		// Start stream on C64 Ultimate (IP only, port comes from config)
		resp, err := apiClient.StreamsStart("debug", listenIP)
		if err != nil {
			formatter.Error("Failed to start debug stream", []string{err.Error()})
			return
		}
		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		// Open UDP socket
		udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", debugStreamPort))
		if err != nil {
			formatter.Error("Failed to resolve UDP address", []string{err.Error()})
			apiClient.StreamsStop("debug")
			return
		}
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			formatter.Error("Failed to open UDP socket", []string{
				err.Error(),
				fmt.Sprintf("Make sure port %d is not already in use", debugStreamPort),
			})
			apiClient.StreamsStop("debug")
			return
		}

		// ── TUI mode ──────────────────────────────────────────────────────────
		if tuiMode {
			if err := debugger.Run(conn); err != nil {
				formatter.Error("Debugger TUI error", []string{err.Error()})
			}
			apiClient.StreamsStop("debug")
			return
		}

		// ── Plain text mode ───────────────────────────────────────────────────
		if !raw {
			formatter.Success("Debug stream started", map[string]interface{}{
				"listening_on": listenAddr,
			})
		}

		// Stop stream and close socket on Ctrl+C / SIGTERM
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sig
			conn.Close()
		}()

		if !raw {
			fmt.Println("Receiving debug stream. Press Ctrl+C to stop.")
			fmt.Println()
		}

		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				// Socket closed by signal handler — clean exit
				break
			}
			if n > 0 {
				dbg.DecodePacket(buf[:n], os.Stdout, raw)
			}
		}

		// Stop stream on C64 Ultimate
		apiClient.StreamsStop("debug")
		if !raw {
			fmt.Println()
			formatter.Success("Debug stream stopped", nil)
		}
	},
}

var streamsListenAudioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Listen to the audio stream and play it back",
	Long: `Start the C64 Ultimate audio stream and play it back via your speakers.

The local IP address is detected automatically. Use --ip to override.
Audio is 48kHz stereo 16-bit signed PCM from the C64 audio mixer.
Press Ctrl+C to stop.

Examples:
  c64u streams listen audio
  c64u streams listen audio --ip 192.168.1.50`,
	Run: func(cmd *cobra.Command, args []string) {
		listenIP, _ := cmd.Flags().GetString("ip")

		if listenIP == "" {
			detected, err := network.LocalIP(host)
			if err != nil {
				formatter.Error("Cannot detect local IP address", []string{
					err.Error(),
					"Use --ip to specify your IP manually",
				})
				return
			}
			listenIP = detected
		}

		if err := audio.Listen(
			listenIP,
			func(ip string) error {
				if err := apiClient.SetConfigItem("Data Streams", "Stream Audio to", fmt.Sprintf("%s:%d", ip, audio.AudioPort)); err != nil {
					return err
				}
				resp, err := apiClient.StreamsStart("audio", ip)
				if err != nil {
					return err
				}
				if resp.HasErrors() {
					return fmt.Errorf("%s", resp.Errors[0])
				}
				formatter.Success("Audio stream started", map[string]interface{}{
					"listening_on": fmt.Sprintf("%s:%d", ip, audio.AudioPort),
				})
				return nil
			},
			func() error {
				apiClient.StreamsStop("audio")
				formatter.Success("Audio stream stopped", nil)
				return nil
			},
		); err != nil {
			formatter.Error("Audio stream error", []string{err.Error()})
		}
	},
}

var streamsListenVideoCmd = &cobra.Command{
	Use:   "video",
	Short: "Listen to the video stream and display it in the terminal",
	Long: `Start the C64 Ultimate video stream and render it in the terminal.

The local IP address is detected automatically. Use --ip to override.
The video is rendered using Unicode half-block characters with true VIC colors.
Press Ctrl+C to stop.

Examples:
  c64u streams listen video
  c64u streams listen video --ip 192.168.1.50`,
	Run: func(cmd *cobra.Command, args []string) {
		listenIP, _ := cmd.Flags().GetString("ip")

		if listenIP == "" {
			detected, err := network.LocalIP(host)
			if err != nil {
				formatter.Error("Cannot detect local IP address", []string{
					err.Error(),
					"Use --ip to specify your IP manually",
				})
				return
			}
			listenIP = detected
		}

		if err := video.Listen(
			listenIP,
			func(ip string) error {
				resp, err := apiClient.StreamsStart("video", ip)
				if err != nil {
					return err
				}
				if resp.HasErrors() {
					return fmt.Errorf("%s", resp.Errors[0])
				}
				return nil
			},
			func() error {
				apiClient.StreamsStop("video")
				return nil
			},
		); err != nil {
			formatter.Error("Video stream error", []string{err.Error()})
		}
	},
}

// streamsListenCmd is the "listen" parent that holds sub-streams
var streamsListenCmd = &cobra.Command{
	Use:   "listen <stream>",
	Short: "Listen to a data stream",
	Long:  `Listen to a data stream from the C64 Ultimate. Currently supports: debug`,
}

func init() {
	// Streams commands
	streamsCmd.AddCommand(streamsStartCmd)
	streamsCmd.AddCommand(streamsStopCmd)
	streamsCmd.AddCommand(streamsListenCmd)

	streamsListenCmd.AddCommand(streamsListenDebugCmd)
	streamsListenCmd.AddCommand(streamsListenVideoCmd)
	streamsListenCmd.AddCommand(streamsListenAudioCmd)

	streamsListenDebugCmd.Flags().String("ip", "", "Local IP address to receive stream (auto-detected if omitted)")
	streamsListenDebugCmd.Flags().Bool("raw", false, "Raw output, suitable for piping")
	streamsListenDebugCmd.Flags().Bool("tui", false, "Start interactive TUI debugger")
	streamsListenVideoCmd.Flags().String("ip", "", "Local IP address to receive stream (auto-detected if omitted)")
	streamsListenAudioCmd.Flags().String("ip", "", "Local IP address to receive stream (auto-detected if omitted)")

	// Files commands
	filesCmd.AddCommand(filesInfoCmd)
	filesCmd.AddCommand(filesCreateD64Cmd)
	filesCmd.AddCommand(filesCreateD71Cmd)
	filesCmd.AddCommand(filesCreateD81Cmd)
	filesCmd.AddCommand(filesCreateDNPCmd)
	filesCmd.AddCommand(filesPackD64Cmd)

	// Flags for file creation commands
	filesCreateD64Cmd.Flags().Int("tracks", 35, "Number of tracks (35 or 40)")
	filesCreateD64Cmd.Flags().String("name", "", "Disk name")
	filesCreateD71Cmd.Flags().String("name", "", "Disk name")
	filesCreateD81Cmd.Flags().String("name", "", "Disk name")
	filesCreateDNPCmd.Flags().Int("tracks", 0, "Number of tracks (max 255)")
	filesCreateDNPCmd.Flags().String("name", "", "Disk name")
	filesCreateDNPCmd.MarkFlagRequired("tracks")

	// Flags for pack-d64 command
	filesPackD64Cmd.Flags().String("name", "", "Disk name (max 16 chars, defaults to directory name)")
	filesPackD64Cmd.Flags().String("id", "2A", "Disk ID (2 chars)")
	filesPackD64Cmd.Flags().Int("tracks", 35, "Number of tracks (35 or 40)")
}
