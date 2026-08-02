package diskimage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// D64 format constants based on https://ist.uwaterloo.ca/~schepers/formats/D64.TXT
const (
	// Standard D64 sizes
	D64Size35Tracks = 174848 // 35 tracks, no error bytes
	D64Size40Tracks = 196608 // 40 tracks (extended), no error bytes

	// Sector and track layout
	SectorSize       = 256
	MaxTracks35      = 35
	MaxTracks40      = 40
	MaxDirEntries    = 144 // 18 sectors * 8 entries per sector
	DirEntrySize     = 32
	EntriesPerSector = 8

	// Track/sector locations
	BAMTrack       = 18
	BAMSector      = 0
	DirTrack       = 18
	DirStartSector = 1
	MaxDirSectors  = 18

	// File types (bits 0-3 of type byte)
	FileTypeDEL = 0x00
	FileTypeSEQ = 0x81
	FileTypePRG = 0x82
	FileTypeUSR = 0x83
	FileTypeREL = 0x84

	// File type flags
	FileTypeClosed = 0x80 // Bit 7: file properly closed
	FileTypeLocked = 0x40 // Bit 6: file locked

	// DOS version
	DOSVersion = 0x41 // Standard DOS 2.6 ('A')

	// Padding character for filenames and disk names
	PETSCIIPadding = 0xA0

	// Default sector interleave
	DefaultInterleave = 10
)

// Track sectors distribution
var trackSectors = []int{
	0,                                                                  // Track 0 doesn't exist
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, // Tracks 1-17: 21 sectors
	19, 19, 19, 19, 19, 19, 19, // Tracks 18-24: 19 sectors
	18, 18, 18, 18, 18, 18, // Tracks 25-30: 18 sectors
	17, 17, 17, 17, 17, // Tracks 31-35: 17 sectors
	// Extended tracks for 40-track images
	17, 17, 17, 17, 17, // Tracks 36-40: 17 sectors
}

// D64 represents a Commodore 64 disk image
type D64 struct {
	data          []byte
	tracks        int
	diskName      string
	diskID        string
	interleave    int
	nextDirSector int
}

// NewD64 creates a new empty D64 disk image
func NewD64(diskName, diskID string, tracks int) (*D64, error) {
	if tracks != MaxTracks35 && tracks != MaxTracks40 {
		return nil, fmt.Errorf("invalid track count: %d (must be 35 or 40)", tracks)
	}

	size := D64Size35Tracks
	if tracks == MaxTracks40 {
		size = D64Size40Tracks
	}

	d64 := &D64{
		data:          make([]byte, size),
		tracks:        tracks,
		diskName:      diskName,
		diskID:        diskID,
		interleave:    DefaultInterleave,
		nextDirSector: DirStartSector,
	}

	// Initialize the disk with empty data
	for i := range d64.data {
		d64.data[i] = 0x00
	}

	// Write BAM and directory structure
	if err := d64.initializeBAM(); err != nil {
		return nil, err
	}

	if err := d64.initializeDirectory(); err != nil {
		return nil, err
	}

	return d64, nil
}

// getSectorOffset returns the byte offset for a given track/sector
func (d *D64) getSectorOffset(track, sector int) (int, error) {
	if track < 1 || track > d.tracks {
		return 0, fmt.Errorf("invalid track: %d (valid: 1-%d)", track, d.tracks)
	}

	maxSectors := trackSectors[track]
	if sector < 0 || sector >= maxSectors {
		return 0, fmt.Errorf("invalid sector: %d for track %d (valid: 0-%d)", sector, track, maxSectors-1)
	}

	offset := 0
	for t := 1; t < track; t++ {
		offset += trackSectors[t] * SectorSize
	}
	offset += sector * SectorSize

	return offset, nil
}

