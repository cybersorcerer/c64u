package api

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

// Runners API - Media playback and program execution

// SidPlay plays a SID file from the C64U filesystem
func (c *Client) SidPlay(file string, songNr int) (*Response, error) {
	params := map[string]string{
		"file": file,
	}
	if songNr > 0 {
		params["songnr"] = strconv.Itoa(songNr)
	}

	return c.Put("/v1/runners:sidplay", params)
}

// SidPlayUpload uploads and plays a SID file
func (c *Client) SidPlayUpload(localFile string, songNr int) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := make(map[string]string)
	if songNr > 0 {
		params["songnr"] = strconv.Itoa(songNr)
	}

	return c.Post("/v1/runners:sidplay", file, params)
}

// ModPlay plays a MOD file from the C64U filesystem
func (c *Client) ModPlay(file string) (*Response, error) {
	params := map[string]string{
		"file": file,
	}

	return c.Put("/v1/runners:modplay", params)
}

// ModPlayUpload uploads and plays a MOD file
func (c *Client) ModPlayUpload(localFile string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return c.Post("/v1/runners:modplay", file, nil)
}

// LoadPRG loads a program into memory via DMA (without execution)
func (c *Client) LoadPRG(file string) (*Response, error) {
	params := map[string]string{
		"file": file,
	}

	return c.Put("/v1/runners:load_prg", params)
}

// LoadPRGUpload uploads and loads a program via DMA (without execution)
func (c *Client) LoadPRGUpload(localFile string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return c.Post("/v1/runners:load_prg", file, nil)
}

// RunPRG loads and automatically executes a program
func (c *Client) RunPRG(file string) (*Response, error) {
	params := map[string]string{
		"file": file,
	}

	return c.Put("/v1/runners:run_prg", params)
}

// RunPRGUpload uploads, loads and executes a program
func (c *Client) RunPRGUpload(localFile string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return c.Post("/v1/runners:run_prg", file, nil)
}

// RunCRT starts a cartridge file with reset
func (c *Client) RunCRT(file string) (*Response, error) {
	params := map[string]string{
		"file": file,
	}

	return c.Put("/v1/runners:run_crt", params)
}

// RunCRTUpload uploads and starts a cartridge file
func (c *Client) RunCRTUpload(localFile string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return c.Post("/v1/runners:run_crt", file, nil)
}

// Helper function to read file into reader
func OpenFileReader(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
