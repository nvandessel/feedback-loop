package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRootCmd creates a root command with persistent flags for testing subcommands
func newTestRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "floop",
	}
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
	rootCmd.PersistentFlags().String("root", ".", "Project root directory")
	return rootCmd
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", []string{}},
		{"single line no newline", "foo", []string{"foo"}},
		{"single line with newline", "foo\n", []string{"foo"}},
		{"multiple lines", "foo\nbar\nbaz", []string{"foo", "bar", "baz"}},
		{"multiple lines with trailing", "foo\nbar\n", []string{"foo", "bar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitLines(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNewVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
}

func TestNewInitCmd(t *testing.T) {
	cmd := newInitCmd()
	if cmd.Use != "init" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init")
	}
}

func TestNewLearnCmd(t *testing.T) {
	cmd := newLearnCmd()
	if cmd.Use != "learn" {
		t.Errorf("Use = %q, want %q", cmd.Use, "learn")
	}

	// Check required flags exist
	wrongFlag := cmd.Flags().Lookup("wrong")
	if wrongFlag == nil {
		t.Error("missing --wrong flag")
	}
	rightFlag := cmd.Flags().Lookup("right")
	if rightFlag == nil {
		t.Error("missing --right flag")
	}
}

func TestNewListCmd(t *testing.T) {
	cmd := newListCmd()
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}

	// Check corrections flag exists
	correctionsFlag := cmd.Flags().Lookup("corrections")
	if correctionsFlag == nil {
		t.Error("missing --corrections flag")
	}
}

func TestInitCmdCreatesDirectory(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Run init command with root command context
	rootCmd := newTestRootCmd()
	rootCmd.AddCommand(newInitCmd())
	rootCmd.SetArgs([]string{"init", "--root", tmpDir})
	rootCmd.SetOut(&bytes.Buffer{}) // Suppress output
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Verify .floop directory was created
	floopDir := filepath.Join(tmpDir, ".floop")
	if _, err := os.Stat(floopDir); os.IsNotExist(err) {
		t.Error(".floop directory not created")
	}

	// Verify manifest.yaml was created
	manifestPath := filepath.Join(floopDir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.yaml not created")
	}

	// Verify corrections.jsonl was created
	correctionsPath := filepath.Join(floopDir, "corrections.jsonl")
	if _, err := os.Stat(correctionsPath); os.IsNotExist(err) {
		t.Error("corrections.jsonl not created")
	}
}

func TestLearnCmdRequiresInit(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd := newTestRootCmd()
	rootCmd.AddCommand(newLearnCmd())
	rootCmd.SetArgs([]string{"learn", "--wrong", "test", "--right", "test", "--root", tmpDir})
	rootCmd.SetOut(&bytes.Buffer{})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when .floop not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("expected 'not initialized' error, got: %v", err)
	}
}

func TestLearnCmdCapturesCorrection(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize first
	rootCmd := newTestRootCmd()
	rootCmd.AddCommand(newInitCmd())
	rootCmd.SetArgs([]string{"init", "--root", tmpDir})
	rootCmd.SetOut(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Run learn command
	rootCmd2 := newTestRootCmd()
	rootCmd2.AddCommand(newLearnCmd())
	rootCmd2.SetArgs([]string{
		"learn",
		"--wrong", "used os.path",
		"--right", "use pathlib.Path",
		"--file", "script.py",
		"--task", "refactor",
		"--root", tmpDir,
	})
	rootCmd2.SetOut(&bytes.Buffer{})
	if err := rootCmd2.Execute(); err != nil {
		t.Fatalf("learn failed: %v", err)
	}

	// Verify correction was written
	correctionsPath := filepath.Join(tmpDir, ".floop", "corrections.jsonl")
	data, err := os.ReadFile(correctionsPath)
	if err != nil {
		t.Fatalf("failed to read corrections: %v", err)
	}

	var correction map[string]interface{}
	if err := json.Unmarshal(data, &correction); err != nil {
		t.Fatalf("failed to parse correction: %v", err)
	}

	if correction["agent_action"] != "used os.path" {
		t.Errorf("agent_action = %v, want %q", correction["agent_action"], "used os.path")
	}
	if correction["corrected_action"] != "use pathlib.Path" {
		t.Errorf("corrected_action = %v, want %q", correction["corrected_action"], "use pathlib.Path")
	}

	// Check context is present
	ctx, ok := correction["context"].(map[string]interface{})
	if !ok {
		t.Fatal("context not present or not a map")
	}
	if ctx["file_path"] != "script.py" {
		t.Errorf("context.file_path = %v, want %q", ctx["file_path"], "script.py")
	}
	if ctx["task"] != "refactor" {
		t.Errorf("context.task = %v, want %q", ctx["task"], "refactor")
	}
	if ctx["file_language"] != "python" {
		t.Errorf("context.file_language = %v, want %q", ctx["file_language"], "python")
	}
}

func TestListCorrectionsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize
	rootCmd := newTestRootCmd()
	rootCmd.AddCommand(newInitCmd())
	rootCmd.SetArgs([]string{"init", "--root", tmpDir})
	rootCmd.SetOut(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// List should succeed with empty results
	err := listCorrections(tmpDir, false)
	if err != nil {
		t.Fatalf("listCorrections failed: %v", err)
	}
}

func TestListCorrectionsNotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// List should succeed gracefully
	err := listCorrections(tmpDir, false)
	if err != nil {
		t.Fatalf("listCorrections failed: %v", err)
	}
}
