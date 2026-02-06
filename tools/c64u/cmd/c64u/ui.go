package main

import (
	"fmt"
	"os"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/tui"
	"github.com/spf13/cobra"
)

// uiCmd represents the ui command
var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start interactive TUI mode",
	Long: `Start the interactive Terminal User Interface (TUI) for controlling the C64 Ultimate.

This mode provides a visual interface for:
- Browsing and mounting files
- Machine control (Reset, Reboot, Pause, etc.)
- viewing device status`,
	Run: func(cmd *cobra.Command, args []string) {
		// Ensure we have an API client (initialized in persistentPreRun)
		if apiClient == nil {
			formatter.Error("API client not initialized", nil)
			return
		}

		// Initialize TUI model
		model := tui.NewMainModel(apiClient)

		// Run Bubble Tea program
		if err := model.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
