package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// drivesCmd represents the drives command group
var drivesCmd = &cobra.Command{
	Use:   "drives",
	Short: "Floppy drive operations",
	Long: `Manage floppy drives on the C64 Ultimate.

Commands include mounting/unmounting disk images, resetting drives,
enabling/disabling drives, loading custom ROMs, and changing drive modes.`,
}

// ============================================================================
// Drive Information
// ============================================================================

var drivesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all drives and mounted images",
	Long:  `Returns information on all internal drives including currently mounted images.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.DrivesList()
		if err != nil {
			formatter.Error("Failed to list drives", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		if jsonOut {
			formatter.PrintData(resp.Data)
		} else {
			// Parse drives data
			drives, ok := resp.Data["drives"].([]interface{})
			if !ok || len(drives) == 0 {
				formatter.Info("No drives found")
				return
			}

			formatter.PrintHeader("C64 Ultimate Drives")
			fmt.Println()

			// Print each drive
			for _, driveData := range drives {
				driveMap, ok := driveData.(map[string]interface{})
				if !ok {
					continue
				}

				// Each drive is a map with one key (the drive name)
				for driveName, driveInfo := range driveMap {
					info, ok := driveInfo.(map[string]interface{})
					if !ok {
						continue
					}

					// Print drive header
					enabledText := ""
					if e, ok := info["enabled"].(bool); ok && e {
						enabledText = " (Enabled ✓)"
					} else {
						enabledText = " (Disabled ✗)"
					}

					formatter.PrintHeader(fmt.Sprintf("%s%s", driveName, enabledText))
					fmt.Println()

					// Print drive details
					if busID, ok := info["bus_id"].(float64); ok {
						formatter.PrintKeyValue("Bus ID", fmt.Sprintf("%d", int(busID)))
					}

					if driveType, ok := info["type"].(string); ok && driveType != "" {
						formatter.PrintKeyValue("Type", driveType)
					}

					if rom, ok := info["rom"].(string); ok && rom != "" {
						formatter.PrintKeyValue("ROM", rom)
					}

					// Image info
					if imageName, ok := info["image_file"].(string); ok && imageName != "" {
						formatter.PrintKeyValue("Image", imageName)
						if imagePath, ok := info["image_path"].(string); ok && imagePath != "" {
							formatter.PrintKeyValue("Path", imagePath)
						}
					} else {
						fmt.Println("  No disk mounted")
					}

					// Partitions info
					if partitions, ok := info["partitions"].([]interface{}); ok && len(partitions) > 0 {
						fmt.Println()
						fmt.Println("  Partitions:")
						for _, partition := range partitions {
							if partMap, ok := partition.(map[string]interface{}); ok {
								partID := ""
								partPath := ""
								if id, ok := partMap["id"].(float64); ok {
									partID = fmt.Sprintf("%d", int(id))
								}
								if path, ok := partMap["path"].(string); ok {
									partPath = path
								}
								if partID != "" && partPath != "" {
									fmt.Printf("    [%s] %s\n", partID, partPath)
								}
							}
						}
					}

					// Last error info
					if lastError, ok := info["last_error"].(string); ok && lastError != "" {
						fmt.Println()
						formatter.PrintKeyValue("Last Error", lastError)
					}

					fmt.Println()
				}
			}
		}
	},
}

// ============================================================================
// Mount/Unmount Operations
// ============================================================================

var drivesMountCmd = &cobra.Command{
	Use:   "mount <drive> <image> [--type TYPE] [--mode MODE]",
	Short: "Mount disk image from C64U filesystem",
	Long: `Mount a disk image that is already on the C64 Ultimate filesystem.

Drive: 8, 9, 10, 11
Types: d64, g64, d71, g71, d81
Modes: readwrite, readonly, unlinked

Example:
  c64u drives mount 8 /usb0/games.d64 --mode readonly`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]
		image := args[1]
		imageType, _ := cmd.Flags().GetString("type")
		mode, _ := cmd.Flags().GetString("mode")

		resp, err := apiClient.DrivesMount(drive, image, imageType, mode)
		if err != nil {
			formatter.Error("Failed to mount image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"drive": drive,
			"image": filepath.Base(image),
		}
		if mode != "" {
			data["mode"] = mode
		}
		formatter.Success("Disk image mounted", data)
	},
}

var drivesMountUploadCmd = &cobra.Command{
	Use:   "mount-upload <drive> <local-file> [--type TYPE] [--mode MODE]",
	Short: "Upload and mount disk image",
	Long: `Upload a local disk image and mount it to the specified drive.

Drive: 8, 9, 10, 11
Types: d64, g64, d71, g71, d81
Modes: readwrite, readonly, unlinked

Example:
  c64u drives mount-upload 8 game.d64 --mode readonly`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]
		localFile := args[1]
		imageType, _ := cmd.Flags().GetString("type")
		mode, _ := cmd.Flags().GetString("mode")

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.DrivesMountUpload(drive, localFile, imageType, mode)
		if err != nil {
			formatter.Error("Failed to upload and mount image", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"drive": drive,
			"image": filepath.Base(localFile),
		}
		if mode != "" {
			data["mode"] = mode
		}
		formatter.Success("Disk image uploaded and mounted", data)
	},
}

var drivesUnmountCmd = &cobra.Command{
	Use:   "unmount <drive>",
	Short: "Unmount disk from drive",
	Long: `Remove the currently mounted disk image from the specified drive.

