package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nvandessel/feedback-loop/internal/backup"
	"github.com/nvandessel/feedback-loop/internal/pathutil"
	"github.com/nvandessel/feedback-loop/internal/store"
	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Export full graph state to a backup file",
		Long: `Backup the complete behavior graph (nodes + edges) to a JSON file.

Default location: ~/.floop/backups/floop-backup-YYYYMMDD-HHMMSS.json
Keeps last 10 backups with automatic rotation.

Examples:
  floop backup                           # Backup to default location
  floop backup --output my-backup.json   # Backup to specific file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")
			outputPath, _ := cmd.Flags().GetString("output")

			if outputPath == "" {
				dir, err := backup.DefaultBackupDir()
				if err != nil {
					return fmt.Errorf("failed to get backup directory: %w", err)
				}
				outputPath = backup.GenerateBackupPath(dir)
			} else {
				// Validate user-specified path
				allowedDirs, err := pathutil.DefaultAllowedBackupDirsWithProjectRoot(root)
				if err != nil {
					return fmt.Errorf("failed to determine allowed backup dirs: %w", err)
				}
				if err := pathutil.ValidatePath(outputPath, allowedDirs); err != nil {
					return fmt.Errorf("backup path rejected: %w", err)
				}
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root, store.ScopeBoth)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := backup.Backup(ctx, graphStore, outputPath)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			// Rotate
			dir := outputPath[:len(outputPath)-len("/"+result.CreatedAt.Format("20060102-150405")+".json")]
			if err := backup.RotateBackups(dir, 10); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to rotate backups: %v\n", err)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"path":       outputPath,
					"node_count": len(result.Nodes),
					"edge_count": len(result.Edges),
					"message":    fmt.Sprintf("Backup created: %d nodes, %d edges", len(result.Nodes), len(result.Edges)),
				})
			}

			fmt.Printf("✓ Backup created: %d nodes, %d edges\n", len(result.Nodes), len(result.Edges))
			fmt.Printf("  Path: %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().String("output", "", "Output file path (default: auto-generated in ~/.floop/backups/)")

	return cmd
}

func newRestoreFromBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore-backup <file>",
		Short: "Restore graph state from a backup file",
		Long: `Restore behavior graph from a backup JSON file.

Modes:
  merge   - Skip existing nodes/edges (default)
  replace - Clear store first, then restore

Examples:
  floop restore-backup ~/.floop/backups/floop-backup-20260206-120000.json
  floop restore-backup backup.json --mode replace`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")
			mode, _ := cmd.Flags().GetString("mode")

			// Validate input path
			allowedDirs, err := pathutil.DefaultAllowedBackupDirsWithProjectRoot(root)
			if err != nil {
				return fmt.Errorf("failed to determine allowed backup dirs: %w", err)
			}
			if err := pathutil.ValidatePath(inputPath, allowedDirs); err != nil {
				return fmt.Errorf("restore path rejected: %w", err)
			}

			restoreMode := backup.RestoreMerge
			if mode == "replace" {
				restoreMode = backup.RestoreReplace
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root, store.ScopeBoth)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := backup.Restore(ctx, graphStore, inputPath, restoreMode)
			if err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"nodes_restored": result.NodesRestored,
					"nodes_skipped":  result.NodesSkipped,
					"edges_restored": result.EdgesRestored,
					"edges_skipped":  result.EdgesSkipped,
					"message":        fmt.Sprintf("Restore complete: %d nodes, %d edges", result.NodesRestored, result.EdgesRestored),
				})
			}

			fmt.Printf("✓ Restore complete (mode: %s)\n", mode)
			fmt.Printf("  Nodes: %d restored, %d skipped\n", result.NodesRestored, result.NodesSkipped)
			fmt.Printf("  Edges: %d restored, %d skipped\n", result.EdgesRestored, result.EdgesSkipped)
			return nil
		},
	}

	cmd.Flags().String("mode", "merge", "Restore mode: merge or replace")

	return cmd
}
