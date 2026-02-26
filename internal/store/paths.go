// Package store provides graph storage implementations.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// GlobalFloopPath returns the path to the global .floop directory.
// On Unix: ~/.floop
// On Windows: %USERPROFILE%\.floop
func GlobalFloopPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".floop"), nil
}

// LocalFloopPath returns the path to the local .floop directory
// for the given project root.
func LocalFloopPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".floop")
}

// EnsureGlobalFloopDir creates the global .floop directory if it doesn't exist.
// Returns nil if the directory already exists or was successfully created.
func EnsureGlobalFloopDir() error {
	globalPath, err := GlobalFloopPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(globalPath, 0700); err != nil {
		return fmt.Errorf("failed to create global .floop directory: %w", err)
	}

	return nil
}

// floopGitignore is the default .gitignore content for .floop directories.
const floopGitignore = `# SQLite database files (source of truth is JSONL)
floop.db
floop.db-shm
floop.db-wal

# Audit logs (runtime data, not version controlled)
audit.jsonl
`

// EnsureGitignore creates a .gitignore in the given .floop directory if one
// does not already exist. This prevents accidentally committing database files
// and runtime artifacts to version control.
func EnsureGitignore(floopDir string) error {
	gitignorePath := filepath.Join(floopDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		return nil // already exists, respect user customizations
	}
	if err := os.WriteFile(gitignorePath, []byte(floopGitignore), 0600); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}
	return nil
}
