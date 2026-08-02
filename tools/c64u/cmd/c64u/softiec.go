package main

import (
	"fmt"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/softiec"
	"github.com/spf13/cobra"
)

var softiecCmd = &cobra.Command{
	Use:   "softiec",
	Short: "Control the SoftIEC (DOS emulation) drive",
	Long: `Control the SoftIEC drive, which serves a directory of the C64 Ultimate
filesystem over the IEC bus.

SoftIEC is not a floppy drive. Enabling it and its bus ID are configuration
settings, and the served directory is changed with a CBM DOS command typed on
the C64, so the C64 has to be at the BASIC prompt for 'softiec root'.

Examples:
  c64u drives softiec status
  c64u drives softiec enable --bus-id 11
  c64u drives softiec root /Usb0/development`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help() //nolint:errcheck
	},
}

var softiecStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SoftIEC state, bus ID and served directory",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		status, err := softiec.ReadStatus(apiClient)
		if err != nil {
			formatter.Error("Failed to read SoftIEC status", []string{err.Error()})
			return
		}
		if !status.Present {
			formatter.Error("This device has no SoftIEC drive", nil)
			return
		}

		state := "disabled"
		if status.Enabled {
			state = "enabled"
		}
		formatter.Success(fmt.Sprintf("SoftIEC %s", state), map[string]interface{}{
			"bus_id":    status.BusID,
			"directory": status.Path,
			"status":    status.LastError,
		})
	},
}

var softiecEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable SoftIEC, optionally on a different bus ID",
	Long: `Enable the SoftIEC drive.

The bus ID is the IEC device number the drive answers on, so LOAD"$",<id>
reaches it. Set it with --bus-id; the valid range is read from the device.

Examples:
  c64u drives softiec enable
  c64u drives softiec enable --bus-id 11`,
	Args: cobra.NoArgs,
	Run:  func(cmd *cobra.Command, args []string) { runSoftIECEnable(cmd, true) },
}

var softiecDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable SoftIEC",
	Args:  cobra.NoArgs,
	Run:   func(cmd *cobra.Command, args []string) { runSoftIECEnable(cmd, false) },
}

func runSoftIECEnable(cmd *cobra.Command, on bool) {
	settings, err := softiec.LoadSettings(apiClient)
	if err != nil {
		formatter.Error("Failed to read SoftIEC settings", []string{err.Error()})
		return
	}

	details := map[string]interface{}{}
	if cmd.Flags().Changed("bus-id") {
		busID, _ := cmd.Flags().GetInt("bus-id")
		if err := softiec.SetBusID(apiClient, settings, busID); err != nil {
			formatter.Error("Failed to set bus ID", []string{err.Error()})
			return
		}
		details["bus_id"] = busID
	}

	if err := softiec.SetEnabled(apiClient, settings, on); err != nil {
		formatter.Error("Failed to change SoftIEC", []string{err.Error()})
		return
	}

	state := "disabled"
	if on {
		state = "enabled"
	}
	formatter.Success("SoftIEC "+state, details)
}

var softiecBusIDCmd = &cobra.Command{
	Use:   "bus-id <id>",
	Short: "Set the IEC device number SoftIEC answers on",
	Long: `Set the IEC device number SoftIEC answers on, without changing whether it
is enabled. The valid range is read from the device.

Example:
  c64u drives softiec bus-id 11`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var busID int
		if _, err := fmt.Sscanf(args[0], "%d", &busID); err != nil {
			formatter.Error("Invalid bus ID", []string{fmt.Sprintf("%q is not a number", args[0])})
			return
		}

		settings, err := softiec.LoadSettings(apiClient)
		if err != nil {
			formatter.Error("Failed to read SoftIEC settings", []string{err.Error()})
			return
		}
		if err := softiec.SetBusID(apiClient, settings, busID); err != nil {
			formatter.Error("Failed to set bus ID", []string{err.Error()})
			return
		}
		formatter.Success(fmt.Sprintf("SoftIEC bus ID set to %d", busID), nil)
	},
}

var softiecRootCmd = &cobra.Command{
	Use:   "root <path>",
	Short: "Point SoftIEC at a directory on the device",
	Long: `Point SoftIEC at a directory of the C64 Ultimate filesystem.

There is no API endpoint for this. The command is typed on the C64 keyboard as
a CBM DOS "CD:" line, so the C64 must be sitting at the BASIC prompt — if a
program is running the line goes nowhere. The drive is read back afterwards and
this reports what it actually serves.

Example:
  c64u drives softiec root /Usb0/development`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		status, err := softiec.ReadStatus(apiClient)
		if err != nil {
			formatter.Error("Failed to read SoftIEC status", []string{err.Error()})
			return
		}
		if !status.Enabled {
			formatter.Error("SoftIEC is disabled", []string{"Enable it first: c64u drives softiec enable"})
			return
		}

		result, err := softiec.SetRoot(apiClient, status.BusID, args[0])
		if err != nil {
			formatter.Error("Failed to set SoftIEC directory", []string{err.Error()})
			return
		}
		formatter.Success("SoftIEC now serves "+result.Path, nil)
	},
}

func init() {
	drivesCmd.AddCommand(softiecCmd)
	softiecCmd.AddCommand(softiecStatusCmd)
	softiecCmd.AddCommand(softiecEnableCmd)
	softiecCmd.AddCommand(softiecDisableCmd)
	softiecCmd.AddCommand(softiecBusIDCmd)
	softiecCmd.AddCommand(softiecRootCmd)

	softiecEnableCmd.Flags().Int("bus-id", 0, "IEC device number to answer on")
}
