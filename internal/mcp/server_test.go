package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Initialize .floop directory
	floopDir := filepath.Join(tmpDir, ".floop")
	if err := os.MkdirAll(floopDir, 0755); err != nil {
		t.Fatalf("Failed to create .floop dir: %v", err)
	}

	// Create server
	cfg := &Config{
		Name:    "test-server",
		Version: "v1.0.0",
		Root:    tmpDir,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Close()

	if server.server == nil {
		t.Error("Server.server is nil")
	}

	if server.store == nil {
		t.Error("Server.store is nil")
	}

	if server.root != tmpDir {
		t.Errorf("Server.root = %q, want %q", server.root, tmpDir)
	}
}

func TestNewServer_CreatesFloopDir(t *testing.T) {
	// Create temp directory WITHOUT .floop
	tmpDir := t.TempDir()

	cfg := &Config{
		Name:    "test-server",
		Version: "v1.0.0",
		Root:    tmpDir,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Close()

	// Verify .floop directory was created
	floopDir := filepath.Join(tmpDir, ".floop")
	if _, err := os.Stat(floopDir); os.IsNotExist(err) {
		t.Error(".floop directory was not created")
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	floopDir := filepath.Join(tmpDir, ".floop")
	if err := os.MkdirAll(floopDir, 0755); err != nil {
		t.Fatalf("Failed to create .floop dir: %v", err)
	}

	cfg := &Config{
		Name:    "test-server",
		Version: "v1.0.0",
		Root:    tmpDir,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Close should not error
	if err := server.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Multiple closes should be safe
	if err := server.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestRun_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	floopDir := filepath.Join(tmpDir, ".floop")
	if err := os.MkdirAll(floopDir, 0755); err != nil {
		t.Fatalf("Failed to create .floop dir: %v", err)
	}

	cfg := &Config{
		Name:    "test-server",
		Version: "v1.0.0",
		Root:    tmpDir,
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Close()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run should return quickly with cancelled context
	err = server.Run(ctx)
	// We expect an error since stdio transport won't work in test
	// but we're just verifying it doesn't hang
	if err == nil {
		t.Log("Run returned nil (expected in test environment)")
	}
}
