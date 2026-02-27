package llm

import (
	"strings"
	"testing"

	"github.com/nvandessel/floop/internal/models"
)

func TestComparisonPrompt(t *testing.T) {
	a := &models.Behavior{
		ID:      "b1",
		Name:    "Use pathlib",
		Kind:    models.BehaviorKindDirective,
		Content: models.BehaviorContent{Canonical: "Use pathlib.Path instead of os.path"},
	}
	b := &models.Behavior{
		ID:      "b2",
		Name:    "Prefer pathlib",
		Kind:    models.BehaviorKindPreference,
		Content: models.BehaviorContent{Canonical: "Prefer pathlib over os.path for file paths"},
	}

	prompt := ComparisonPrompt(a, b)

	// Check that all behavior details are included
	if !strings.Contains(prompt, "b1") {
		t.Error("prompt should contain behavior A ID")
	}
	if !strings.Contains(prompt, "b2") {
		t.Error("prompt should contain behavior B ID")
	}
	if !strings.Contains(prompt, "Use pathlib") {
		t.Error("prompt should contain behavior A name")
	}
	if !strings.Contains(prompt, "Prefer pathlib") {
		t.Error("prompt should contain behavior B name")
	}
	if !strings.Contains(prompt, "pathlib.Path instead of os.path") {
		t.Error("prompt should contain behavior A content")
	}
	if !strings.Contains(prompt, "semantic_similarity") {
		t.Error("prompt should mention semantic_similarity in expected format")
	}
	if !strings.Contains(prompt, "intent_match") {
		t.Error("prompt should mention intent_match in expected format")
	}
}

