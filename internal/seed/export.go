package seed

import (
	"context"
	"fmt"

	"github.com/nvandessel/floop/internal/pack"
	"github.com/nvandessel/floop/internal/store"
)

// ExportCorePack generates a .fpack file from the core seed behaviors.
// It creates an in-memory store, seeds it, and exports via pack.Create,
// dogfooding the full pack pipeline.
func ExportCorePack(outputPath string) error {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	seeder := NewSeeder(s)
	if _, err := seeder.SeedGlobalStore(ctx); err != nil {
		return fmt.Errorf("seeding in-memory store: %w", err)
	}

	manifest := pack.PackManifest{
		ID:          "floop/core",
		Version:     SeedVersion,
		Description: "Core behaviors that teach agents how to use floop",
		Author:      "floop",
		Source:      "https://github.com/nvandessel/feedback-loop",
	}

	_, err := pack.Create(ctx, s, pack.CreateFilter{}, manifest, outputPath, pack.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating core pack: %w", err)
	}

	return nil
}
