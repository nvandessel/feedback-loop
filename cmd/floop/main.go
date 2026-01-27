package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nvandessel/feedback-loop/internal/activation"
	"github.com/nvandessel/feedback-loop/internal/learning"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
	"github.com/spf13/cobra"
)

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
		newActiveCmd(),
		newShowCmd(),
		newWhyCmd(),
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

			// Use persistent graph store
			graphStore, err := store.NewBeadsGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open graph store: %w", err)
			}
			defer graphStore.Close()

			// Process through learning loop
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

// loadBehaviors loads behaviors from the persistent graph store.
func loadBehaviors(floopDir string) ([]models.Behavior, error) {
	// Get the project root from the floop directory
	projectRoot := filepath.Dir(floopDir)

	// Open the graph store
	graphStore, err := store.NewBeadsGraphStore(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open graph store: %w", err)
	}
	defer graphStore.Close()

	// Query all behavior nodes
	ctx := context.Background()
	nodes, err := graphStore.QueryNodes(ctx, map[string]interface{}{"kind": "behavior"})
	if err != nil {
		return nil, fmt.Errorf("failed to query behaviors: %w", err)
	}

	// Convert nodes to behaviors
	behaviors := make([]models.Behavior, 0, len(nodes))
	for _, node := range nodes {
		b := learning.NodeToBehavior(node)
		behaviors = append(behaviors, b)
	}

	return behaviors, nil
}

func newActiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "active",
		Short: "Show behaviors active in current context",
		Long: `List all behaviors that are currently active based on the
current context (file, task, language, etc.).

Use --json for machine-readable output suitable for agent consumption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetString("task")
			env, _ := cmd.Flags().GetString("env")
			jsonOut, _ := cmd.Flags().GetBool("json")

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

			// Load all behaviors
			behaviors, err := loadBehaviors(floopDir)
			if err != nil {
				return fmt.Errorf("failed to load behaviors: %w", err)
			}

			// Build context
			ctxBuilder := activation.NewContextBuilder().
				WithFile(file).
				WithTask(task).
				WithEnvironment(env).
				WithRepoRoot(root)
			ctx := ctxBuilder.Build()

			// Evaluate which behaviors are active
			evaluator := activation.NewEvaluator()
			matches := evaluator.Evaluate(ctx, behaviors)

			// Resolve conflicts
			resolver := activation.NewResolver()
			result := resolver.Resolve(matches)

			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"context":    ctx,
					"active":     result.Active,
					"overridden": result.Overridden,
					"excluded":   result.Excluded,
					"count":      len(result.Active),
				})
			} else {
				fmt.Printf("Context:\n")
				if ctx.FilePath != "" {
					fmt.Printf("  File: %s\n", ctx.FilePath)
				}
				if ctx.FileLanguage != "" {
					fmt.Printf("  Language: %s\n", ctx.FileLanguage)
				}
				if ctx.Task != "" {
					fmt.Printf("  Task: %s\n", ctx.Task)
				}
				if ctx.Branch != "" {
					fmt.Printf("  Branch: %s\n", ctx.Branch)
				}
				fmt.Println()

				if len(result.Active) == 0 {
					fmt.Println("No active behaviors for this context.")
					if len(behaviors) > 0 {
						fmt.Printf("\n(%d behaviors exist but none match current context)\n", len(behaviors))
					}
					return nil
				}

				fmt.Printf("Active behaviors (%d):\n\n", len(result.Active))
				for i, b := range result.Active {
					fmt.Printf("%d. [%s] %s\n", i+1, b.Kind, b.Name)
					fmt.Printf("   %s\n", b.Content.Canonical)
					if len(b.When) > 0 {
						fmt.Printf("   When: %v\n", b.When)
					}
					fmt.Println()
				}

				if len(result.Overridden) > 0 {
					fmt.Printf("Overridden behaviors (%d):\n", len(result.Overridden))
					for _, o := range result.Overridden {
						fmt.Printf("  - %s (by %s)\n", o.Behavior.Name, o.OverrideBy)
					}
					fmt.Println()
				}

				if len(result.Excluded) > 0 {
					fmt.Printf("Excluded due to conflicts (%d):\n", len(result.Excluded))
					for _, e := range result.Excluded {
						fmt.Printf("  - %s (conflicts with %s)\n", e.Behavior.Name, e.ConflictsWith)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().String("file", "", "Current file path")
	cmd.Flags().String("task", "", "Current task type")
	cmd.Flags().String("env", "", "Environment (dev, staging, prod)")

	return cmd
}

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [behavior-id]",
		Short: "Show details of a behavior",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")
			id := args[0]

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

			// Load all behaviors
			behaviors, err := loadBehaviors(floopDir)
			if err != nil {
				return fmt.Errorf("failed to load behaviors: %w", err)
			}

			// Find the behavior
			var found *models.Behavior
			for _, b := range behaviors {
				if b.ID == id || b.Name == id {
					found = &b
					break
				}
			}

			if found == nil {
				if jsonOut {
					json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
						"error": "behavior not found",
						"id":    id,
					})
				} else {
					fmt.Printf("Behavior not found: %s\n", id)
				}
				return nil
			}

			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(found)
			} else {
				fmt.Printf("Behavior: %s\n", found.ID)
				fmt.Printf("Name: %s\n", found.Name)
				fmt.Printf("Kind: %s\n", found.Kind)
				fmt.Printf("Confidence: %.2f\n", found.Confidence)
				fmt.Printf("Priority: %d\n", found.Priority)
				fmt.Println()

				fmt.Println("Content:")
				fmt.Printf("  Canonical: %s\n", found.Content.Canonical)
				if found.Content.Expanded != "" {
					fmt.Printf("  Expanded: %s\n", found.Content.Expanded)
				}
				if len(found.Content.Structured) > 0 {
					fmt.Printf("  Structured: %v\n", found.Content.Structured)
				}
				fmt.Println()

				if len(found.When) > 0 {
					fmt.Println("Activation conditions:")
					for k, v := range found.When {
						fmt.Printf("  %s: %v\n", k, v)
					}
					fmt.Println()
				}

				fmt.Println("Provenance:")
				fmt.Printf("  Source: %s\n", found.Provenance.SourceType)
				fmt.Printf("  Created: %s\n", found.Provenance.CreatedAt.Format(time.RFC3339))
				if found.Provenance.CorrectionID != "" {
					fmt.Printf("  Correction: %s\n", found.Provenance.CorrectionID)
				}
				if found.Provenance.ApprovedBy != "" {
					fmt.Printf("  Approved by: %s\n", found.Provenance.ApprovedBy)
				}
				fmt.Println()

				if len(found.Requires) > 0 {
					fmt.Printf("Requires: %v\n", found.Requires)
				}
				if len(found.Overrides) > 0 {
					fmt.Printf("Overrides: %v\n", found.Overrides)
				}
				if len(found.Conflicts) > 0 {
					fmt.Printf("Conflicts: %v\n", found.Conflicts)
				}
			}

			return nil
		},
	}

	return cmd
}

func newWhyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why [behavior-id]",
		Short: "Explain why a behavior is or isn't active",
		Long: `Show the activation status of a behavior and explain why.

