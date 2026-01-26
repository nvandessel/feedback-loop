package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nvandessel/feedback-loop/internal/learning"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
	"github.com/spf13/cobra"
)

// behaviorsCache holds cached behaviors derived from corrections
type behaviorsCache struct {
	Version   string            `json:"version"`
	BuiltAt   time.Time         `json:"built_at"`
	Behaviors []models.Behavior `json:"behaviors"`
}

var version = "0.1.0-dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "floop",
		Short: "Feedback loop - behavior learning for AI agents",
		Long: `floop manages learned behaviors and conventions for AI coding agents.

It captures corrections, extracts reusable behaviors, and provides
context-aware behavior activation for consistent agent operation.`,
	}

	// Global flags
	rootCmd.PersistentFlags().Bool("json", false, "Output as JSON (for agent consumption)")
	rootCmd.PersistentFlags().String("root", ".", "Project root directory")

	// Add subcommands
	rootCmd.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newLearnCmd(),
		newListCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"version": version})
			} else {
				fmt.Printf("floop version %s\n", version)
			}
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize feedback loop tracking in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			floopDir := filepath.Join(root, ".floop")

			// Create .floop directory
			if err := os.MkdirAll(floopDir, 0755); err != nil {
				return fmt.Errorf("failed to create .floop directory: %w", err)
			}

			// Create manifest.yaml
			manifestPath := filepath.Join(floopDir, "manifest.yaml")
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				manifest := `# Feedback Loop Manifest
version: "1.0"
created: %s

# Behaviors learned from corrections are stored in this directory
# Run 'floop list' to see all behaviors
# Run 'floop active' to see behaviors active in current context
`
				content := fmt.Sprintf(manifest, time.Now().Format(time.RFC3339))
				if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to create manifest.yaml: %w", err)
				}
			}

			// Create corrections log for dogfooding
			correctionsPath := filepath.Join(floopDir, "corrections.jsonl")
			if _, err := os.Stat(correctionsPath); os.IsNotExist(err) {
				if err := os.WriteFile(correctionsPath, []byte{}, 0644); err != nil {
					return fmt.Errorf("failed to create corrections.jsonl: %w", err)
				}
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]string{
					"status": "initialized",
					"path":   floopDir,
				})
			} else {
				fmt.Printf("Initialized .floop/ in %s\n", root)
			}

			return nil
		},
	}
}

func newLearnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learn",
		Short: "Capture a correction and extract behavior",
		Long: `Capture a correction from a human-agent interaction and extract a behavior.

This command is called by agents when they receive a correction.
It records the correction, extracts a candidate behavior, and determines
whether the behavior can be auto-accepted or requires human review.

Example:
  floop learn --wrong "used os.path" --right "use pathlib.Path instead"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			wrong, _ := cmd.Flags().GetString("wrong")
			right, _ := cmd.Flags().GetString("right")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetString("task")
			root, _ := cmd.Flags().GetString("root")

			// Build context snapshot
			now := time.Now()
			ctxSnapshot := models.ContextSnapshot{
				Timestamp: now,
				FilePath:  file,
				Task:      task,
			}
			if file != "" {
				ctxSnapshot.FileLanguage = models.InferLanguage(file)
				ctxSnapshot.FileExt = filepath.Ext(file)
			}

			// Create correction using models.Correction
			correction := models.Correction{
				ID:              fmt.Sprintf("c-%d", now.UnixNano()),
				Timestamp:       now,
				Context:         ctxSnapshot,
				AgentAction:     wrong,
				CorrectedAction: right,
				Processed:       false,
			}

			// Ensure .floop exists
			floopDir := filepath.Join(root, ".floop")
			if _, err := os.Stat(floopDir); os.IsNotExist(err) {
				return fmt.Errorf(".floop not initialized. Run 'floop init' first")
			}

			// Append to corrections log
			correctionsPath := filepath.Join(floopDir, "corrections.jsonl")
			f, err := os.OpenFile(correctionsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open corrections log: %w", err)
			}
			defer f.Close()

			if err := json.NewEncoder(f).Encode(correction); err != nil {
				return fmt.Errorf("failed to write correction: %w", err)
			}

			// Invalidate behaviors cache (will be rebuilt on next read)
			if err := invalidateCache(floopDir); err != nil {
				return fmt.Errorf("failed to invalidate cache: %w", err)
			}

			// Process through learning loop
			graphStore := store.NewInMemoryGraphStore()
			loop := learning.NewLearningLoop(graphStore, nil)
			ctx := context.Background()

			result, err := loop.ProcessCorrection(ctx, correction)
			if err != nil {
				return fmt.Errorf("failed to process correction: %w", err)
			}

			// Mark correction as processed
			correction.Processed = true
			processedAt := time.Now()
			correction.ProcessedAt = &processedAt

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"status":          "processed",
					"correction":      correction,
					"behavior":        result.CandidateBehavior,
					"placement":       result.Placement,
					"auto_accepted":   result.AutoAccepted,
					"requires_review": result.RequiresReview,
					"review_reasons":  result.ReviewReasons,
				})
			} else {
				fmt.Println("Correction captured and processed:")
				fmt.Printf("  Wrong: %s\n", correction.AgentAction)
				fmt.Printf("  Right: %s\n", correction.CorrectedAction)
				if correction.Context.FilePath != "" {
					fmt.Printf("  File:  %s\n", correction.Context.FilePath)
				}
				if correction.Context.Task != "" {
					fmt.Printf("  Task:  %s\n", correction.Context.Task)
				}
				fmt.Println()
				fmt.Println("Extracted behavior:")
				fmt.Printf("  ID:   %s\n", result.CandidateBehavior.ID)
				fmt.Printf("  Name: %s\n", result.CandidateBehavior.Name)
				fmt.Printf("  Kind: %s\n", result.CandidateBehavior.Kind)
				fmt.Println()
				if result.AutoAccepted {
					fmt.Println("Status: Auto-accepted")
				} else if result.RequiresReview {
					fmt.Println("Status: Requires review")
					for _, reason := range result.ReviewReasons {
						fmt.Printf("  - %s\n", reason)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().String("wrong", "", "What the agent did (required)")
	cmd.Flags().String("right", "", "What should have been done (required)")
	cmd.Flags().String("file", "", "Current file path")
	cmd.Flags().String("task", "", "Current task type")
	cmd.MarkFlagRequired("wrong")
	cmd.MarkFlagRequired("right")

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List behaviors or corrections",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")
			showCorrections, _ := cmd.Flags().GetBool("corrections")

			floopDir := filepath.Join(root, ".floop")
			if _, err := os.Stat(floopDir); os.IsNotExist(err) {
				if jsonOut {
					json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
						"error": ".floop not initialized",
					})
				} else {
					fmt.Println("Not initialized. Run 'floop init' first.")
				}
				return nil
			}

			if showCorrections {
				return listCorrections(root, jsonOut)
			}

			// Load behaviors from cache (rebuilds if stale)
			behaviors, err := loadBehaviors(floopDir)
			if err != nil {
				return fmt.Errorf("failed to load behaviors: %w", err)
			}

			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"behaviors": behaviors,
					"count":     len(behaviors),
				})
			} else {
				if len(behaviors) == 0 {
					fmt.Println("No behaviors learned yet.")
					fmt.Println("\nUse 'floop learn --wrong \"X\" --right \"Y\"' to capture corrections.")
					return nil
				}
				fmt.Printf("Learned behaviors (%d):\n\n", len(behaviors))
				for i, b := range behaviors {
					fmt.Printf("%d. [%s] %s\n", i+1, b.Kind, b.Name)
					fmt.Printf("   %s\n", b.Content.Canonical)
					if len(b.When) > 0 {
						fmt.Printf("   When: %v\n", b.When)
					}
					fmt.Printf("   Confidence: %.2f\n", b.Confidence)
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().Bool("corrections", false, "Show captured corrections instead of behaviors")

	return cmd
}

func listCorrections(root string, jsonOut bool) error {
	correctionsPath := filepath.Join(root, ".floop", "corrections.jsonl")

	data, err := os.ReadFile(correctionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"corrections": []models.Correction{},
					"count":       0,
				})
			} else {
				fmt.Println("No corrections captured yet.")
			}
			return nil
		}
		return err
	}

	// Parse JSONL into models.Correction
	var corrections []models.Correction
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var c models.Correction
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		corrections = append(corrections, c)
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"corrections": corrections,
			"count":       len(corrections),
		})
	} else {
		if len(corrections) == 0 {
			fmt.Println("No corrections captured yet.")
			return nil
		}
		fmt.Printf("Captured corrections (%d):\n\n", len(corrections))
		for i, c := range corrections {
			fmt.Printf("%d. [%s]\n", i+1, c.Timestamp.Format(time.RFC3339))
			fmt.Printf("   Wrong: %s\n", c.AgentAction)
			fmt.Printf("   Right: %s\n", c.CorrectedAction)
			if c.Context.FilePath != "" {
				fmt.Printf("   File:  %s\n", c.Context.FilePath)
			}
			fmt.Println()
		}
	}

	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Cache file paths
const behaviorsCacheFile = "behaviors.json"

// isCacheStale checks if the behaviors cache needs to be rebuilt
func isCacheStale(floopDir string) bool {
	cachePath := filepath.Join(floopDir, behaviorsCacheFile)
	correctionsPath := filepath.Join(floopDir, "corrections.jsonl")

	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		return true // Cache doesn't exist
	}

	correctionsInfo, err := os.Stat(correctionsPath)
	if err != nil {
		return false // No corrections, cache is fine
	}

	// Cache is stale if corrections were modified after cache was built
	return correctionsInfo.ModTime().After(cacheInfo.ModTime())
}

// invalidateCache removes the behaviors cache file
func invalidateCache(floopDir string) error {
	cachePath := filepath.Join(floopDir, behaviorsCacheFile)
	err := os.Remove(cachePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// loadBehaviors loads behaviors from cache, rebuilding if stale
func loadBehaviors(floopDir string) ([]models.Behavior, error) {
	if !isCacheStale(floopDir) {
		// Load from cache
		cachePath := filepath.Join(floopDir, behaviorsCacheFile)
		data, err := os.ReadFile(cachePath)
		if err == nil {
			var cache behaviorsCache
			if err := json.Unmarshal(data, &cache); err == nil {
				return cache.Behaviors, nil
			}
		}
	}

	// Rebuild cache from corrections
	behaviors, err := rebuildBehaviorsCache(floopDir)
	if err != nil {
		return nil, err
	}

	return behaviors, nil
}

// rebuildBehaviorsCache processes all corrections and rebuilds the cache
func rebuildBehaviorsCache(floopDir string) ([]models.Behavior, error) {
	correctionsPath := filepath.Join(floopDir, "corrections.jsonl")

	data, err := os.ReadFile(correctionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No corrections, return empty and write empty cache
			if err := writeBehaviorsCache(floopDir, nil); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}

	// Parse corrections
	var corrections []models.Correction
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var c models.Correction
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		corrections = append(corrections, c)
	}

	if len(corrections) == 0 {
		if err := writeBehaviorsCache(floopDir, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Process corrections through learning loop
	graphStore := store.NewInMemoryGraphStore()
	loop := learning.NewLearningLoop(graphStore, nil)
	ctx := context.Background()

	var behaviors []models.Behavior
	for _, correction := range corrections {
		result, err := loop.ProcessCorrection(ctx, correction)
		if err != nil {
			continue // Skip failed corrections
		}
		behaviors = append(behaviors, result.CandidateBehavior)
	}

	// Write cache
	if err := writeBehaviorsCache(floopDir, behaviors); err != nil {
		return nil, err
	}

	return behaviors, nil
}

// writeBehaviorsCache writes behaviors to the cache file
func writeBehaviorsCache(floopDir string, behaviors []models.Behavior) error {
	cachePath := filepath.Join(floopDir, behaviorsCacheFile)

	if behaviors == nil {
		behaviors = []models.Behavior{}
	}

	cache := behaviorsCache{
		Version:   "1",
		BuiltAt:   time.Now(),
		Behaviors: behaviors,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}
