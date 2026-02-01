package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudePlatformName(t *testing.T) {
	p := NewClaudePlatform()
	if p.Name() != "Claude Code" {
		t.Errorf("expected 'Claude Code', got '%s'", p.Name())
	}
}

func TestClaudePlatformDetect(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewClaudePlatform()

	// No .claude directory
	if p.Detect(tmpDir) {
		t.Error("expected Detect=false when .claude doesn't exist")
	}

	// Create .claude as file (not directory)
	filePath := filepath.Join(tmpDir, ".claude")
	if err := os.WriteFile(filePath, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	if p.Detect(tmpDir) {
		t.Error("expected Detect=false when .claude is a file")
	}

	// Remove and create as directory
	os.Remove(filePath)
	if err := os.MkdirAll(filePath, 0755); err != nil {
		t.Fatal(err)
	}
	if !p.Detect(tmpDir) {
		t.Error("expected Detect=true when .claude is a directory")
	}
}

func TestClaudePlatformConfigPath(t *testing.T) {
	p := NewClaudePlatform()
	path := p.ConfigPath("/project")
	expected := filepath.Join("/project", ".claude", "settings.json")
	if path != expected {
		t.Errorf("expected '%s', got '%s'", expected, path)
	}
}

func TestClaudePlatformReadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewClaudePlatform()

	// No config file - should return nil
	config, err := p.ReadConfig(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if config != nil {
		t.Error("expected nil config when file doesn't exist")
	}

	// Create .claude directory and empty file
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty file - should return nil
	config, err = p.ReadConfig(tmpDir)
	if err != nil {
		t.Errorf("unexpected error for empty file: %v", err)
	}
	if config != nil {
		t.Error("expected nil config for empty file")
	}

	// Valid JSON
	validJSON := `{"key": "value"}`
	if err := os.WriteFile(configPath, []byte(validJSON), 0644); err != nil {
		t.Fatal(err)
	}
	config, err = p.ReadConfig(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if config == nil {
		t.Error("expected non-nil config")
	}
	if config["key"] != "value" {
		t.Errorf("expected key='value', got '%v'", config["key"])
	}

	// Invalid JSON
	if err := os.WriteFile(configPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = p.ReadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestClaudePlatformGenerateHookConfig(t *testing.T) {
	p := NewClaudePlatform()

	// From nil config
	config, err := p.GenerateHookConfig(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify structure
	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected hooks to be a map")
	}

	preToolUse, ok := hooks["PreToolUse"].([]interface{})
	if !ok {
		t.Fatal("expected PreToolUse to be an array")
	}

	if len(preToolUse) != 1 {
		t.Errorf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}

	// Verify hook entry
	entry := preToolUse[0].(map[string]interface{})
	if entry["matcher"] != "Read" {
		t.Errorf("expected matcher='Read', got '%v'", entry["matcher"])
	}

	hooksList := entry["hooks"].([]interface{})
	hook := hooksList[0].(map[string]interface{})
	if hook["type"] != "command" {
		t.Errorf("expected type='command', got '%v'", hook["type"])
	}
	if !strings.Contains(hook["command"].(string), "floop") {
		t.Errorf("expected command to contain 'floop', got '%v'", hook["command"])
	}
}

func TestClaudePlatformGenerateHookConfigMerge(t *testing.T) {
	p := NewClaudePlatform()

	// Existing config with other settings
	existing := map[string]interface{}{
		"otherSetting": "preserved",
		"hooks": map[string]interface{}{
			"OtherHook": []interface{}{"existing"},
		},
	}

	config, err := p.GenerateHookConfig(existing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Other setting should be preserved
	if config["otherSetting"] != "preserved" {
		t.Error("expected otherSetting to be preserved")
	}

	// Both hooks should exist
	hooks := config["hooks"].(map[string]interface{})
	if hooks["OtherHook"] == nil {
		t.Error("expected OtherHook to be preserved")
	}
	if hooks["PreToolUse"] == nil {
		t.Error("expected PreToolUse to be added")
	}
}

func TestClaudePlatformWriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewClaudePlatform()

	config := map[string]interface{}{
		"key": "value",
	}

	// Should create .claude directory and write file
	err := p.WriteConfig(tmpDir, config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify file exists and is valid JSON
	configPath := filepath.Join(tmpDir, ".claude", "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("failed to read config: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("config is not valid JSON: %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("expected key='value', got '%v'", parsed["key"])
	}
}

func TestClaudePlatformInjectCommand(t *testing.T) {
	p := NewClaudePlatform()
	cmd := p.InjectCommand()

	if !strings.Contains(cmd, "floop") {
		t.Error("expected command to contain 'floop'")
	}
	if !strings.Contains(cmd, "prompt") {
		t.Error("expected command to contain 'prompt'")
	}
}

func TestClaudePlatformHasFloopHook(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewClaudePlatform()

	// No config - should return false
	has, err := p.HasFloopHook(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if has {
		t.Error("expected false when no config exists")
	}

	// Create config without floop hooks
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	noFloopConfig := `{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Read",
					"hooks": [
						{"type": "command", "command": "other-tool"}
					]
				}
			]
		}
	}`
	configPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(configPath, []byte(noFloopConfig), 0644); err != nil {
		t.Fatal(err)
	}

	has, err = p.HasFloopHook(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if has {
		t.Error("expected false when floop hooks not present")
	}

	// Config with floop hooks
	floopConfig := `{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Read",
					"hooks": [
						{"type": "command", "command": "floop prompt --format markdown"}
					]
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(floopConfig), 0644); err != nil {
		t.Fatal(err)
	}

	has, err = p.HasFloopHook(tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !has {
		t.Error("expected true when floop hooks are present")
	}
}
