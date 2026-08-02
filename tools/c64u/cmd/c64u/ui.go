package main

import (
	"fmt"
	"os"
	"os/exec"

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

		for {
			model := tui.NewMainModel(apiClient, host)
			if err := model.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				os.Exit(1)
			}

			pending := model.PendingStream
			if pending == "" {
				break
			}

			// Run blocking stream in foreground, then restart TUI
			var streamArgs []string
			switch pending {
			case "debug":
				streamArgs = []string{"streams", "listen", "debug", "--tui", "--host", host}
			default:
				streamArgs = []string{"streams", "listen", pending, "--host", host}
			}

			cmd := exec.Command(os.Args[0], streamArgs...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run() //nolint:errcheck
			// After stream exits, loop restarts TUI
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
