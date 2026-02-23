package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nvandessel/feedback-loop/internal/config"
	"github.com/nvandessel/feedback-loop/internal/pack"
	"github.com/nvandessel/feedback-loop/internal/store"
	"github.com/spf13/cobra"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Manage skill packs (create, install, list, remove)",
		Long: `Skill packs are portable behavior collections that can be shared and installed.

Examples:
  floop pack create my-pack.fpack --id my-org/my-pack --version 1.0.0
  floop pack install my-pack.fpack
  floop pack list
  floop pack info my-org/my-pack
  floop pack remove my-org/my-pack`,
	}

	cmd.AddCommand(
		newPackCreateCmd(),
		newPackInstallCmd(),
		newPackListCmd(),
		newPackInfoCmd(),
		newPackUpdateCmd(),
		newPackRemoveCmd(),
	)

	return cmd
}

func newPackCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <output-path>",
		Short: "Create a skill pack from current behaviors",
		Long: `Export filtered behaviors into a portable .fpack file.

Examples:
  floop pack create my-pack.fpack --id my-org/my-pack --version 1.0.0
  floop pack create my-pack.fpack --id my-org/my-pack --version 1.0.0 --filter-tags go,testing
  floop pack create my-pack.fpack --id my-org/my-pack --version 1.0.0 --filter-scope global`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")
			id, _ := cmd.Flags().GetString("id")
			ver, _ := cmd.Flags().GetString("version")
			desc, _ := cmd.Flags().GetString("description")
			author, _ := cmd.Flags().GetString("author")
			tags, _ := cmd.Flags().GetString("tags")
			source, _ := cmd.Flags().GetString("source")
			filterTags, _ := cmd.Flags().GetString("filter-tags")
			filterScope, _ := cmd.Flags().GetString("filter-scope")
			filterKinds, _ := cmd.Flags().GetString("filter-kinds")

			manifest := pack.PackManifest{
				ID:          pack.PackID(id),
				Version:     ver,
				Description: desc,
				Author:      author,
				Source:      source,
			}
			if tags != "" {
				manifest.Tags = strings.Split(tags, ",")
			}

			filter := pack.CreateFilter{
				Scope: filterScope,
			}
			if filterTags != "" {
				filter.Tags = strings.Split(filterTags, ",")
			}
			if filterKinds != "" {
				filter.Kinds = strings.Split(filterKinds, ",")
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := pack.Create(ctx, graphStore, filter, manifest, outputPath, pack.CreateOptions{
				FloopVersion: version,
			})
			if err != nil {
				return fmt.Errorf("pack create failed: %w", err)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"path":           result.Path,
					"behavior_count": result.BehaviorCount,
					"edge_count":     result.EdgeCount,
					"pack_id":        id,
					"version":        ver,
					"message":        fmt.Sprintf("Pack created: %d behaviors, %d edges", result.BehaviorCount, result.EdgeCount),
				})
			}

			fmt.Printf("Pack created: %d behaviors, %d edges\n", result.BehaviorCount, result.EdgeCount)
			fmt.Printf("  ID: %s\n", id)
			fmt.Printf("  Version: %s\n", ver)
			fmt.Printf("  Path: %s\n", result.Path)
			return nil
		},
	}

	cmd.Flags().String("id", "", "Pack ID in namespace/name format (required)")
	cmd.Flags().String("version", "", "Pack version (required)")
	cmd.Flags().String("description", "", "Pack description")
	cmd.Flags().String("author", "", "Pack author")
	cmd.Flags().String("tags", "", "Comma-separated pack tags")
	cmd.Flags().String("source", "", "Pack source URL")
	cmd.Flags().String("filter-tags", "", "Filter: only include behaviors with these tags (comma-separated)")
	cmd.Flags().String("filter-scope", "", "Filter: only include behaviors from this scope (global/local)")
	cmd.Flags().String("filter-kinds", "", "Filter: only include behaviors of these kinds (comma-separated)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("version")

	return cmd
}

func newPackInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <file-path>",
		Short: "Install a skill pack from a .fpack file",
		Long: `Install behaviors from a skill pack file into the store.

Follows the seeder pattern: forgotten behaviors are not re-added,
existing behaviors are version-gated for updates, and provenance
is stamped on each installed behavior.

Examples:
  floop pack install my-pack.fpack
  floop pack install ~/.floop/packs/go-best-practices.fpack`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := pack.Install(ctx, graphStore, filePath, cfg, pack.InstallOptions{})
			if err != nil {
				return fmt.Errorf("pack install failed: %w", err)
			}

			// Save config with updated pack list
			if saveErr := cfg.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save config: %v\n", saveErr)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"pack_id":       result.PackID,
					"version":       result.Version,
					"added":         result.Added,
					"updated":       result.Updated,
					"skipped":       result.Skipped,
					"edges_added":   result.EdgesAdded,
					"edges_skipped": result.EdgesSkipped,
					"message":       fmt.Sprintf("Installed %s v%s: %d added, %d updated, %d skipped", result.PackID, result.Version, len(result.Added), len(result.Updated), len(result.Skipped)),
				})
			}

			fmt.Printf("Installed %s v%s\n", result.PackID, result.Version)
			fmt.Printf("  Added: %d behaviors\n", len(result.Added))
			fmt.Printf("  Updated: %d behaviors\n", len(result.Updated))
			fmt.Printf("  Skipped: %d behaviors\n", len(result.Skipped))
			fmt.Printf("  Edges: %d added, %d skipped\n", result.EdgesAdded, result.EdgesSkipped)
			return nil
		},
	}

	return cmd
}

func newPackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skill packs",
		Long: `Show all currently installed skill packs from config.

Examples:
  floop pack list
  floop pack list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			installed := pack.ListInstalled(cfg)

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"installed": installed,
					"count":     len(installed),
				})
			}

			if len(installed) == 0 {
				fmt.Println("No skill packs installed.")
				return nil
			}

			fmt.Printf("Installed packs (%d):\n", len(installed))
			for _, p := range installed {
				fmt.Printf("  %s v%s (%d behaviors, %d edges)\n", p.ID, p.Version, p.BehaviorCount, p.EdgeCount)
				if !p.InstalledAt.IsZero() {
					fmt.Printf("    Installed: %s\n", p.InstalledAt.Format("2006-01-02 15:04:05"))
				}
			}
			return nil
		},
	}

	return cmd
}

func newPackInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <pack-id>",
		Short: "Show details of an installed skill pack",
		Long: `Display pack details and behavior count from the store.

Examples:
  floop pack info my-org/my-pack
  floop pack info my-org/my-pack --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packID := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			// Find pack in config
			var installed *config.InstalledPack
			for _, p := range cfg.Packs.Installed {
				if p.ID == packID {
					installed = &p
					break
				}
			}

			// Find behaviors in store
			behaviors, err := pack.FindByPack(ctx, graphStore, packID)
			if err != nil {
				return fmt.Errorf("querying pack behaviors: %w", err)
			}

			if jsonOut {
				out := map[string]interface{}{
					"pack_id":        packID,
					"behavior_count": len(behaviors),
				}
				if installed != nil {
					out["version"] = installed.Version
					out["installed_at"] = installed.InstalledAt
					out["edge_count"] = installed.EdgeCount
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			if installed == nil && len(behaviors) == 0 {
				fmt.Printf("Pack %q not found.\n", packID)
				return nil
			}

			fmt.Printf("Pack: %s\n", packID)
			if installed != nil {
				fmt.Printf("  Version: %s\n", installed.Version)
				if !installed.InstalledAt.IsZero() {
					fmt.Printf("  Installed: %s\n", installed.InstalledAt.Format("2006-01-02 15:04:05"))
				}
				fmt.Printf("  Config edges: %d\n", installed.EdgeCount)
			}
			fmt.Printf("  Behaviors in store: %d\n", len(behaviors))
			for _, b := range behaviors {
				name := ""
				if content, ok := b.Content["name"].(string); ok {
					name = content
				}
				fmt.Printf("    - %s (%s)\n", b.ID, name)
			}
			return nil
		},
	}

	return cmd
}

func newPackUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <file-path>",
		Short: "Update an installed pack from a newer .fpack file",
		Long: `Reinstall a pack with a newer version. This is equivalent to running install
with a newer pack file -- existing behaviors are version-gated for updates.

Examples:
  floop pack update my-pack-v2.fpack`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Update is the same as install -- the version-gating handles the upgrade
			filePath := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := pack.Install(ctx, graphStore, filePath, cfg, pack.InstallOptions{})
			if err != nil {
				return fmt.Errorf("pack update failed: %w", err)
			}

			if saveErr := cfg.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save config: %v\n", saveErr)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"pack_id":       result.PackID,
					"version":       result.Version,
					"added":         result.Added,
					"updated":       result.Updated,
					"skipped":       result.Skipped,
					"edges_added":   result.EdgesAdded,
					"edges_skipped": result.EdgesSkipped,
					"message":       fmt.Sprintf("Updated %s to v%s: %d added, %d updated, %d skipped", result.PackID, result.Version, len(result.Added), len(result.Updated), len(result.Skipped)),
				})
			}

			fmt.Printf("Updated %s to v%s\n", result.PackID, result.Version)
			fmt.Printf("  Added: %d behaviors\n", len(result.Added))
			fmt.Printf("  Updated: %d behaviors\n", len(result.Updated))
			fmt.Printf("  Skipped: %d behaviors\n", len(result.Skipped))
			fmt.Printf("  Edges: %d added, %d skipped\n", result.EdgesAdded, result.EdgesSkipped)
			return nil
		},
	}

	return cmd
}

func newPackRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <pack-id>",
		Short: "Remove an installed skill pack",
		Long: `Remove a pack by marking its behaviors as forgotten and removing
the pack from the installed packs list.

Examples:
  floop pack remove my-org/my-pack
  floop pack remove my-org/my-pack --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packID := args[0]
			root, _ := cmd.Flags().GetString("root")
			jsonOut, _ := cmd.Flags().GetBool("json")

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			ctx := context.Background()
			graphStore, err := store.NewMultiGraphStore(root)
			if err != nil {
				return fmt.Errorf("failed to open store: %w", err)
			}
			defer graphStore.Close()

			result, err := pack.Remove(ctx, graphStore, packID, cfg)
			if err != nil {
				return fmt.Errorf("pack remove failed: %w", err)
			}

			if saveErr := cfg.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save config: %v\n", saveErr)
			}

			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"pack_id":            result.PackID,
					"behaviors_removed":  result.BehaviorsRemoved,
					"behaviors_notfound": result.BehaviorsNotFound,
					"message":            fmt.Sprintf("Removed %s: %d behaviors marked as forgotten", result.PackID, result.BehaviorsRemoved),
				})
			}

			fmt.Printf("Removed %s\n", result.PackID)
			fmt.Printf("  Behaviors marked as forgotten: %d\n", result.BehaviorsRemoved)
			return nil
		},
	}

	return cmd
}