This helps debug when a behavior isn't being applied as expected.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			file, _ := cmd.Flags().GetString("file")
			task, _ := cmd.Flags().GetString("task")
			env, _ := cmd.Flags().GetString("env")
			jsonOut, _ := cmd.Flags().GetBool("json")
			id := args[0]

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

			// Load all behaviors
			behaviors, err := loadBehaviors(floopDir)
			if err != nil {
				return fmt.Errorf("failed to load behaviors: %w", err)
			}

			// Find the behavior
			var found *models.Behavior
			for _, b := range behaviors {
				if b.ID == id || b.Name == id {
					found = &b
					break
				}
			}

			if found == nil {
				if jsonOut {
					json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
						"error": "behavior not found",
						"id":    id,
					})
				} else {
					fmt.Printf("Behavior not found: %s\n", id)
				}
				return nil
			}

			// Build context
			ctxBuilder := activation.NewContextBuilder().
				WithFile(file).
				WithTask(task).
				WithEnvironment(env).
				WithRepoRoot(root)
			ctx := ctxBuilder.Build()

			// Get explanation
			evaluator := activation.NewEvaluator()
			explanation := evaluator.WhyActive(ctx, *found)

			if jsonOut {
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"behavior":    found,
					"context":     ctx,
					"explanation": explanation,
				})
			} else {
				fmt.Printf("Behavior: %s\n", found.Name)
				fmt.Printf("ID: %s\n", found.ID)
				fmt.Println()

				if explanation.IsActive {
					fmt.Println("Status: ACTIVE")
				} else {
					fmt.Println("Status: NOT ACTIVE")
				}
				fmt.Printf("Reason: %s\n", explanation.Reason)
				fmt.Println()

				if len(explanation.Conditions) > 0 {
					fmt.Println("Condition evaluation:")
					for _, c := range explanation.Conditions {
						status := "✓"
						if !c.Matched {
							status = "✗"
						}
						fmt.Printf("  %s %s: required=%v, actual=%v\n",
							status, c.Field, c.Required, c.Actual)
					}
					fmt.Println()
				}

				fmt.Println("Current context:")
				if ctx.FilePath != "" {
					fmt.Printf("  file_path: %s\n", ctx.FilePath)
				}
				if ctx.FileLanguage != "" {
					fmt.Printf("  language: %s\n", ctx.FileLanguage)
				}
				if ctx.Task != "" {
					fmt.Printf("  task: %s\n", ctx.Task)
				}
				if ctx.Branch != "" {
					fmt.Printf("  branch: %s\n", ctx.Branch)
				}
				if ctx.Environment != "" {
					fmt.Printf("  environment: %s\n", ctx.Environment)
				}
			}

			return nil
		},
	}

	cmd.Flags().String("file", "", "Current file path")
	cmd.Flags().String("task", "", "Current task type")
	cmd.Flags().String("env", "", "Environment (dev, staging, prod)")

	return cmd
}
