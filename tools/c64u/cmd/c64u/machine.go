package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/spf13/cobra"
)

// machineCmd represents the machine command group
var machineCmd = &cobra.Command{
	Use:   "machine",
	Short: "Machine control and memory operations",
	Long: `Control the C64 Ultimate machine state and perform memory operations.

Commands include reset, reboot, pause/resume, power control, and direct
memory read/write operations via DMA.`,
}

// ============================================================================
// Machine Control Commands
// ============================================================================

var machineResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the machine",
	Long:  `Send a reset signal to the machine without changing configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachineReset()
		if err != nil {
			formatter.Error("Failed to reset machine", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Machine reset successfully", nil)
	},
}

var machineRebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot the machine",
	Long:  `Restart the machine with cartridge reinitialization.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachineReboot()
		if err != nil {
			formatter.Error("Failed to reboot machine", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Machine rebooted successfully", nil)
	},
}

var machinePauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the machine",
	Long:  `Pause the machine by pulling the DMA line low.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachinePause()
		if err != nil {
			formatter.Error("Failed to pause machine", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Machine paused", nil)
	},
}

var machineResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the machine",
	Long:  `Resume the machine from paused state.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachineResume()
		if err != nil {
			formatter.Error("Failed to resume machine", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Machine resumed", nil)
	},
}

var machinePowerOffCmd = &cobra.Command{
	Use:   "poweroff",
	Short: "Power off the machine (U64 only)",
	Long:  `Power off the machine. This command only works on Ultimate 64 hardware.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachinePowerOff()
		if err != nil {
			formatter.Error("Failed to power off machine", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Machine powered off", nil)
	},
}

var machineMenuButtonCmd = &cobra.Command{
	Use:   "menu-button",
	Short: "Simulate pressing the Menu button",
	Long: `Simulate pressing the Menu button.

On 1541 Ultimate cartridge: Activates the Menu button
On Ultimate 64: Brief press of the Multi Button`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachineMenuButton()
		if err != nil {
			formatter.Error("Failed to activate menu button", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success("Menu button activated", nil)
	},
}

// ============================================================================
// Memory Operations
// ============================================================================

var machineWriteMemCmd = &cobra.Command{
	Use:   "write-mem <address> <data>",
	Short: "Write data to memory",
	Long: `Write up to 128 bytes via DMA to specified hex address.

Examples:
  c64u machine write-mem 0400 01020304    # Write hex bytes to screen memory
  c64u machine write-mem d020 00          # Change border color to black`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		data := args[1]

		resp, err := apiClient.MachineWriteMem(address, data)
		if err != nil {
			formatter.Error("Failed to write memory", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Wrote data to address $%s", address), nil)
	},
}

var machineWriteMemFileCmd = &cobra.Command{
	Use:   "write-mem-file <address> <file>",
	Short: "Write file contents to memory",
	Long: `Write binary file contents to specified hex address via DMA.

Example:
  c64u machine write-mem-file 0400 screen.bin  # Load screen data`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		filePath := args[1]

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			formatter.Error("File not found", []string{filePath})
			return
		}

		resp, err := apiClient.MachineWriteMemFile(address, filePath)
		if err != nil {
			formatter.Error("Failed to write memory from file", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		fileInfo, _ := os.Stat(filePath)
		data := map[string]interface{}{
			"address": "$" + address,
			"file":    filePath,
			"size":    fileInfo.Size(),
		}
		formatter.Success("Wrote file to memory", data)
	},
}

var machineReadMemCmd = &cobra.Command{
	Use:   "read-mem <address> [--length N]",
	Short: "Read memory via DMA",
	Long: `Perform DMA read operation and return binary data.

The output can be redirected to a file or viewed as hex dump.

Examples:
  c64u machine read-mem 0400 --length 1000 > screen.bin
  c64u machine read-mem d020 --length 1`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		length, _ := cmd.Flags().GetInt("length")

		resp, err := apiClient.MachineReadMem(address, length)
		if err != nil {
			formatter.Error("Failed to read memory", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		// Parse address for hex dump
		addr, err := strconv.ParseInt(address, 16, 64)
		if err != nil {
			addr = 0
		}

		// Display as hex dump in text mode, raw bytes in JSON mode
		if jsonOut {
			formatter.PrintData(map[string]interface{}{
				"address": "$" + address,
				"length":  len(resp.RawBody),
				"data":    fmt.Sprintf("%x", resp.RawBody),
			})
		} else {
			formatter.PrintHeader(fmt.Sprintf("Memory dump from $%s (%d bytes)", address, len(resp.RawBody)))
			fmt.Println()
			fmt.Print(api.FormatMemoryDump(resp.RawBody, int(addr)))
		}
	},
}

// ============================================================================
// Debug Register (U64 only)
// ============================================================================

var machineDebugRegCmd = &cobra.Command{
	Use:   "debug-reg",
	Short: "Read debug register $D7FF (U64 only)",
	Long:  `Read the debug register at $D7FF. This command only works on Ultimate 64 hardware.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.MachineDebugReg()
		if err != nil {
			formatter.Error("Failed to read debug register", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.PrintResponse(resp, "Debug register read")
	},
}

var machineDebugRegSetCmd = &cobra.Command{
	Use:   "debug-reg-set <value>",
	Short: "Write to debug register $D7FF (U64 only)",
	Long: `Write a value to the debug register at $D7FF. This command only works on Ultimate 64 hardware.

Example:
  c64u machine debug-reg-set FF`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		value := args[0]

		resp, err := apiClient.MachineDebugRegSet(value)
		if err != nil {
			formatter.Error("Failed to write debug register", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		formatter.Success(fmt.Sprintf("Debug register set to $%s", value), nil)
	},
}

func init() {
	// Add control commands
	machineCmd.AddCommand(machineResetCmd)
	machineCmd.AddCommand(machineRebootCmd)
	machineCmd.AddCommand(machinePauseCmd)
	machineCmd.AddCommand(machineResumeCmd)
	machineCmd.AddCommand(machinePowerOffCmd)
	machineCmd.AddCommand(machineMenuButtonCmd)

	// Add memory operation commands
	machineCmd.AddCommand(machineWriteMemCmd)
	machineCmd.AddCommand(machineWriteMemFileCmd)
	machineCmd.AddCommand(machineReadMemCmd)

	// Add debug register commands
	machineCmd.AddCommand(machineDebugRegCmd)
	machineCmd.AddCommand(machineDebugRegSetCmd)

	// Add flags
	machineReadMemCmd.Flags().Int("length", 256, "Number of bytes to read")
}