func TestMergePrompt(t *testing.T) {
	t.Run("empty input returns empty string", func(t *testing.T) {
		prompt := MergePrompt([]*models.Behavior{})
		if prompt != "" {
			t.Error("expected empty prompt for empty input")
		}
	})

	t.Run("single behavior", func(t *testing.T) {
		b := &models.Behavior{
			ID:         "b1",
			Name:       "Test behavior",
			Kind:       models.BehaviorKindDirective,
			Content:    models.BehaviorContent{Canonical: "Do the thing"},
			Priority:   5,
			Confidence: 0.9,
		}

		prompt := MergePrompt([]*models.Behavior{b})

		if !strings.Contains(prompt, "b1") {
			t.Error("prompt should contain behavior ID")
		}
		if !strings.Contains(prompt, "Test behavior") {
			t.Error("prompt should contain behavior name")
		}
		if !strings.Contains(prompt, "Priority: 5") {
			t.Error("prompt should contain priority")
		}
		if !strings.Contains(prompt, "Confidence: 0.90") {
			t.Error("prompt should contain confidence")
		}
	})

	t.Run("multiple behaviors", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{ID: "b1", Name: "First", Content: models.BehaviorContent{Canonical: "first content"}},
			{ID: "b2", Name: "Second", Content: models.BehaviorContent{Canonical: "second content"}},
		}

		prompt := MergePrompt(behaviors)

		if !strings.Contains(prompt, "b1") || !strings.Contains(prompt, "b2") {
			t.Error("prompt should contain all behavior IDs")
		}
		if !strings.Contains(prompt, `["b1","b2"]`) {
			t.Error("prompt should contain source_ids array")
		}
	})

	t.Run("behavior content with double quotes does not corrupt prompt", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{
				ID:   "b1",
				Name: `Use "pathlib" library`,
				Kind: models.BehaviorKindDirective,
				Content: models.BehaviorContent{
					Canonical: `Always use "pathlib.Path" instead of "os.path"`,
				},
			},
		}

		prompt := MergePrompt(behaviors)

		// Verify behavior content with quotes is preserved
		if !strings.Contains(prompt, `Use "pathlib" library`) {
			t.Error("prompt should contain behavior name with quotes")
		}
		if !strings.Contains(prompt, `Always use "pathlib.Path" instead of "os.path"`) {
			t.Error("prompt should contain behavior content with quotes")
		}
		// Verify JSON template structure is intact
		if !strings.Contains(prompt, `"source_ids": ["b1"]`) {
			t.Error("prompt should contain properly formatted source_ids")
		}
	})
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw JSON object",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "raw JSON array",
			input: `[1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "JSON in markdown code block with language",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in generic markdown code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "text without JSON",
			input: "This is just some text",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "JSON with surrounding whitespace",
			input: "  \n  {\"key\": \"value\"}  \n  ",
			want:  `{"key": "value"}`,
		},
		{
			name:  "markdown block with extra whitespace",
			input: "```json\n\n  {\"key\": \"value\"}  \n\n```",
			want:  `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJSON(tt.input)
			if got != tt.want {
				t.Errorf("ExtractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseComparisonResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		response := `{
			"semantic_similarity": 0.85,
			"intent_match": true,
			"merge_candidate": true,
			"reasoning": "Both behaviors address file path handling"
		}`

		result, err := ParseComparisonResponse(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.SemanticSimilarity != 0.85 {
			t.Errorf("SemanticSimilarity = %v, want 0.85", result.SemanticSimilarity)
		}
		if !result.IntentMatch {
			t.Error("IntentMatch should be true")
		}
		if !result.MergeCandidate {
			t.Error("MergeCandidate should be true")
		}
		if result.Reasoning == "" {
			t.Error("Reasoning should not be empty")
		}
	})

	t.Run("response in markdown block", func(t *testing.T) {
		response := "```json\n{\"semantic_similarity\": 0.5, \"intent_match\": false, \"merge_candidate\": false}\n```"

		result, err := ParseComparisonResponse(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.SemanticSimilarity != 0.5 {
			t.Errorf("SemanticSimilarity = %v, want 0.5", result.SemanticSimilarity)
		}
	})

	t.Run("similarity out of range - too high", func(t *testing.T) {
		response := `{"semantic_similarity": 1.5, "intent_match": true, "merge_candidate": true}`

		_, err := ParseComparisonResponse(response)
		if err == nil {
			t.Error("expected error for similarity > 1.0")
		}
	})

	t.Run("similarity out of range - negative", func(t *testing.T) {
		response := `{"semantic_similarity": -0.5, "intent_match": true, "merge_candidate": true}`

		_, err := ParseComparisonResponse(response)
		if err == nil {
			t.Error("expected error for negative similarity")
		}
	})

	t.Run("no JSON in response", func(t *testing.T) {
		response := "This is just text with no JSON"

		_, err := ParseComparisonResponse(response)
		if err == nil {
			t.Error("expected error for response without JSON")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		response := `{invalid json}`

		_, err := ParseComparisonResponse(response)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestParseMergeResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		response := `{
			"merged": {
				"name": "Use pathlib for paths",
				"kind": "directive",
				"content": {
					"canonical": "Use pathlib.Path for all file path operations"
				},
				"priority": 5,
				"confidence": 0.85
			},
			"source_ids": ["b1", "b2"],
			"reasoning": "Merged two similar pathlib behaviors"
		}`

		result, err := ParseMergeResponse(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Merged == nil {
			t.Fatal("Merged should not be nil")
		}
		if result.Merged.Name != "Use pathlib for paths" {
			t.Errorf("Name = %q, want %q", result.Merged.Name, "Use pathlib for paths")
		}
		if result.Merged.Kind != models.BehaviorKind("directive") {
			t.Errorf("Kind = %q, want directive", result.Merged.Kind)
		}
		if result.Merged.Content.Canonical != "Use pathlib.Path for all file path operations" {
			t.Errorf("unexpected canonical content: %q", result.Merged.Content.Canonical)
		}
		if result.Merged.Priority != 5 {
			t.Errorf("Priority = %d, want 5", result.Merged.Priority)
		}
		if result.Merged.Confidence != 0.85 {
			t.Errorf("Confidence = %v, want 0.85", result.Merged.Confidence)
		}
		if len(result.SourceIDs) != 2 {
			t.Errorf("SourceIDs length = %d, want 2", len(result.SourceIDs))
		}
	})

	t.Run("response in markdown block", func(t *testing.T) {
		response := "```json\n{\"merged\": {\"name\": \"Test\", \"kind\": \"directive\", \"content\": {\"canonical\": \"test content\"}}, \"source_ids\": [\"b1\"]}\n```"

		result, err := ParseMergeResponse(response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Merged.Name != "Test" {
			t.Errorf("Name = %q, want Test", result.Merged.Name)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		response := `{"merged": {"kind": "directive", "content": {"canonical": "test"}}, "source_ids": ["b1"]}`

		_, err := ParseMergeResponse(response)
		if err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("missing content", func(t *testing.T) {
		response := `{"merged": {"name": "Test", "kind": "directive", "content": {}}, "source_ids": ["b1"]}`

		_, err := ParseMergeResponse(response)
		if err == nil {
			t.Error("expected error for missing content")
		}
	})

	t.Run("missing source_ids", func(t *testing.T) {
		response := `{"merged": {"name": "Test", "kind": "directive", "content": {"canonical": "test"}}}`

		_, err := ParseMergeResponse(response)
		if err == nil {
			t.Error("expected error for missing source_ids")
		}
	})

	t.Run("empty source_ids", func(t *testing.T) {
		response := `{"merged": {"name": "Test", "kind": "directive", "content": {"canonical": "test"}}, "source_ids": []}`

		_, err := ParseMergeResponse(response)
		if err == nil {
			t.Error("expected error for empty source_ids")
		}
	})

	t.Run("no JSON in response", func(t *testing.T) {
		response := "No JSON here"

		_, err := ParseMergeResponse(response)
		if err == nil {
			t.Error("expected error for response without JSON")
		}
	})
}

func TestToJSONArray(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  "[]",
		},
		{
			name:  "single item",
			input: []string{"a"},
			want:  `["a"]`,
		},
		{
			name:  "multiple items",
			input: []string{"a", "b", "c"},
			want:  `["a","b","c"]`,
		},
		{
			name:  "items with special characters",
			input: []string{"hello world", "foo\"bar"},
			want:  `["hello world","foo\"bar"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSONArray(tt.input)
			if got != tt.want {
				t.Errorf("toJSONArray() = %q, want %q", got, tt.want)
			}
		})
	}
}
