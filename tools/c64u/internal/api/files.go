package api

import (
	"fmt"
	"strconv"
)

// File Manipulation API

// FilesInfo returns file size and extension (supports wildcards)
func (c *Client) FilesInfo(path string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/files/%s:info", path)
	return c.Get(endpoint, nil)
}

// FilesCreateD64 creates a D64 image
// path: destination path on C64U filesystem
// tracks: 35 or 40
// diskName: optional disk name
func (c *Client) FilesCreateD64(path string, tracks int, diskName string) (*Response, error) {
	params := make(map[string]string)

	if tracks > 0 {
		params["tracks"] = strconv.Itoa(tracks)
	}

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d64", path)
	return c.Put(endpoint, params)
}

// FilesCreateD71 creates a D71 image (70 tracks fixed)
// path: destination path on C64U filesystem
// diskName: optional disk name
func (c *Client) FilesCreateD71(path string, diskName string) (*Response, error) {
	params := make(map[string]string)

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d71", path)
	return c.Put(endpoint, params)
}

// FilesCreateD81 creates a D81 image (160 tracks fixed)
// path: destination path on C64U filesystem
// diskName: optional disk name
func (c *Client) FilesCreateD81(path string, diskName string) (*Response, error) {
	params := make(map[string]string)

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_d81", path)
	return c.Put(endpoint, params)
}

// FilesCreateDNP creates a DNP image (max 255 tracks)
// path: destination path on C64U filesystem
// tracks: number of tracks (max 255, ~16MB)
// diskName: optional disk name
func (c *Client) FilesCreateDNP(path string, tracks int, diskName string) (*Response, error) {
	params := map[string]string{
		"tracks": strconv.Itoa(tracks),
	}

	if diskName != "" {
		params["diskname"] = diskName
	}

	endpoint := fmt.Sprintf("/v1/files/%s:create_dnp", path)
	return c.Put(endpoint, params)
}
