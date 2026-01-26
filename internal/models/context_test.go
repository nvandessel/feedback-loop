package models

import (
	"testing"
)

func TestContextSnapshot_Matches(t *testing.T) {
	tests := []struct {
		name      string
		context   ContextSnapshot
		predicate map[string]interface{}
		want      bool
	}{
		{
			name: "exact match on language",
			context: ContextSnapshot{
				FileLanguage: "go",
			},
			predicate: map[string]interface{}{
				"language": "go",
			},
			want: true,
		},
		{
			name: "no match on language",
			context: ContextSnapshot{
				FileLanguage: "python",
			},
			predicate: map[string]interface{}{
				"language": "go",
			},
			want: false,
		},
		{
			name: "match with multiple conditions",
			context: ContextSnapshot{
				FileLanguage: "go",
				Task:         "refactor",
				Environment:  "dev",
			},
			predicate: map[string]interface{}{
				"language": "go",
				"task":     "refactor",
			},
			want: true,
		},
		{
			name: "partial match fails - all conditions required",
			context: ContextSnapshot{
				FileLanguage: "go",
				Task:         "write",
			},
			predicate: map[string]interface{}{
				"language": "go",
				"task":     "refactor",
			},
			want: false,
		},
		{
			name: "glob pattern match - filename only",
			context: ContextSnapshot{
				FilePath: "behavior.go",
			},
			predicate: map[string]interface{}{
				"file_path": "*.go",
			},
			want: true,
		},
		{
			name: "glob pattern no match",
			context: ContextSnapshot{
				FilePath: "behavior.go",
			},
			predicate: map[string]interface{}{
				"file_path": "*.py",
			},
			want: false,
		},
		{
			name: "glob pattern - full path doesn't match simple glob",
			context: ContextSnapshot{
				FilePath: "internal/models/behavior.go",
			},
			predicate: map[string]interface{}{
				"file_path": "*.go",
			},
			want: false, // filepath.Match("*.go", "internal/models/behavior.go") = false
		},
		{
			name: "array membership match",
			context: ContextSnapshot{
				Task: "refactor",
			},
			predicate: map[string]interface{}{
				"task": []interface{}{"write", "refactor", "review"},
			},
			want: true,
		},
		{
			name: "array membership no match",
			context: ContextSnapshot{
				Task: "deploy",
			},
			predicate: map[string]interface{}{
				"task": []interface{}{"write", "refactor", "review"},
			},
			want: false,
		},
		{
			name: "string array membership match",
			context: ContextSnapshot{
				Task: "refactor",
			},
			predicate: map[string]interface{}{
				"task": []string{"write", "refactor", "review"},
			},
			want: true,
		},
		{
			name: "empty predicate matches everything",
			context: ContextSnapshot{
				FileLanguage: "go",
				Task:         "write",
			},
			predicate: map[string]interface{}{},
			want:      true,
		},
		{
			name:    "nil actual value fails",
			context: ContextSnapshot{
				// Task is empty
			},
			predicate: map[string]interface{}{
				"task": "write",
			},
			want: false,
		},
		{
			name: "alternate field name - env",
			context: ContextSnapshot{
				Environment: "prod",
			},
			predicate: map[string]interface{}{
				"env": "prod",
			},
			want: true,
		},
		{
			name: "custom field match",
			context: ContextSnapshot{
				Custom: map[string]interface{}{
					"team": "backend",
				},
			},
			predicate: map[string]interface{}{
				"team": "backend",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.context.Matches(tt.predicate)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInferLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		want     string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.ts", "typescript"},
		{"lib.rs", "rust"},
		{"app.rb", "ruby"},
		{"Main.java", "java"},
		{"main.c", "c"},
		{"header.h", "c"},
		{"main.cpp", "cpp"},
		{"main.cc", "cpp"},
		{"header.hpp", "cpp"},
		{"README.md", "markdown"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.json", "json"},
		{"unknown.xyz", ""},
		{"noextension", ""},
		{"path/to/file.go", "go"},
		{"FILE.GO", "go"}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := InferLanguage(tt.filePath)
			if got != tt.want {
				t.Errorf("InferLanguage(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestMatchValue(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		required interface{}
		want     bool
	}{
		{"nil actual", nil, "value", false},
		{"exact string match", "hello", "hello", true},
		{"string mismatch", "hello", "world", false},
		{"glob match star", "test.go", "*.go", true},
		{"glob no match", "test.go", "*.py", false},
		{"non-string actual with string required", 123, "123", false},
		{"interface array match", "b", []interface{}{"a", "b", "c"}, true},
		{"interface array no match", "d", []interface{}{"a", "b", "c"}, false},
		{"string array match", "b", []string{"a", "b", "c"}, true},
		{"string array no match", "d", []string{"a", "b", "c"}, false},
		{"non-string actual with array", 123, []interface{}{"a", "b"}, false},
		{"equal non-string values", 42, 42, true},
		{"unequal non-string values", 42, 43, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchValue(tt.actual, tt.required)
			if got != tt.want {
				t.Errorf("matchValue(%v, %v) = %v, want %v", tt.actual, tt.required, got, tt.want)
			}
		})
	}
}
