package consolidation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nvandessel/floop/internal/logging"
)

func TestLogDecision_ExceedsMaxFields(t *testing.T) {
	dir := t.TempDir()
	dl := logging.NewDecisionLogger(dir, "debug")
	defer dl.Close()

	c := &LLMConsolidator{
		decisions: dl,
		runID:     "test-run",
		config:    LLMConsolidatorConfig{Model: "test-model"},
	}

	// Build a map that exceeds the adjusted limit (maxFields - 2 = 9998).
	// 9999 fields should be rejected because logDecision adds 2 more fields
	// (run_id, model), which would push the total to 10001.
	huge := make(map[string]any, 9_999)
	for i := range 9_999 {
		huge[fmt.Sprintf("key_%d", i)] = i
	}

	c.logDecision(huge)

	dl.Close()

	path := filepath.Join(dir, "decisions.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading decisions.jsonl: %v", err)
	}
	if len(strings.TrimSpace(string(data))) > 0 {
		t.Error("expected no entries written for oversized fields map")
	}
}

func TestLogDecision_AtLimit(t *testing.T) {
	dir := t.TempDir()
	dl := logging.NewDecisionLogger(dir, "debug")
	defer dl.Close()

	c := &LLMConsolidator{
		decisions: dl,
		runID:     "test-run",
		config:    LLMConsolidatorConfig{Model: "test-model"},
	}

	// 9998 fields + 2 injected = 10000, which is exactly at the downstream limit.
	fields := make(map[string]any, 9_998)
	for i := range 9_998 {
		fields[fmt.Sprintf("key_%d", i)] = i
	}

	c.logDecision(fields)

	dl.Close()

	path := filepath.Join(dir, "decisions.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading decisions.jsonl: %v", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Error("expected entry to be written when fields are at the limit")
	}
}