// initializeBAM creates the Block Availability Map in track 18, sector 0
func (d *D64) initializeBAM() error {
	offset, err := d.getSectorOffset(BAMTrack, BAMSector)
	if err != nil {
		return err
	}

	bam := d.data[offset : offset+SectorSize]

	// Bytes 00-01: First directory sector (track 18, sector 1)
	bam[0x00] = DirTrack
	bam[0x01] = DirStartSector

	// Byte 02: DOS version ('A' = $41)
	bam[0x02] = DOSVersion

	// Bytes 04-8F: BAM entries (4 bytes per track)
	for track := 1; track <= d.tracks; track++ {
		bamOffset := 0x04 + (track-1)*4

		// Don't mark track 18 as free (reserved for directory)
		if track == DirTrack {
			bam[bamOffset] = 0 // 0 free sectors
			bam[bamOffset+1] = 0x00
			bam[bamOffset+2] = 0x00
			bam[bamOffset+3] = 0x00
		} else {
			sectors := trackSectors[track]
			bam[bamOffset] = byte(sectors) // All sectors free

			// Set bitmap (1 = free, 0 = used)
			// Sectors 0-7 in byte 1, 8-15 in byte 2, 16-20 in byte 3
			for i := 0; i < 3; i++ {
				var bitmap byte = 0xFF
				startSector := i * 8
				endSector := startSector + 8
				if endSector > sectors {
					endSector = sectors
				}

				// Only set bits for existing sectors
				for s := startSector; s < endSector; s++ {
					bitmap |= (1 << (s - startSector))
				}

				// Clear bits for non-existing sectors
				if endSector < startSector+8 {
					for s := endSector; s < startSector+8; s++ {
						bitmap &^= (1 << (s - startSector))
					}
				}

				bam[bamOffset+1+i] = bitmap
			}
		}
	}

	// Bytes 90-9F: Disk name (16 chars, PETSCII, padded with $A0)
	diskNamePadded := padPETSCII(d.diskName, 16)
	copy(bam[0x90:0xA0], diskNamePadded)

	// Bytes A0-A1: Filled with $A0
	bam[0xA0] = PETSCIIPadding
	bam[0xA1] = PETSCIIPadding

	// Bytes A2-A3: Disk ID
	diskIDPadded := padPETSCII(d.diskID, 2)
	copy(bam[0xA2:0xA4], diskIDPadded)

	// Byte A4: Usually $A0
	bam[0xA4] = PETSCIIPadding

	// Bytes A5-A6: DOS type "2A"
	bam[0xA5] = '2'
	bam[0xA6] = 'A'

	// Bytes A7-AA: Filled with $A0
	for i := 0xA7; i <= 0xAA; i++ {
		bam[i] = PETSCIIPadding
	}

	return nil
}

// initializeDirectory sets up the first directory sector
func (d *D64) initializeDirectory() error {
	offset, err := d.getSectorOffset(DirTrack, DirStartSector)
	if err != nil {
		return err
	}

	dirSector := d.data[offset : offset+SectorSize]

	// Bytes 00-01: Next directory sector (0,FF = last sector)
	dirSector[0x00] = 0x00
	dirSector[0x01] = 0xFF

	// Initialize all directory entries as empty (file type = 0x00)
	for i := 0; i < EntriesPerSector; i++ {
		entryOffset := 0x02 + i*DirEntrySize
		dirSector[entryOffset+0x00] = FileTypeDEL // File type: scratched/deleted
	}

	return nil
}

// allocateSector marks a sector as used in the BAM and returns true if successful
func (d *D64) allocateSector(track, sector int) error {
	bamOffset, err := d.getSectorOffset(BAMTrack, BAMSector)
	if err != nil {
		return err
	}

	bam := d.data[bamOffset : bamOffset+SectorSize]

	bamEntryOffset := 0x04 + (track-1)*4
	freeSectors := bam[bamEntryOffset]

	if freeSectors == 0 {
		return fmt.Errorf("track %d is full", track)
	}

	// Calculate which byte and bit to clear
	byteIndex := sector / 8
	bitIndex := sector % 8

	bitmap := &bam[bamEntryOffset+1+byteIndex]

	// Check if sector is already allocated
	if (*bitmap & (1 << bitIndex)) == 0 {
		return fmt.Errorf("sector %d/%d is already allocated", track, sector)
	}

	// Mark sector as used (clear bit)
	*bitmap &^= (1 << bitIndex)

	// Decrement free sector count
	bam[bamEntryOffset]--

	return nil
}

