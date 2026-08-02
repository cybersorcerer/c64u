package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/petscii"
	"github.com/spf13/cobra"
)

// machineCmd represents the machine command group
var machineCmd = &cobra.Command{
	Use:   "machine",
	Args:  cobra.NoArgs,
	Short: "Machine control and memory operations",
	Long: `Control the C64 Ultimate machine state and perform memory operations.

Commands include reset, reboot, pause/resume, power control, and direct
memory read/write operations via DMA.

Examples:
  c64u machine reset
  c64u machine reboot`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// ============================================================================
// Machine Control Commands
// ============================================================================

var machineResetCmd = &cobra.Command{
	Use:   "reset",
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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

// parseAddress reads a C64 address written as 0400, $0400 or 0x0400. The device
// accepts nonsense like "zzzz" without complaint and answers with whatever it
// finds, so the check happens here rather than there.
func parseAddress(s string) (int, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "$")
	t = strings.TrimPrefix(strings.TrimPrefix(t, "0x"), "0X")
	if t == "" {
		return 0, fmt.Errorf("address is empty")
	}
	v, err := strconv.ParseUint(t, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a hexadecimal address", s)
	}
	if v > 0xFFFF {
		return 0, fmt.Errorf("$%X is beyond $FFFF", v)
	}
	return int(v), nil
}

// rangeLength turns an inclusive end address into a byte count: --to 07e7 reads
// up to and including $07E7, because that is how a C64 memory map reads.
func rangeLength(start int, end string) (int, error) {
	last, err := parseAddress(end)
	if err != nil {
		return 0, err
	}
	if last < start {
		return 0, fmt.Errorf("end $%04X is below start $%04X", last, start)
	}
	return last - start + 1, nil
}

var machineReadMemCmd = &cobra.Command{
	Use:   "read-mem <address> [--length N | --to ADDR]",
	Short: "Read memory via DMA",
	Long: `Perform a DMA read and show the memory as a hex dump.

How much to read is either a byte count with --length, or an end address with
--to, which is inclusive: --to 07e7 reads up to and including $07E7. Without
either, 256 bytes are read.

By default the memory is shown as a hex dump. To get the bytes themselves,
add --output (-o) to write them to a file, or --raw to write them to standard
output for piping. Either can be combined with --length or --to, and both hand
over the memory exactly as the C64 returned it, with nothing added or removed.

Addresses may be written as 0400, $0400 or 0x0400.

Examples:
  c64u machine read-mem 0400 --length 1000                    # hex dump
  c64u machine read-mem 0400 --to 07e7                        # screen RAM
  c64u machine read-mem 0400 --to 07e7 --output screen.bin    # binary file
  c64u machine read-mem 0000 --to ffff -o memory.bin          # whole address space
  c64u machine read-mem 0400 --length 1000 --raw | xxd        # pipe the bytes
  c64u machine read-mem d020 --length 1`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outPath, _ := cmd.Flags().GetString("output")
		raw, _ := cmd.Flags().GetBool("raw")

		addr, err := parseAddress(args[0])
		if err != nil {
			formatter.Error("Invalid address", []string{err.Error()})
			return
		}

		length, _ := cmd.Flags().GetInt("length")
		if cmd.Flags().Changed("to") {
			end, _ := cmd.Flags().GetString("to")
			length, err = rangeLength(addr, end)
			if err != nil {
				formatter.Error("Invalid range", []string{err.Error()})
				return
			}
		}

		address := fmt.Sprintf("%04x", addr)
		resp, err := apiClient.MachineReadMem(address, length)
		if err != nil {
			formatter.Error("Failed to read memory", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		// --raw writes the bytes and nothing else, so a pipe stays clean.
		if raw {
			if _, err := os.Stdout.Write(resp.RawBody); err != nil {
				formatter.Error("Failed to write memory to stdout", []string{err.Error()})
			}
			return
		}

		if outPath != "" {
			if err := os.WriteFile(outPath, resp.RawBody, 0o644); err != nil {
				formatter.Error("Failed to write memory to file", []string{err.Error()})
				return
			}
			formatter.Success("Memory written", map[string]interface{}{
				"address": "$" + address,
				"file":    outPath,
				"size":    len(resp.RawBody),
			})
			return
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
	Args:  cobra.NoArgs,
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

var machineSendKeyCmd = &cobra.Command{
	Use:   "sendkey <string>",
	Short: "Send keystrokes to the C64 keyboard buffer",
	Long: `Convert a string to PETSCII and inject it into the C64 keyboard buffer via DMA.

Escape sequences:
  \n      Return
  \f1-\f8 Function keys F1-F8
  \clr    CLR/HOME
  \del    DEL
  \stop   RUN/STOP
  \home   HOME

Strings longer than 10 characters are sent in chunks with --delay ms between them.

Examples:
  c64u machine sendkey "A"
  c64u machine sendkey 'LOAD"*",8,1\n'
  c64u machine sendkey '\f1'
  c64u machine sendkey 'HELLO\n' --delay 50`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		delay, _ := cmd.Flags().GetInt("delay")
		input := args[0]

		encoded, err := petscii.Encode(input)
		if err != nil {
			formatter.Error("Invalid input", []string{err.Error()})
			return
		}

		if err := apiClient.SendKeyBytes(encoded, time.Duration(delay)*time.Millisecond); err != nil {
			formatter.Error("Failed to send keystrokes", []string{err.Error()})
			return
		}

		formatter.Success(fmt.Sprintf("Sent %d byte(s) to keyboard buffer", len(encoded)), nil)
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
	machineCmd.AddCommand(machineSendKeyCmd)
	machineSendKeyCmd.Flags().Int("delay", 100, "Delay between chunks in milliseconds")

	// Add memory operation commands
	machineCmd.AddCommand(machineWriteMemCmd)
	machineCmd.AddCommand(machineWriteMemFileCmd)
	machineCmd.AddCommand(machineReadMemCmd)

	// Add debug register commands
	machineCmd.AddCommand(machineDebugRegCmd)
	machineCmd.AddCommand(machineDebugRegSetCmd)

	// Add flags
	machineReadMemCmd.Flags().Int("length", 256, "Number of bytes to read")
	machineReadMemCmd.Flags().String("to", "", "Read up to and including this address, instead of --length")
	machineReadMemCmd.Flags().StringP("output", "o", "", "Write the raw bytes to this file instead of a hex dump")
	machineReadMemCmd.Flags().Bool("raw", false, "Write the raw bytes to stdout instead of a hex dump, for piping")
	machineReadMemCmd.MarkFlagsMutuallyExclusive("length", "to")
	machineReadMemCmd.MarkFlagsMutuallyExclusive("output", "raw")
}
