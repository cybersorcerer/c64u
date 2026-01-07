package api

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
)

// Machine Control API - System control and memory operations

// MachineReset sends a reset without changing configuration
func (c *Client) MachineReset() (*Response, error) {
	return c.Put("/v1/machine:reset", nil)
}

// MachineReboot restarts machine with cartridge reinitialization
func (c *Client) MachineReboot() (*Response, error) {
	return c.Put("/v1/machine:reboot", nil)
}

// MachinePause pauses machine by pulling DMA line low
func (c *Client) MachinePause() (*Response, error) {
	return c.Put("/v1/machine:pause", nil)
}

// MachineResume resumes from paused state
func (c *Client) MachineResume() (*Response, error) {
	return c.Put("/v1/machine:resume", nil)
}

// MachinePowerOff powers off (U64-only)
func (c *Client) MachinePowerOff() (*Response, error) {
	return c.Put("/v1/machine:poweroff", nil)
}

// MachineWriteMem writes up to 128 bytes via DMA to specified hex address
// address: hex address (e.g., "0400")
// data: hex data string (e.g., "01020304")
func (c *Client) MachineWriteMem(address string, data string) (*Response, error) {
	params := map[string]string{
		"address": address,
		"data":    data,
	}

	return c.Put("/v1/machine:writemem", params)
}

// MachineWriteMemFile writes binary file data to hex address
// address: hex address (e.g., "0400")
// filePath: path to binary file to upload
func (c *Client) MachineWriteMemFile(address string, filePath string) (*Response, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := map[string]string{
		"address": address,
	}

	return c.Post("/v1/machine:writemem", file, params)
}

// MachineReadMem performs DMA read action returning binary data
// address: hex address (e.g., "0400")
// length: number of bytes to read (optional, default from API)
func (c *Client) MachineReadMem(address string, length int) (*Response, error) {
	params := map[string]string{
		"address": address,
	}

	if length > 0 {
		params["length"] = strconv.Itoa(length)
	}

	return c.Get("/v1/machine:readmem", params)
}

// MachineDebugReg reads debug register $D7FF (U64-only)
func (c *Client) MachineDebugReg() (*Response, error) {
	return c.Get("/v1/machine:debugreg", nil)
}

// MachineDebugRegSet writes to debug register $D7FF (U64-only)
// value: hex value to write
func (c *Client) MachineDebugRegSet(value string) (*Response, error) {
	params := map[string]string{
		"value": value,
	}

	return c.Put("/v1/machine:debugreg", params)
}

// MachineMenuButton simulates pressing the Menu button
// On 1541 Ultimate cartridge, this is the Menu button
// On Ultimate 64, this is a brief press of the Multi Button
func (c *Client) MachineMenuButton() (*Response, error) {
	return c.Put("/v1/machine:menu_button", nil)
}

// GetInfo returns device information including product name, firmware versions, and hostname
func (c *Client) GetInfo() (*Response, error) {
	return c.Get("/v1/info", nil)
}

// Helper function to convert hex string to bytes
func hexToBytes(hexStr string) ([]byte, error) {
	// Remove "0x" prefix if present
	if len(hexStr) > 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	// Ensure even length (pad with leading zero if needed)
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}

	return hex.DecodeString(hexStr)
}

// Helper function to convert bytes to hex string
func bytesToHex(data []byte) string {
	return hex.EncodeToString(data)
}

// FormatMemoryDump formats binary memory data as hex dump
func FormatMemoryDump(data []byte, startAddr int) string {
	var buf bytes.Buffer

	for i := 0; i < len(data); i += 16 {
		// Address
		buf.WriteString(fmt.Sprintf("%04X: ", startAddr+i))

		// Hex bytes
		for j := 0; j < 16; j++ {
			if i+j < len(data) {
				buf.WriteString(fmt.Sprintf("%02X ", data[i+j]))
			} else {
				buf.WriteString("   ")
			}
			if j == 7 {
				buf.WriteString(" ")
			}
		}

		// ASCII representation
		buf.WriteString(" |")
		for j := 0; j < 16 && i+j < len(data); j++ {
			b := data[i+j]
			if b >= 32 && b <= 126 {
				buf.WriteByte(b)
			} else {
				buf.WriteByte('.')
			}
		}
		buf.WriteString("|\n")
	}

	return buf.String()
}
