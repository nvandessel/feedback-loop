// Package pack defines the skill pack format, pack ID scheme, and provenance model.
// A skill pack (.fpack) is a thin wrapper over the backup V2 format with pack
// metadata stamped into the BackupHeader.Metadata map.
package pack

import (
	"fmt"
	"regexp"
)

// packIDPattern validates namespace/name format: lowercase alphanumeric and hyphens.
var packIDPattern = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9-]+$`)

// PackID is a validated pack identifier in "namespace/name" format.
type PackID string

// ValidatePackID checks if a string is a valid pack ID.
func ValidatePackID(id string) error {
	if !packIDPattern.MatchString(id) {
		return fmt.Errorf("invalid pack ID %q: must match namespace/name pattern ([a-z0-9-]+/[a-z0-9-]+)", id)
	}
	return nil
}

// PackManifest describes a skill pack.
type PackManifest struct {
	ID          PackID   `json:"id" yaml:"id"`
	Version     string   `json:"version" yaml:"version"`
	Description string   `json:"description" yaml:"description"`
	Author      string   `json:"author,omitempty" yaml:"author,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Source      string   `json:"source,omitempty" yaml:"source,omitempty"`
}

// Metadata key constants for stamping into BackupHeader.Metadata when writing pack files.
const (
	MetaKeyType       = "type"    // value: "skill-pack"
	MetaKeyPackID     = "pack_id" // value: "namespace/name"
	MetaKeyPackVer    = "pack_version"
	MetaKeyPackAuthor = "pack_author"
	MetaKeyPackDesc   = "pack_description"
	MetaKeyPackTags   = "pack_tags" // comma-separated
	MetaKeyPackSource = "pack_source"
)

// PackFileType is the metadata type value that identifies a file as a skill pack.
const PackFileType = "skill-pack"
