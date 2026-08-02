package api

import (
	"fmt"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/petscii"
)

// The C64 keyboard buffer and its fill count, written via DMA. The buffer holds
// ten keystrokes, so longer input has to be handed over in chunks that the
// running KERNAL has time to drain.
const (
	kbBufferAddr = "0277"
	kbLengthAddr = "00C6"
	kbBufferSize = 10
)

// SendKeys converts text to PETSCII and types it on the C64.
func (c *Client) SendKeys(text string, delay time.Duration) error {
	encoded, err := petscii.Encode(text)
	if err != nil {
		return err
	}
	return c.SendKeyBytes(encoded, delay)
}

// SendKeyBytes types already encoded PETSCII bytes on the C64.
func (c *Client) SendKeyBytes(data []byte, delay time.Duration) error {
	for start := 0; start < len(data); start += kbBufferSize {
		end := min(start+kbBufferSize, len(data))
		if err := c.writeKeyChunk(data[start:end]); err != nil {
			return err
		}
		if end < len(data) {
			time.Sleep(delay)
		}
	}
	return nil
}

func (c *Client) writeKeyChunk(chunk []byte) error {
	if err := c.writeMem(kbBufferAddr, fmt.Sprintf("%X", chunk)); err != nil {
		return fmt.Errorf("keyboard buffer: %w", err)
	}
	if err := c.writeMem(kbLengthAddr, fmt.Sprintf("%02X", len(chunk))); err != nil {
		return fmt.Errorf("keyboard buffer length: %w", err)
	}
	return nil
}

func (c *Client) writeMem(address, data string) error {
	resp, err := c.MachineWriteMem(address, data)
	if err != nil {
		return err
	}
	if resp.HasErrors() {
		return fmt.Errorf("%s", resp.Errors[0])
	}
	return nil
}