// findFreeSector finds the next free sector on a track
func (d *D64) findFreeSector(track, startSector int) (int, error) {
	bamOffset, err := d.getSectorOffset(BAMTrack, BAMSector)
	if err != nil {
		return -1, err
	}

	bam := d.data[bamOffset : bamOffset+SectorSize]
	bamEntryOffset := 0x04 + (track-1)*4

	freeSectors := bam[bamEntryOffset]
	if freeSectors == 0 {
		return -1, fmt.Errorf("track %d is full", track)
	}

	maxSectors := trackSectors[track]

	// Try from startSector with interleave
	for i := 0; i < maxSectors; i++ {
		sector := (startSector + i*d.interleave) % maxSectors

		byteIndex := sector / 8
		bitIndex := sector % 8
		bitmap := bam[bamEntryOffset+1+byteIndex]

		if (bitmap & (1 << bitIndex)) != 0 {
			return sector, nil
		}
	}

	// If no free sector found with interleave, try sequential
	for sector := 0; sector < maxSectors; sector++ {
		byteIndex := sector / 8
		bitIndex := sector % 8
		bitmap := bam[bamEntryOffset+1+byteIndex]

		if (bitmap & (1 << bitIndex)) != 0 {
			return sector, nil
		}
	}

	return -1, fmt.Errorf("no free sectors on track %d", track)
}

// padPETSCII pads a string with PETSCII $A0 characters to the specified length
// and converts ASCII to PETSCII screen codes
func padPETSCII(s string, length int) []byte {
	result := make([]byte, length)
	for i := range result {
		result[i] = PETSCIIPadding
	}

	// Convert ASCII to PETSCII screen codes
	// In C64 directory, uppercase letters A-Z are stored as $41-$5A (ASCII codes)
	// Numbers 0-9 are $30-$39
	// Space is $20
	// Special chars vary
	sUpper := strings.ToUpper(s)
	for i := 0; i < len(sUpper) && i < length; i++ {
		result[i] = byte(sUpper[i])
	}

	return result
}

// WriteTo writes the D64 image to an io.Writer
func (d *D64) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(d.data)
	return int64(n), err
}

// WriteFile writes the D64 image to a file
func (d *D64) WriteFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = d.WriteTo(f)
	return err
}

// Bytes returns the raw D64 image data.
func (d *D64) Bytes() ([]byte, error) {
	result := make([]byte, len(d.data))
	copy(result, d.data)
	return result, nil
}

// DirEntry represents a single directory entry in a D64 image.
type DirEntry struct {
	Name     string // PETSCII filename, spaces trimmed
	FileType string // "PRG", "SEQ", "USR", "REL", "DEL"
	Blocks   int
}

