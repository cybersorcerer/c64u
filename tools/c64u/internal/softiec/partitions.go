package softiec

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// PartitionFile is where the firmware keeps the SoftIEC partitions. The drives
// list reports the partitions, but no REST endpoint sets them. Firmware 1.2.1
// loads this file by itself only at power-on - machine reboot restarts only the
// C64 and its cartridge - and at once when it is opened with Load in the menu's
// file browser. Either way it replaces the whole list: the built-in partition 1
// on /Temp is gone unless the file names it, and the first partition becomes the
// current one (checked 2026-11-27).
const PartitionFile = "/Flash/config/iec_partitions.ipr"

// maxPartitionName is as long as the drive keeps a partition name.
const maxPartitionName = 16

// Partition is one SoftIEC partition, in the file's own field names.
type Partition struct {
	Number int    `json:"number"`
	Path   string `json:"path"`
	Name   string `json:"name,omitempty"`
}

// Files is the part of the device's FTP access the partition file needs.
type Files interface {
	FTPList(path string) ([]api.FileEntry, error)
	FTPReadFile(path string) ([]byte, error)
	FTPUploadBytes(path string, data []byte) error
	FTPDelete(path string) error
}

// partitionFile is the file's layout, as the firmware writes it.
type partitionFile struct {
	Version    int         `json:"version"`
	Partitions []Partition `json:"partitions"`
}

// ParsePartition reads N=[NAME:]PATH. A path starts with a slash, so whatever
// stands before the first colon of anything else is the name.
func ParsePartition(arg string) (Partition, error) {
	num, rest, ok := strings.Cut(arg, "=")
	if !ok {
		return Partition{}, fmt.Errorf("%q: want N=[NAME:]PATH", arg)
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return Partition{}, fmt.Errorf("%q: partition number must be 1 to 255", arg)
	}
	p := Partition{Number: n, Path: rest}
	if !strings.HasPrefix(rest, "/") {
		name, dir, found := strings.Cut(rest, ":")
		if !found {
			return Partition{}, fmt.Errorf("%q: path must be absolute, starting with /", arg)
		}
		if name == "" {
			return Partition{}, fmt.Errorf("%q: name before the colon is empty", arg)
		}
		p.Name, p.Path = name, dir
	}
	if err := validatePartition(p); err != nil {
		return Partition{}, fmt.Errorf("%q: %w", arg, err)
	}
	return p, nil
}

func validatePartition(p Partition) error {
	if p.Number < 1 || p.Number > 255 {
		return fmt.Errorf("partition number must be 1 to 255")
	}
	if err := ValidatePath(p.Path); err != nil {
		return err
	}
	if len(p.Name) > maxPartitionName {
		return fmt.Errorf("name is longer than %d characters", maxPartitionName)
	}
	if strings.Contains(p.Name, `"`) {
		return fmt.Errorf("name must not contain a quote")
	}
	return nil
}

// EncodePartitions renders the partition file, sorted by number so the lowest
// number becomes the current partition.
func EncodePartitions(parts []Partition) ([]byte, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("no partitions given")
	}
	sorted := append([]Partition(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })
	for i, p := range sorted {
		if err := validatePartition(p); err != nil {
			return nil, fmt.Errorf("partition %d: %w", p.Number, err)
		}
		if i > 0 && sorted[i-1].Number == p.Number {
			return nil, fmt.Errorf("partition %d is given twice", p.Number)
		}
	}
	return json.Marshal(partitionFile{Version: 1, Partitions: sorted})
}

// SavePartitions writes the partition file after checking that every path is
// a directory on the device. The firmware loads it at the next power-on.
func SavePartitions(f Files, parts []Partition) error {
	data, err := EncodePartitions(parts)
	if err != nil {
		return err
	}
	for _, p := range parts {
		ok, err := exists(f, p.Path, true)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("partition %d: directory %s does not exist on the device", p.Number, p.Path)
		}
	}
	return f.FTPUploadBytes(PartitionFile, data)
}

// SavedPartitions reads the partition file; ok is false when there is none.
func SavedPartitions(f Files) (parts []Partition, ok bool, err error) {
	ok, err = exists(f, PartitionFile, false)
	if err != nil || !ok {
		return nil, false, err
	}
	data, err := f.FTPReadFile(PartitionFile)
	if err != nil {
		return nil, true, err
	}
	var file partitionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, true, fmt.Errorf("%s is not a partition file: %w", PartitionFile, err)
	}
	return file.Partitions, true, nil
}

// ClearPartitions deletes the partition file; existed is false when there was
// none.
func ClearPartitions(f Files) (existed bool, err error) {
	existed, err = exists(f, PartitionFile, false)
	if err != nil || !existed {
		return false, err
	}
	return true, f.FTPDelete(PartitionFile)
}

// exists looks p up in its parent directory. Listing a missing path over FTP
// returns an empty list, not an error, so the path itself cannot be listed to
// find out. Names are compared without case, as the device does.
func exists(f Files, p string, dir bool) (bool, error) {
	p = path.Clean(p)
	if p == "/" {
		return dir, nil
	}
	entries, err := f.FTPList(path.Dir(p))
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name, path.Base(p)) && e.IsDir == dir {
			return true, nil
		}
	}
	return false, nil
}

// ActivePartitions reads the partitions the drive serves now. The drives list
// reports number and path, not the name.
func ActivePartitions(d Device) ([]Partition, error) {
	fields, err := iecFields(d)
	if err != nil || fields == nil {
		return nil, err
	}
	list, _ := fields["partitions"].([]interface{})
	var parts []Partition
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		p := Partition{}
		if id, ok := m["id"].(float64); ok {
			p.Number = int(id)
		}
		p.Path, _ = m["path"].(string)
		parts = append(parts, p)
	}
	return parts, nil
}
