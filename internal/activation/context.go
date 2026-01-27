package activation

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/nvandessel/feedback-loop/internal/models"
)

// ContextBuilder gathers context from the environment for activation evaluation
type ContextBuilder struct {
	// Override values (from CLI flags)
	FilePath    string
	Task        string
	Environment string
	RepoRoot    string

	// Additional custom values
	Custom map[string]interface{}
}

// NewContextBuilder creates a new context builder
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		Custom: make(map[string]interface{}),
	}
}

// WithFile sets the file path for context
func (b *ContextBuilder) WithFile(path string) *ContextBuilder {
	b.FilePath = path
	return b
}

// WithTask sets the current task type
func (b *ContextBuilder) WithTask(task string) *ContextBuilder {
	b.Task = task
	return b
}

// WithEnvironment sets the environment (dev, staging, prod)
func (b *ContextBuilder) WithEnvironment(env string) *ContextBuilder {
	b.Environment = env
	return b
}

// WithRepoRoot sets the repository root path
func (b *ContextBuilder) WithRepoRoot(root string) *ContextBuilder {
	b.RepoRoot = root
	return b
}

// WithCustom adds a custom context field
func (b *ContextBuilder) WithCustom(key string, value interface{}) *ContextBuilder {
	b.Custom[key] = value
	return b
}

// Build creates a ContextSnapshot from the current environment
func (b *ContextBuilder) Build() models.ContextSnapshot {
	ctx := models.ContextSnapshot{
		Timestamp: time.Now(),
		Custom:    b.Custom,
	}

	// Set file info
	if b.FilePath != "" {
		ctx.FilePath = b.FilePath
		ctx.FileLanguage = models.InferLanguage(b.FilePath)
		ctx.FileExt = filepath.Ext(b.FilePath)
	}

	// Set task
	if b.Task != "" {
		ctx.Task = b.Task
	}

	// Set environment
	if b.Environment != "" {
		ctx.Environment = b.Environment
	} else if env := os.Getenv("FLOOP_ENV"); env != "" {
		ctx.Environment = env
	}

	// Get git info
	repoRoot := b.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	ctx.RepoRoot = repoRoot
	ctx.Repo = getGitRemote(repoRoot)
	ctx.Branch = getGitBranch(repoRoot)

	// Get user info
	if u, err := user.Current(); err == nil {
		ctx.User = u.Username
	}

	return ctx
}

// getGitRemote returns the git remote URL
func getGitRemote(repoRoot string) string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getGitBranch returns the current git branch
func getGitBranch(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