Example:
  c64u drives unmount 8`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]

		resp, err := apiClient.DrivesRemove(drive)
		if err != nil {
			formatter.Error("Failed to unmount disk", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Disk unmounted from drive %s", drive), nil)
	},
}

// ============================================================================
// Drive Control
// ============================================================================

var drivesResetCmd = &cobra.Command{
	Use:   "reset <drive>",
	Short: "Reset drive",
	Long: `Reset the specified drive.

Example:
  c64u drives reset 8`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]

		resp, err := apiClient.DrivesReset(drive)
		if err != nil {
			formatter.Error("Failed to reset drive", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Drive %s reset", drive), nil)
	},
}

var drivesOnCmd = &cobra.Command{
	Use:   "on <drive>",
	Short: "Enable drive",
	Long: `Enable the specified drive.

Example:
  c64u drives on 8`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]

		resp, err := apiClient.DrivesOn(drive)
		if err != nil {
			formatter.Error("Failed to enable drive", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Drive %s enabled", drive), nil)
	},
}

var drivesOffCmd = &cobra.Command{
	Use:   "off <drive>",
	Short: "Disable drive",
	Long: `Disable the specified drive.

Example:
  c64u drives off 8`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]

		resp, err := apiClient.DrivesOff(drive)
		if err != nil {
			formatter.Error("Failed to disable drive", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Drive %s disabled", drive), nil)
	},
}

// ============================================================================
// ROM and Mode Operations
// ============================================================================

var drivesLoadROMCmd = &cobra.Command{
	Use:   "load-rom <drive> <file>",
	Short: "Load custom ROM from C64U filesystem",
	Long: `Load a custom drive ROM (16K/32K) temporarily from C64U filesystem.

Example:
  c64u drives load-rom 8 /usb0/speeddos.rom`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]
		file := args[1]

		resp, err := apiClient.DrivesLoadROM(drive, file)
		if err != nil {
			formatter.Error("Failed to load ROM", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"drive": drive,
			"rom":   filepath.Base(file),
		}
		formatter.Success("Custom ROM loaded", data)
	},
}

var drivesLoadROMUploadCmd = &cobra.Command{
	Use:   "load-rom-upload <drive> <local-file>",
	Short: "Upload and load custom ROM",
	Long: `Upload a local custom drive ROM (16K/32K) and load it temporarily.

Example:
  c64u drives load-rom-upload 8 speeddos.rom`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]
		localFile := args[1]

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.DrivesLoadROMUpload(drive, localFile)
		if err != nil {
			formatter.Error("Failed to upload and load ROM", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"drive": drive,
			"rom":   filepath.Base(localFile),
		}
		formatter.Success("Custom ROM uploaded and loaded", data)
	},
}

var drivesSetModeCmd = &cobra.Command{
	Use:   "set-mode <drive> <mode>",
	Short: "Set drive emulation mode",
	Long: `Change the drive emulation mode.

Modes: 1541, 1571, 1581

Example:
  c64u drives set-mode 8 1541`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		drive := args[0]
		mode := args[1]

		// Validate mode
		validModes := map[string]bool{"1541": true, "1571": true, "1581": true}
		if !validModes[mode] {
			formatter.Error("Invalid mode", []string{
				fmt.Sprintf("Mode '%s' is not valid", mode),
				"Valid modes: 1541, 1571, 1581",
			})
			return
		}

		resp, err := apiClient.DrivesSetMode(drive, mode)
		if err != nil {
			formatter.Error("Failed to set drive mode", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		data := map[string]interface{}{
			"drive": drive,
			"mode":  mode,
		}
		formatter.Success("Drive mode changed", data)
	},
}

func init() {
	// Add list command
	drivesCmd.AddCommand(drivesListCmd)

	// Add mount/unmount commands
	drivesCmd.AddCommand(drivesMountCmd)
	drivesCmd.AddCommand(drivesMountUploadCmd)
	drivesCmd.AddCommand(drivesUnmountCmd)

	// Add control commands
	drivesCmd.AddCommand(drivesResetCmd)
	drivesCmd.AddCommand(drivesOnCmd)
	drivesCmd.AddCommand(drivesOffCmd)

	// Add ROM and mode commands
	drivesCmd.AddCommand(drivesLoadROMCmd)
	drivesCmd.AddCommand(drivesLoadROMUploadCmd)
	drivesCmd.AddCommand(drivesSetModeCmd)

	// Add flags for mount commands
	drivesMountCmd.Flags().String("type", "", "Image type (d64, g64, d71, g71, d81)")
	drivesMountCmd.Flags().String("mode", "", "Mount mode (readwrite, readonly, unlinked)")
	drivesMountUploadCmd.Flags().String("type", "", "Image type (d64, g64, d71, g71, d81)")
	drivesMountUploadCmd.Flags().String("mode", "", "Mount mode (readwrite, readonly, unlinked)")
}
