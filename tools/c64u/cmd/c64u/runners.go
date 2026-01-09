package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// runnersCmd represents the runners command group
var runnersCmd = &cobra.Command{
	Use:   "runners",
	Short: "Media playback and program execution",
	Long: `The runners commands allow you to play media files (SID, MOD) and
execute programs (PRG, CRT) on the C64 Ultimate.

Each command has two variants:
- Without 'upload': Uses a file already on the C64U filesystem
- With 'upload': Uploads a local file and then executes it`,
}

// ============================================================================
// SID Playback Commands
// ============================================================================

var sidPlayCmd = &cobra.Command{
	Use:   "sidplay <file> [--song N]",
	Short: "Play SID file from C64U filesystem",
	Long:  `Play a SID file that is already stored on the C64 Ultimate filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		songNr, _ := cmd.Flags().GetInt("song")

		resp, err := apiClient.SidPlay(file, songNr)
		if err != nil {
			formatter.Error("Failed to play SID file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		msg := fmt.Sprintf("Playing SID file: %s", filepath.Base(file))
		if songNr > 0 {
			msg += fmt.Sprintf(" (song %d)", songNr)
		}
		formatter.Success(msg, nil)
	},
}

var sidPlayUploadCmd = &cobra.Command{
	Use:   "sidplay-upload <local-file> [--song N]",
	Short: "Upload and play SID file",
	Long:  `Upload a local SID file to the C64 Ultimate and play it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]
		songNr, _ := cmd.Flags().GetInt("song")

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.SidPlayUpload(localFile, songNr)
		if err != nil {
			formatter.Error("Failed to upload and play SID file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		msg := fmt.Sprintf("Uploaded and playing: %s", filepath.Base(localFile))
		if songNr > 0 {
			msg += fmt.Sprintf(" (song %d)", songNr)
		}
		formatter.Success(msg, nil)
	},
}

// ============================================================================
// MOD Playback Commands
// ============================================================================

var modPlayCmd = &cobra.Command{
	Use:   "modplay <file>",
	Short: "Play MOD file from C64U filesystem",
	Long:  `Play an Amiga MOD file that is already stored on the C64 Ultimate filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		resp, err := apiClient.ModPlay(file)
		if err != nil {
			formatter.Error("Failed to play MOD file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Playing MOD file: %s", filepath.Base(file)), nil)
	},
}

var modPlayUploadCmd = &cobra.Command{
	Use:   "modplay-upload <local-file>",
	Short: "Upload and play MOD file",
	Long:  `Upload a local Amiga MOD file to the C64 Ultimate and play it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.ModPlayUpload(localFile)
		if err != nil {
			formatter.Error("Failed to upload and play MOD file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Uploaded and playing: %s", filepath.Base(localFile)), nil)
	},
}

// ============================================================================
// PRG Commands (Load without execution)
// ============================================================================

var loadPrgCmd = &cobra.Command{
	Use:   "load-prg <file>",
	Short: "Load PRG file from C64U filesystem (no execution)",
	Long:  `Load a program into memory via DMA without executing it. The file must already be on the C64 Ultimate filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		resp, err := apiClient.LoadPRG(file)
		if err != nil {
			formatter.Error("Failed to load PRG file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Loaded PRG file: %s", filepath.Base(file)), nil)
	},
}

var loadPrgUploadCmd = &cobra.Command{
	Use:   "load-prg-upload <local-file>",
	Short: "Upload and load PRG file (no execution)",
	Long:  `Upload a local program file and load it into memory via DMA without executing it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.LoadPRGUpload(localFile)
		if err != nil {
			formatter.Error("Failed to upload and load PRG file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Uploaded and loaded: %s", filepath.Base(localFile)), nil)
	},
}

// ============================================================================
// PRG Commands (Load and Run)
// ============================================================================

var runPrgCmd = &cobra.Command{
	Use:   "run-prg <file>",
	Short: "Load and run PRG file from C64U filesystem",
	Long:  `Load a program into memory and automatically execute it. The file must already be on the C64 Ultimate filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		resp, err := apiClient.RunPRG(file)
		if err != nil {
			formatter.Error("Failed to run PRG file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Running PRG file: %s", filepath.Base(file)), nil)
	},
}

var runPrgUploadCmd = &cobra.Command{
	Use:   "run-prg-upload <local-file>",
	Short: "Upload and run PRG file",
	Long:  `Upload a local program file, load it into memory, and automatically execute it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.RunPRGUpload(localFile)
		if err != nil {
			formatter.Error("Failed to upload and run PRG file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Uploaded and running: %s", filepath.Base(localFile)), nil)
	},
}

// ============================================================================
// CRT Commands (Cartridge)
// ============================================================================

var runCrtCmd = &cobra.Command{
	Use:   "run-crt <file>",
	Short: "Start cartridge file from C64U filesystem",
	Long:  `Start a cartridge file with reset. The file must already be on the C64 Ultimate filesystem.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]

		resp, err := apiClient.RunCRT(file)
		if err != nil {
			formatter.Error("Failed to start cartridge", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Starting cartridge: %s", filepath.Base(file)), nil)
	},
}

var runCrtUploadCmd = &cobra.Command{
	Use:   "run-crt-upload <local-file>",
	Short: "Upload and start cartridge file",
	Long:  `Upload a local cartridge file and start it with reset.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		localFile := args[0]

		// Check if file exists
		if _, err := os.Stat(localFile); os.IsNotExist(err) {
			formatter.Error("File not found", []string{localFile})
			return
		}

		resp, err := apiClient.RunCRTUpload(localFile)
		if err != nil {
			formatter.Error("Failed to upload and start cartridge", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Uploaded and starting: %s", filepath.Base(localFile)), nil)
	},
}

func init() {
	// Add --song flag for SID commands
	sidPlayCmd.Flags().Int("song", 0, "Song number to play (default: 0)")
	sidPlayUploadCmd.Flags().Int("song", 0, "Song number to play (default: 0)")

	// Add all SID commands
	runnersCmd.AddCommand(sidPlayCmd)
	runnersCmd.AddCommand(sidPlayUploadCmd)

	// Add all MOD commands
	runnersCmd.AddCommand(modPlayCmd)
	runnersCmd.AddCommand(modPlayUploadCmd)

	// Add all PRG commands (load without execution)
	runnersCmd.AddCommand(loadPrgCmd)
	runnersCmd.AddCommand(loadPrgUploadCmd)

	// Add all PRG commands (load and run)
	runnersCmd.AddCommand(runPrgCmd)
	runnersCmd.AddCommand(runPrgUploadCmd)

	// Add all CRT commands
	runnersCmd.AddCommand(runCrtCmd)
	runnersCmd.AddCommand(runCrtUploadCmd)
}
