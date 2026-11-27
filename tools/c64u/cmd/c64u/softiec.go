package main

import (
	"fmt"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/softiec"
	"github.com/spf13/cobra"
)

var softiecCmd = &cobra.Command{
	Use:   "softiec",
	Args:  cobra.NoArgs,
	Short: "Control the SoftIEC (DOS emulation) drive",
	Long: `Control the SoftIEC drive, which serves a directory of the C64 Ultimate
filesystem over the IEC bus.

This is the alternative to disk images. With SoftIEC enabled, the C64 loads
straight from the Ultimate filesystem — no .d64 to create, fill and mount:

  LOAD"$",11 : LIST      directory of the served path
  LOAD"NAME",11,1        load a program, honouring its load address

With a JiffyDOS or CMD wedge in the C64, the short forms work too and, unlike
LOAD"$", listing does not overwrite the BASIC program in memory:

  @#11                   direct the wedge at SoftIEC
  @$                     list the directory
  @CD:SUBDIR             change directory
  /NAME                  load

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
	Long: `Report what SoftIEC is currently doing.

The directory and the drive status come from the device itself rather than from
the configuration, so they show what the drive actually serves — the configured
default path is only applied when the drive starts.

Example:
  c64u drives softiec status`,
	Args: cobra.NoArgs,
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
	Long: `Disable the SoftIEC drive, so it stops answering on the IEC bus.

On firmware 1.2.1 and newer the drive stays usable through the Command
Interface ("UCI Only"); --completely switches it off there as well. Older
firmware has only the complete switch-off, so both forms do the same there.

This writes the device configuration; the drives endpoint has no effect on
SoftIEC. The bus ID and directory are left as they are.

Examples:
  c64u drives softiec disable
  c64u drives softiec disable --completely`,
	Args: cobra.NoArgs,
	Run:  func(cmd *cobra.Command, args []string) { runSoftIECEnable(cmd, false) },
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

	completely, _ := cmd.Flags().GetBool("completely")
	if completely {
		err = softiec.DisableCompletely(apiClient, settings)
	} else {
		err = softiec.SetEnabled(apiClient, settings, on)
	}
	if err != nil {
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

Firmware 1.2 and newer confine SoftIEC to its partitions, and out of the box the
only one is the RAM disk (/Temp/). A directory outside the partitions cannot be
reached this way; add a partition for it with 'c64u drives softiec partitions set'.

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

// partitionsTakeEffect is said wherever the partition file changes.
const partitionsTakeEffect = "Takes effect when the C64 Ultimate is switched off and on again; " +
	"c64u machine reboot is not enough"

var softiecPartitionsCmd = &cobra.Command{
	Use:   "partitions",
	Short: "Show SoftIEC partitions, active now and saved for the next power-on",
	Long: `Show the SoftIEC partitions: the ones the drive serves now, and the ones saved
in ` + softiec.PartitionFile + ` for the next power-on.

The device only reads that file when it is switched on, so the two lists differ
after 'partitions set' or 'partitions clear' until it has been switched off and
on again. 'c64u machine reboot' restarts only the C64 and does not read it.

The drive reports partition names only through the file, so the active list
shows numbers and paths.

Examples:
  c64u drives softiec partitions
  c64u drives softiec partitions set 1=RAMDISK:/Temp 2=USB:/USB0/
  c64u drives softiec partitions clear`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		active, err := softiec.ActivePartitions(apiClient)
		if err != nil {
			formatter.Error("Failed to read SoftIEC partitions", []string{err.Error()})
			return
		}
		saved, ok, err := softiec.SavedPartitions(apiClient)
		if err != nil {
			formatter.Error("Failed to read "+softiec.PartitionFile, []string{err.Error()})
			return
		}
		data := map[string]interface{}{"active": partitionRows(active)}
		if ok {
			data["saved"] = partitionRows(saved)
		} else {
			data["saved"] = "none - the device starts with 1 = /Temp"
		}
		formatter.Success("SoftIEC partitions", data)
	},
}

// partitionRows lists partitions for the formatter; a name only when there is one.
func partitionRows(parts []softiec.Partition) []interface{} {
	rows := []interface{}{}
	for _, p := range parts {
		row := map[string]interface{}{"number": p.Number, "path": p.Path}
		if p.Name != "" {
			row["name"] = p.Name
		}
		rows = append(rows, row)
	}
	return rows
}

var softiecPartitionsSetCmd = &cobra.Command{
	Use:   "set N=[NAME:]PATH...",
	Short: "Save the SoftIEC partitions for the next power-on",
	Long: `Save the SoftIEC partitions the device sets up when it is switched on.

Each argument is one partition: its number (1-255), an optional name of up to
16 characters, and an absolute path to a directory on the device. The paths are
checked before anything is written.

The list replaces the device's own: partition 1 (the RAM disk on /Temp) is gone
unless you name it, and the lowest number becomes the current partition, the
one LOAD"$",11 lists.

` + partitionsTakeEffect + `.

Examples:
  c64u drives softiec partitions set 1=RAMDISK:/Temp 2=USB:/USB0/
  c64u drives softiec partitions set 2=/USB0/games`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var parts []softiec.Partition
		for _, arg := range args {
			p, err := softiec.ParsePartition(arg)
			if err != nil {
				formatter.Error("Invalid partition", []string{err.Error()})
				return
			}
			parts = append(parts, p)
		}
		if err := softiec.SavePartitions(apiClient, parts); err != nil {
			formatter.Error("Failed to save SoftIEC partitions", []string{err.Error()})
			return
		}
		saved, _, _ := softiec.SavedPartitions(apiClient)
		formatter.Success("SoftIEC partitions saved", map[string]interface{}{
			"saved": partitionRows(saved),
			"note":  partitionsTakeEffect,
		})
	},
}

var softiecPartitionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete the saved SoftIEC partitions",
	Long: `Delete ` + softiec.PartitionFile + `, so the device starts with its own single
partition, 1 = /Temp.

` + partitionsTakeEffect + `.

Example:
  c64u drives softiec partitions clear`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		existed, err := softiec.ClearPartitions(apiClient)
		if err != nil {
			formatter.Error("Failed to delete "+softiec.PartitionFile, []string{err.Error()})
			return
		}
		if !existed {
			formatter.Success("No SoftIEC partitions were saved", nil)
			return
		}
		formatter.Success("SoftIEC partitions deleted", map[string]interface{}{"note": partitionsTakeEffect})
	},
}

func init() {
	drivesCmd.AddCommand(softiecCmd)
	softiecCmd.AddCommand(softiecStatusCmd)
	softiecCmd.AddCommand(softiecEnableCmd)
	softiecCmd.AddCommand(softiecDisableCmd)
	softiecCmd.AddCommand(softiecBusIDCmd)
	softiecCmd.AddCommand(softiecRootCmd)
	softiecCmd.AddCommand(softiecPartitionsCmd)
	softiecPartitionsCmd.AddCommand(softiecPartitionsSetCmd)
	softiecPartitionsCmd.AddCommand(softiecPartitionsClearCmd)

	softiecEnableCmd.Flags().Int("bus-id", 0, "IEC device number to answer on")
	softiecDisableCmd.Flags().Bool("completely", false, "Also switch it off for the Command Interface")
}
