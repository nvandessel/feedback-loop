package activation

import (
	"testing"
)

func TestContextBuilder_Build(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ContextBuilder)
		wantFile string
		wantLang string
		wantTask string
	}{
		{
			name: "empty builder",
			setup: func(b *ContextBuilder) {
				// no setup
			},
			wantFile: "",
			wantLang: "",
			wantTask: "",
		},
		{
			name: "with go file",
			setup: func(b *ContextBuilder) {
				b.WithFile("internal/models/behavior.go")
			},
			wantFile: "internal/models/behavior.go",
			wantLang: "go",
			wantTask: "",
		},
		{
			name: "with python file",
			setup: func(b *ContextBuilder) {
				b.WithFile("scripts/deploy.py")
			},
			wantFile: "scripts/deploy.py",
			wantLang: "python",
			wantTask: "",
		},
		{
			name: "with task",
			setup: func(b *ContextBuilder) {
				b.WithTask("refactor")
			},
			wantFile: "",
			wantLang: "",
			wantTask: "refactor",
		},
		{
			name: "with file and task",
			setup: func(b *ContextBuilder) {
				b.WithFile("main.go").WithTask("debug")
			},
			wantFile: "main.go",
			wantLang: "go",
			wantTask: "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewContextBuilder()
			tt.setup(builder)
			ctx := builder.Build()

			if ctx.FilePath != tt.wantFile {
				t.Errorf("FilePath = %v, want %v", ctx.FilePath, tt.wantFile)
			}
			if ctx.FileLanguage != tt.wantLang {
				t.Errorf("FileLanguage = %v, want %v", ctx.FileLanguage, tt.wantLang)
			}
			if ctx.Task != tt.wantTask {
				t.Errorf("Task = %v, want %v", ctx.Task, tt.wantTask)
			}
		})
	}
}

func TestContextBuilder_WithCustom(t *testing.T) {
	builder := NewContextBuilder()
	builder.WithCustom("project_type", "cli")
	builder.WithCustom("team", "platform")

	ctx := builder.Build()

	if ctx.Custom["project_type"] != "cli" {
		t.Errorf("Custom[project_type] = %v, want cli", ctx.Custom["project_type"])
	}
	if ctx.Custom["team"] != "platform" {
		t.Errorf("Custom[team] = %v, want platform", ctx.Custom["team"])
	}
}

func TestContextBuilder_Chaining(t *testing.T) {
	ctx := NewContextBuilder().
		WithFile("src/main.go").
		WithTask("implement").
		WithEnvironment("dev").
		WithRepoRoot("/tmp/test").
		WithCustom("priority", "high").
		Build()

	if ctx.FilePath != "src/main.go" {
		t.Errorf("FilePath = %v, want src/main.go", ctx.FilePath)
	}
	if ctx.Task != "implement" {
		t.Errorf("Task = %v, want implement", ctx.Task)
	}
	if ctx.Environment != "dev" {
		t.Errorf("Environment = %v, want dev", ctx.Environment)
	}
	if ctx.Custom["priority"] != "high" {
		t.Errorf("Custom[priority] = %v, want high", ctx.Custom["priority"])
	}
}
