package diskimage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackOptions contains options for packing a directory into a disk image
type PackOptions struct {
	DiskName   string   // Disk name (max 16 chars)
	DiskID     string   // Disk ID (2 chars), default "2A"
	Tracks     int      // Number of tracks (35 or 40)
	Interleave int      // Sector interleave, default 10
	Filter     []string // File extensions to include (e.g., [".prg", ".seq"])
}

// DefaultPackOptions returns default packing options
func DefaultPackOptions() *PackOptions {
	return &PackOptions{
		DiskName:   "DISK",
		DiskID:     "2A",
		Tracks:     35,
		Interleave: DefaultInterleave,
		Filter:     nil, // Include all files
	}
}

// PackDirectory creates a D64 image from a directory
func PackDirectory(sourceDir string, outputPath string, opts *PackOptions) error {
	if opts == nil {
		opts = DefaultPackOptions()
	}

	// Validate disk name length
	if len(opts.DiskName) > 16 {
		opts.DiskName = opts.DiskName[:16]
	}

	// Validate disk ID length
	if len(opts.DiskID) != 2 {
		if len(opts.DiskID) > 2 {
			opts.DiskID = opts.DiskID[:2]
		} else {
			opts.DiskID = "2A"
		}
	}

	// Create new D64 image
	d64, err := NewD64(opts.DiskName, opts.DiskID, opts.Tracks)
	if err != nil {
		return err
	}

	// Set interleave
	if opts.Interleave > 0 {
		d64.interleave = opts.Interleave
	}

	// Collect files from directory
	files, err := collectFiles(sourceDir, opts.Filter)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in directory %s", sourceDir)
	}

	if len(files) > MaxDirEntries {
		return fmt.Errorf("too many files (%d), maximum is %d", len(files), MaxDirEntries)
	}

	// Add files to disk image
	for _, file := range files {
		// Determine file type from extension
		fileType := getFileType(file.Path)

		// Use filename without extension as disk filename
		diskFilename := file.Name

		err := d64.AddFile(file.Path, diskFilename, fileType)
		if err != nil {
			return fmt.Errorf("failed to add file %s: %w", file.Path, err)
		}
	}

	// Write D64 image to file
	if err := d64.WriteFile(outputPath); err != nil {
		return fmt.Errorf("failed to write D64 image: %w", err)
	}

	return nil
}

// FileInfo represents a file to be added to the disk
type FileInfo struct {
	Path string
	Name string
}

// collectFiles scans a directory and returns all files matching the filter
func collectFiles(dir string, filter []string) ([]FileInfo, error) {
	var files []FileInfo

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories for now
		}

		fullPath := filepath.Join(dir, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		// Apply filter if specified
		if len(filter) > 0 {
			matched := false
			for _, filterExt := range filter {
				if strings.ToLower(filterExt) == ext {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Keep full filename with extension
		name := entry.Name()

		// Truncate to 16 characters (max PETSCII filename length)
		if len(name) > 16 {
			name = name[:16]
		}

		files = append(files, FileInfo{
			Path: fullPath,
			Name: name,
		})
	}

	return files, nil
}

// getFileType determines the C64 file type from the file extension
func getFileType(filename string) byte {
	ext := strings.ToLower(filepath.Ext(filename))

	// Known binary types
	switch ext {
	case ".prg", ".p00":
		return FileTypePRG | FileTypeClosed
	case ".usr", ".u00":
		return FileTypeUSR | FileTypeClosed
	case ".rel", ".r00":
		return FileTypeREL | FileTypeClosed
	case ".t64", ".sid", ".crt":
		return FileTypePRG | FileTypeClosed
	}

	// Known text types
	switch ext {
	case ".seq", ".s00":
		return FileTypeSEQ | FileTypeClosed
	case ".txt", ".asm", ".sym", ".bas", ".md", ".log":
		return FileTypeSEQ | FileTypeClosed
	}

	// Unknown extension - check file content
	if isTextFile(filename) {
		return FileTypeSEQ | FileTypeClosed
	}

	// Default to PRG for binary files
	return FileTypePRG | FileTypeClosed
}

// isTextFile checks if a file contains text by examining its content
func isTextFile(filename string) bool {
	// Read first 512 bytes to determine if it's text
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false
	}

	// Check if content is mostly printable ASCII/UTF-8
	printableCount := 0
	for i := 0; i < n; i++ {
		b := buffer[i]
		// Printable ASCII (0x20-0x7E) + common whitespace (tab, newline, carriage return)
		if (b >= 0x20 && b <= 0x7E) || b == 0x09 || b == 0x0A || b == 0x0D {
			printableCount++
		} else if b == 0x00 {
			// Null bytes are strong indicator of binary file
			return false
		}
	}

	// If more than 85% of bytes are printable, consider it text
	return n > 0 && (float64(printableCount)/float64(n)) > 0.85
}

// GetDiskInfo returns information about files that would be packed
func GetDiskInfo(sourceDir string, filter []string) ([]FileInfo, error) {
	return collectFiles(sourceDir, filter)
}
