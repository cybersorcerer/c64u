package api

import (
	"fmt"
	"os"
)

// Floppy Drive Operations API

// DrivesList returns info on all internal drives including mounted images
func (c *Client) DrivesList() (*Response, error) {
	return c.Get("/v1/drives", nil)
}

// DrivesMount mounts a disk image
// drive: drive number (e.g., "8", "9")
// image: path to image file on C64U filesystem
// imageType: d64, g64, d71, g71, d81 (optional)
// mode: readwrite, readonly, unlinked (optional)
func (c *Client) DrivesMount(drive, image, imageType, mode string) (*Response, error) {
	params := map[string]string{
		"image": image,
	}

	if imageType != "" {
		params["type"] = imageType
	}

	if mode != "" {
		params["mode"] = mode
	}

	endpoint := fmt.Sprintf("/v1/drives/%s:mount", drive)
	return c.Put(endpoint, params)
}

// DrivesMountUpload uploads and mounts a disk image
// drive: drive number (e.g., "8", "9")
// localFile: path to local image file
// imageType: d64, g64, d71, g71, d81 (optional)
// mode: readwrite, readonly, unlinked (optional)
func (c *Client) DrivesMountUpload(drive, localFile, imageType, mode string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	params := make(map[string]string)

	if imageType != "" {
		params["type"] = imageType
	}

	if mode != "" {
		params["mode"] = mode
	}

	endpoint := fmt.Sprintf("/v1/drives/%s:mount", drive)
	return c.Post(endpoint, file, params)
}

// DrivesReset resets selected drive
func (c *Client) DrivesReset(drive string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/drives/%s:reset", drive)
	return c.Put(endpoint, nil)
}

// DrivesRemove unmounts disk from drive
func (c *Client) DrivesRemove(drive string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/drives/%s:remove", drive)
	return c.Put(endpoint, nil)
}

// DrivesOn enables selected drive
func (c *Client) DrivesOn(drive string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/drives/%s:on", drive)
	return c.Put(endpoint, nil)
}

// DrivesOff disables selected drive
func (c *Client) DrivesOff(drive string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/drives/%s:off", drive)
	return c.Put(endpoint, nil)
}

// DrivesLoadROM loads custom ROM (16K/32K) temporarily
// drive: drive number (e.g., "8", "9")
// file: path to ROM file on C64U filesystem
func (c *Client) DrivesLoadROM(drive, file string) (*Response, error) {
	params := map[string]string{
		"file": file,
	}

	endpoint := fmt.Sprintf("/v1/drives/%s:load_rom", drive)
	return c.Put(endpoint, params)
}

// DrivesLoadROMUpload uploads and loads custom ROM
// drive: drive number (e.g., "8", "9")
// localFile: path to local ROM file
func (c *Client) DrivesLoadROMUpload(drive, localFile string) (*Response, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	endpoint := fmt.Sprintf("/v1/drives/%s:load_rom", drive)
	return c.Post(endpoint, file, nil)
}

// DrivesSetMode changes drive mode
// drive: drive number (e.g., "8", "9")
// mode: 1541, 1571, or 1581
func (c *Client) DrivesSetMode(drive, mode string) (*Response, error) {
	params := map[string]string{
		"mode": mode,
	}

	endpoint := fmt.Sprintf("/v1/drives/%s:set_mode", drive)
	return c.Put(endpoint, params)
}
