package main

import (
	"errors"
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

		// Carries a failed stream's error into the status line of the TUI that
		// is started afterwards; the stream writes to a screen the TUI wipes.
		var status string
		// The tab to return to after a stream, so leaving a stream viewer does
		// not dump the user back on the first tab.
		tab := 0

		for {
			model := tui.NewMainModel(apiClient, host)
			model.SetTab(tab)
			model.InitialStatus = status
			status = ""

			if err := model.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				os.Exit(1)
			}

			tab = model.ActiveTab()
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

			streamCmd := exec.Command(os.Args[0], streamArgs...)
			streamCmd.Stdin = os.Stdin
			streamCmd.Stdout = os.Stdout
			streamCmd.Stderr = os.Stderr
			if err := streamCmd.Run(); err != nil && !endedBySignal(err) {
				status = fmt.Sprintf("Error: %s stream ended: %v", pending, err)
			}
			// After stream exits, loop restarts TUI
		}
	},
}

// endedBySignal reports whether a child process was terminated by a signal,
// which is how a user normally leaves a stream viewer (Ctrl+C) and therefore
// not worth reporting as an error.
func endedBySignal(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && !exitErr.Exited()
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