// ReadD64Directory parses directory entries from raw D64 image bytes.
// Returns entries, disk name, blocks used, error.
func ReadD64Directory(data []byte) ([]DirEntry, string, int, error) {
	if len(data) < D64Size35Tracks {
		return nil, "", 0, fmt.Errorf("invalid D64: size %d < minimum %d", len(data), D64Size35Tracks)
	}

	bamOffset, err := sectorOffset(BAMTrack, BAMSector)
	if err != nil {
		return nil, "", 0, err
	}

	diskName := petsciiToASCII(data[bamOffset+0x90 : bamOffset+0xA0])

	totalFree := 0
	for track := 1; track <= MaxTracks35; track++ {
		if track == BAMTrack {
			continue
		}
		bamEntry := bamOffset + 4 + (track-1)*4
		if bamEntry+1 <= len(data) {
			totalFree += int(data[bamEntry])
		}
	}
	const totalSectors = 664 // standard 35-track D64: 664 user-accessible sectors (excl. track 18)
	blocksUsed := totalSectors - totalFree

	var entries []DirEntry
	track, sector := DirTrack, DirStartSector
	visited := map[[2]int]bool{}

	for {
		key := [2]int{track, sector}
		if visited[key] {
			break
		}
		visited[key] = true

		off, err := sectorOffset(track, sector)
		if err != nil || off+SectorSize > len(data) {
			break
		}
		sec := data[off : off+SectorSize]

		nextTrack := int(sec[0])
		nextSector := int(sec[1])

		for i := 0; i < EntriesPerSector; i++ {
			entryStart := 2 + i*DirEntrySize
			if entryStart+DirEntrySize > len(sec) {
				break
			}
			entry := sec[entryStart : entryStart+DirEntrySize]
			fileType := entry[0]
			if fileType == 0x00 {
				continue
			}
			name := petsciiToASCII(entry[3:19])
			blocks := int(entry[28]) | int(entry[29])<<8
			entries = append(entries, DirEntry{
				Name:     name,
				FileType: fileTypeName(fileType),
				Blocks:   blocks,
			})
		}

		if nextTrack == 0 {
			break
		}
		track, sector = nextTrack, nextSector
	}

	return entries, diskName, blocksUsed, nil
}

// ExtractPRG extracts a PRG file by name from raw D64 data.
func ExtractPRG(data []byte, name string) ([]byte, error) {
	entries, _, _, err := ReadD64Directory(data)
	if err != nil {
		return nil, err
	}
	found := false
	for i := range entries {
		if strings.EqualFold(entries[i].Name, name) {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("file %q not found in disk image", name)
	}
	return extractFileData(data, name)
}

// extractFileData walks the D64 directory to find and extract file data by name.
func extractFileData(data []byte, name string) ([]byte, error) {
	track, sector := DirTrack, DirStartSector
	visited := map[[2]int]bool{}

	for {
		key := [2]int{track, sector}
		if visited[key] {
			break
		}
		visited[key] = true

		off, err := sectorOffset(track, sector)
		if err != nil || off+SectorSize > len(data) {
			break
		}
		sec := data[off : off+SectorSize]

		for i := 0; i < EntriesPerSector; i++ {
			entryStart := 2 + i*DirEntrySize
			if entryStart+DirEntrySize > len(sec) {
				break
			}
			entry := sec[entryStart : entryStart+DirEntrySize]
			if entry[0] == 0x00 {
				continue
			}
			entryName := petsciiToASCII(entry[3:19])
			if !strings.EqualFold(entryName, name) {
				continue
			}
			fileTrack := int(entry[1])
			fileSector := int(entry[2])
			if fileTrack < 1 {
				return nil, fmt.Errorf("corrupt dir entry %q: invalid start track %d", name, fileTrack)
			}
			var fileData []byte
			fileVisited := map[[2]int]bool{}
			for {
				fkey := [2]int{fileTrack, fileSector}
				if fileVisited[fkey] {
					break
				}
				fileVisited[fkey] = true
				foff, err := sectorOffset(fileTrack, fileSector)
				if err != nil || foff+SectorSize > len(data) {
					break
				}
				fsec := data[foff : foff+SectorSize]
				nextTrack := int(fsec[0])
				nextSector := int(fsec[1])
				if nextTrack == 0 {
					end := nextSector + 1
					if end < 2 {
						end = 2
					}
					fileData = append(fileData, fsec[2:end]...)
					break
				}
				fileData = append(fileData, fsec[2:]...)
				fileTrack, fileSector = nextTrack, nextSector
			}
			return fileData, nil
		}

		nextTrack := int(sec[0])
		nextSector := int(sec[1])
		if nextTrack == 0 {
			break
		}
		track, sector = nextTrack, nextSector
	}
	return nil, fmt.Errorf("file %q not found", name)
}

// sectorOffset returns the byte offset for a given track/sector in D64 data.
func sectorOffset(track, sector int) (int, error) {
	if track < 1 || track > MaxTracks40 {
		return 0, fmt.Errorf("invalid track %d", track)
	}
	offset := 0
	for t := 1; t < track; t++ {
		if t < len(trackSectors) {
			offset += trackSectors[t] * SectorSize
		}
	}
	if track < len(trackSectors) && sector >= trackSectors[track] {
		return 0, fmt.Errorf("invalid sector %d for track %d", sector, track)
	}
	return offset + sector*SectorSize, nil
}

// petsciiToASCII converts PETSCII bytes to a printable ASCII string, trimming padding.
// PETSCII 0x41-0x5A are uppercase letters (same as ASCII), kept as-is.
// PETSCII 0x61-0x7A are lowercase letters, mapped to uppercase (C64 convention).
func petsciiToASCII(b []byte) string {
	var result []byte
	for _, c := range b {
		if c == PETSCIIPadding {
			break
		}
		if c >= 0x61 && c <= 0x7A {
			result = append(result, c-0x20) // PETSCII lowercase → ASCII uppercase
		} else if c >= 0x20 && c < 0x80 {
			result = append(result, c)
		} else {
			result = append(result, '?')
		}
	}
	return strings.TrimRight(string(result), " ")
}

// fileTypeName converts a raw file type byte to a display string.
func fileTypeName(t byte) string {
	switch t & 0x0F {
	case 0x01:
		return "SEQ"
	case 0x02:
		return "PRG"
	case 0x03:
		return "USR"
	case 0x04:
		return "REL"
	default:
		return "DEL"
	}
}

// AddFile adds a file to the disk image
func (d *D64) AddFile(localPath string, diskFilename string, fileType byte) error {
	// Read the local file
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", localPath, err)
	}

	// Use basename if no disk filename specified
	if diskFilename == "" {
		diskFilename = filepath.Base(localPath)
		// Remove extension
		if idx := strings.LastIndex(diskFilename, "."); idx > 0 {
			diskFilename = diskFilename[:idx]
		}
	}

	// Default to PRG file type
	if fileType == 0 {
		fileType = FileTypePRG | FileTypeClosed
	}

	return d.addFileData(diskFilename, data, fileType)
}

