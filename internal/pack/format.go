package pack

import (
	"fmt"
	"strings"

	"github.com/nvandessel/feedback-loop/internal/backup"
)

// WritePackFile writes a BackupFormat as a pack file with pack metadata in the header.
func WritePackFile(path string, data *backup.BackupFormat, manifest PackManifest, writeOpts *backup.WriteOptions) error {
	if err := ValidatePackID(string(manifest.ID)); err != nil {
		return err
	}

	// Build combined write options with pack metadata
	opts := &backup.WriteOptions{}
	if writeOpts != nil {
		opts.FloopVersion = writeOpts.FloopVersion
		// Start with user metadata
		opts.Metadata = make(map[string]string)
		for k, v := range writeOpts.Metadata {
			opts.Metadata[k] = v
		}
	} else {
		opts.Metadata = make(map[string]string)
	}

	// Inject pack metadata
	opts.Metadata[MetaKeyType] = PackFileType
	opts.Metadata[MetaKeyPackID] = string(manifest.ID)
	opts.Metadata[MetaKeyPackVer] = manifest.Version
	if manifest.Author != "" {
		opts.Metadata[MetaKeyPackAuthor] = manifest.Author
	}
	if manifest.Description != "" {
		opts.Metadata[MetaKeyPackDesc] = manifest.Description
	}
	if len(manifest.Tags) > 0 {
		opts.Metadata[MetaKeyPackTags] = strings.Join(manifest.Tags, ",")
	}
	if manifest.Source != "" {
		opts.Metadata[MetaKeyPackSource] = manifest.Source
	}

	return backup.WriteV2(path, data, opts)
}

// ReadPackFile reads a pack file and extracts the manifest from the header.
func ReadPackFile(path string) (*backup.BackupFormat, *PackManifest, error) {
	header, err := backup.ReadV2Header(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading pack header: %w", err)
	}

	if header.Metadata[MetaKeyType] != PackFileType {
		return nil, nil, fmt.Errorf("file is not a skill pack (type=%q)", header.Metadata[MetaKeyType])
	}

	manifest := headerToManifest(header)

	data, err := backup.ReadV2(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading pack data: %w", err)
	}

	return data, manifest, nil
}

// ReadPackHeader reads only the header of a pack file and returns the manifest.
func ReadPackHeader(path string) (*PackManifest, error) {
	header, err := backup.ReadV2Header(path)
	if err != nil {
		return nil, fmt.Errorf("reading pack header: %w", err)
	}

	if header.Metadata[MetaKeyType] != PackFileType {
		return nil, fmt.Errorf("file is not a skill pack (type=%q)", header.Metadata[MetaKeyType])
	}

	return headerToManifest(header), nil
}

// IsPackFile checks if a file is a pack file by reading its header.
func IsPackFile(path string) bool {
	header, err := backup.ReadV2Header(path)
	if err != nil {
		return false
	}
	return header.Metadata[MetaKeyType] == PackFileType
}

// headerToManifest extracts a PackManifest from a BackupHeader.
func headerToManifest(header *backup.BackupHeader) *PackManifest {
	manifest := &PackManifest{
		ID:          PackID(header.Metadata[MetaKeyPackID]),
		Version:     header.Metadata[MetaKeyPackVer],
		Description: header.Metadata[MetaKeyPackDesc],
		Author:      header.Metadata[MetaKeyPackAuthor],
		Source:      header.Metadata[MetaKeyPackSource],
	}

	if tagsStr := header.Metadata[MetaKeyPackTags]; tagsStr != "" {
		manifest.Tags = strings.Split(tagsStr, ",")
	}

	return manifest
}