// addFileData adds file data to the disk
func (d *D64) addFileData(filename string, data []byte, fileType byte) error {
	// Find free directory entry
	dirEntry, err := d.allocateDirectoryEntry()
	if err != nil {
		return err
	}

	// Calculate number of sectors needed
	sectorsNeeded := (len(data) + SectorSize - 1) / SectorSize
	if sectorsNeeded == 0 {
		sectorsNeeded = 1 // Minimum 1 sector
	}

	// Find a suitable track (avoid track 18)
	startTrack := 17
	if startTrack == DirTrack {
		startTrack--
	}

	// Allocate sectors and write file data
	var firstTrack, firstSector int
	lastTrack, lastSector := 0, 0

	currentTrack := startTrack
	currentSector := 0

	for i := 0; i < sectorsNeeded; i++ {
		// Find free sector
		sector, err := d.findFreeSector(currentTrack, currentSector)
		if err != nil {
			// Try next track
			currentTrack--
			if currentTrack == DirTrack {
				currentTrack--
			}
			if currentTrack < 1 {
				return fmt.Errorf("disk full: cannot allocate sector for file")
			}
			sector, err = d.findFreeSector(currentTrack, 0)
			if err != nil {
				return err
			}
		}

		// Allocate sector
		if err := d.allocateSector(currentTrack, sector); err != nil {
			return err
		}

		// Remember first sector
		if i == 0 {
			firstTrack = currentTrack
			firstSector = sector
		}

		// Write data to sector
		offset, _ := d.getSectorOffset(currentTrack, sector)
		sectorData := d.data[offset : offset+SectorSize]

		// Calculate data to write to this sector
		dataStart := i * (SectorSize - 2) // First 2 bytes are link
		dataEnd := dataStart + (SectorSize - 2)
		if dataEnd > len(data) {
			dataEnd = len(data)
		}

		// Check if this is the last sector
		isLastSector := (i == sectorsNeeded-1)

		if isLastSector {
			// Last sector: track = 0, sector = bytes used
			bytesUsed := (dataEnd - dataStart) + 2 // +2 for the link bytes
			sectorData[0] = 0x00
			sectorData[1] = byte(bytesUsed)
		} else {
			// Link to next sector (will be filled in next iteration)
			// For now, just mark as continuing
			sectorData[0] = byte(currentTrack)
			sectorData[1] = byte((sector + d.interleave) % trackSectors[currentTrack])
		}

		// Copy file data (starting at byte 2)
		copy(sectorData[2:], data[dataStart:dataEnd])

		// Update last track/sector for linking
		if lastTrack != 0 {
			// Update previous sector's link
			lastOffset, _ := d.getSectorOffset(lastTrack, lastSector)
			d.data[lastOffset] = byte(currentTrack)
			d.data[lastOffset+1] = byte(sector)
		}

		lastTrack = currentTrack
		lastSector = sector
		currentSector = (sector + d.interleave) % trackSectors[currentTrack]
	}

	// Write directory entry
	// dirEntry points to start of 32-byte entry (already offset by allocateDirectoryEntry)
	// Directory entry format (each entry is 32 bytes):
	// 0x00: File type (with flags in high bits)
	// 0x01: Track of first data sector
	// 0x02: Sector of first data sector
	// 0x03-0x12: Filename (16 bytes PETSCII, padded with $A0)
	// 0x13-0x1B: REL file side-sector info (zero for PRG/SEQ/USR)
	// 0x1C-0x1D: File size in sectors (low byte, high byte)
	// 0x1E-0x1F: Unused (zero)

	dirEntry[0x00] = fileType
	dirEntry[0x01] = byte(firstTrack)
	dirEntry[0x02] = byte(firstSector)

	// Filename (16 bytes, PETSCII padded)
	filenamePadded := padPETSCII(filename, 16)
	copy(dirEntry[0x03:0x13], filenamePadded)

	// Bytes 0x13-0x1B: Zero for PRG/SEQ/USR files (REL file info)
	for i := 0x13; i <= 0x1B; i++ {
		dirEntry[i] = 0x00
	}

	// File size in sectors (little endian)
	dirEntry[0x1C] = byte(sectorsNeeded & 0xFF)
	dirEntry[0x1D] = byte((sectorsNeeded >> 8) & 0xFF)

	// Bytes 0x1E-0x1F: Unused
	dirEntry[0x1E] = 0x00
	dirEntry[0x1F] = 0x00

	return nil
}

// allocateDirectoryEntry finds and returns a free directory entry
func (d *D64) allocateDirectoryEntry() ([]byte, error) {
	for sector := DirStartSector; sector < DirStartSector+MaxDirSectors; sector++ {
		offset, err := d.getSectorOffset(DirTrack, sector)
		if err != nil {
			continue
		}

		dirSector := d.data[offset : offset+SectorSize]

		// Check if this sector is allocated (first directory entry will be non-zero in first BAM read)
		// For simplicity, we check if the track link is valid
		if sector > DirStartSector && dirSector[0x00] == 0x00 && dirSector[0x01] == 0x00 {
			// This directory sector isn't initialized yet
			// Initialize it
			dirSector[0x00] = 0x00
			dirSector[0x01] = 0xFF
		}

		// Look for free entry (file type = 0x00 or DEL)
		for i := 0; i < EntriesPerSector; i++ {
			entryOffset := 0x02 + i*DirEntrySize
			entry := dirSector[entryOffset : entryOffset+DirEntrySize]

			// Check file type at byte 0 of directory entry
			if entry[0x00] == FileTypeDEL || entry[0x00] == 0x00 {
				return entry, nil
			}
		}
	}

	return nil, fmt.Errorf("directory full (max %d entries)", MaxDirEntries)
}
