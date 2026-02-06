package assembly

import (
	"strings"
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func makeInjectedBehavior(id string, kind models.BehaviorKind, tags []string, score float64, content string) models.InjectedBehavior {
	return makeInjectedBehaviorNamed(id, id, kind, tags, score, content)
}

func makeInjectedBehaviorNamed(id, name string, kind models.BehaviorKind, tags []string, score float64, content string) models.InjectedBehavior {
	return models.InjectedBehavior{
		Behavior: &models.Behavior{
			ID:   id,
			Name: name,
			Kind: kind,
			Content: models.BehaviorContent{
				Canonical: content,
				Tags:      tags,
			},
		},
		Tier:    models.TierFull,
		Content: content,
		Score:   score,
	}
}

func TestCoalescer_NoClusters(t *testing.T) {
	c := NewCoalescer(DefaultCoalesceConfig())

	// 2 behaviors with different tags. MinClusterSize=3, so no cluster possible.
	behaviors := []models.InjectedBehavior{
		makeInjectedBehavior("b1", models.BehaviorKindDirective, []string{"go", "testing"}, 0.8, "Use table-driven tests"),
		makeInjectedBehavior("b2", models.BehaviorKindDirective, []string{"python", "linting"}, 0.6, "Use pylint"),
	}

	individuals, clusters := c.Coalesce(behaviors)

	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
	if len(individuals) != 2 {
		t.Errorf("expected 2 individuals, got %d", len(individuals))
	}
}

func TestCoalescer_TagClustering(t *testing.T) {
	c := NewCoalescer(DefaultCoalesceConfig())

	// 4 behaviors: 3 share tags ["python", "filesystem"], 1 has different tags.
	behaviors := []models.InjectedBehavior{
		makeInjectedBehavior("b1", models.BehaviorKindDirective, []string{"python", "filesystem"}, 0.9, "Use pathlib.Path"),
		makeInjectedBehavior("b2", models.BehaviorKindDirective, []string{"python", "filesystem", "io"}, 0.7, "Use context managers for files"),
		makeInjectedBehavior("b3", models.BehaviorKindDirective, []string{"python", "filesystem"}, 0.5, "Avoid os.walk"),
		makeInjectedBehavior("b4", models.BehaviorKindDirective, []string{"go", "concurrency"}, 0.8, "Use channels"),
	}

	individuals, clusters := c.Coalesce(behaviors)

	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if len(individuals) != 1 {
		t.Errorf("expected 1 individual, got %d", len(individuals))
	}

	// The individual should be the Go behavior.
	if len(individuals) > 0 && individuals[0].Behavior.ID != "b4" {
		t.Errorf("expected individual to be b4, got %s", individuals[0].Behavior.ID)
	}

	// The cluster should have 3 members total (1 representative + 2 members).
	cluster := clusters[0]
	totalInCluster := 1 + len(cluster.Members)
	if totalInCluster != 3 {
		t.Errorf("expected 3 behaviors in cluster, got %d", totalInCluster)
	}
}

func TestCoalescer_RepresentativeSelection(t *testing.T) {
	c := NewCoalescer(DefaultCoalesceConfig())

	// 3 behaviors with same tags, different activation scores.
	behaviors := []models.InjectedBehavior{
		makeInjectedBehavior("low", models.BehaviorKindDirective, []string{"go", "testing"}, 0.3, "Low activation"),
		makeInjectedBehavior("high", models.BehaviorKindDirective, []string{"go", "testing"}, 0.9, "High activation"),
		makeInjectedBehavior("mid", models.BehaviorKindDirective, []string{"go", "testing"}, 0.6, "Mid activation"),
	}

	_, clusters := c.Coalesce(behaviors)

	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}

	// Highest activation should be the representative.
	if clusters[0].Representative.Behavior.ID != "high" {
		t.Errorf("expected representative to be 'high', got %q", clusters[0].Representative.Behavior.ID)
	}

	// The other two should be members.
	if len(clusters[0].Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(clusters[0].Members))
	}
}

func TestCoalescer_KindGrouping(t *testing.T) {
	c := NewCoalescer(DefaultCoalesceConfig())

	// 3 directives and 3 constraints, all with the same tags.
	// Should create 2 separate clusters, not mix kinds.
	behaviors := []models.InjectedBehavior{
		makeInjectedBehavior("d1", models.BehaviorKindDirective, []string{"go", "testing"}, 0.9, "Directive 1"),
		makeInjectedBehavior("d2", models.BehaviorKindDirective, []string{"go", "testing"}, 0.7, "Directive 2"),
		makeInjectedBehavior("d3", models.BehaviorKindDirective, []string{"go", "testing"}, 0.5, "Directive 3"),
		makeInjectedBehavior("c1", models.BehaviorKindConstraint, []string{"go", "testing"}, 0.8, "Constraint 1"),
		makeInjectedBehavior("c2", models.BehaviorKindConstraint, []string{"go", "testing"}, 0.6, "Constraint 2"),
		makeInjectedBehavior("c3", models.BehaviorKindConstraint, []string{"go", "testing"}, 0.4, "Constraint 3"),
	}

	_, clusters := c.Coalesce(behaviors)

	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters (one per kind), got %d", len(clusters))
	}

	// Verify each cluster has behaviors of only one kind.
	for _, cluster := range clusters {
		repKind := cluster.Representative.Behavior.Kind
		for _, m := range cluster.Members {
			if m.Behavior.Kind != repKind {
				t.Errorf("cluster has mixed kinds: representative=%s, member=%s", repKind, m.Behavior.Kind)
			}
		}
	}
}

func TestCoalescer_ClusterLabel(t *testing.T) {
	c := NewCoalescer(DefaultCoalesceConfig())

	behaviors := []models.InjectedBehavior{
		makeInjectedBehavior("b1", models.BehaviorKindDirective, []string{"go", "testing"}, 0.9, "B1"),
		makeInjectedBehavior("b2", models.BehaviorKindDirective, []string{"go", "testing"}, 0.7, "B2"),
		makeInjectedBehavior("b3", models.BehaviorKindDirective, []string{"go", "testing"}, 0.5, "B3"),
	}

	_, clusters := c.Coalesce(behaviors)

	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}

	label := clusters[0].ClusterLabel
	if !strings.Contains(label, "Go") || !strings.Contains(label, "Testing") {
		t.Errorf("expected label to contain 'Go' and 'Testing', got %q", label)
	}

	// Verify shared tags.
	if len(clusters[0].SharedTags) != 2 {
		t.Errorf("expected 2 shared tags, got %d: %v", len(clusters[0].SharedTags), clusters[0].SharedTags)
	}
}

func TestCompileCoalesced_Output(t *testing.T) {
	compiler := NewCompiler()

	individuals := []models.InjectedBehavior{
		makeInjectedBehavior("ind1", models.BehaviorKindDirective, []string{"go"}, 0.8, "Use Go modules"),
	}

	clusters := []BehaviorCluster{
		{
			Representative: makeInjectedBehavior("rep1", models.BehaviorKindDirective, []string{"python", "filesystem"}, 0.9, "Use pathlib.Path instead of os.path for all file operations"),
			Members: []models.InjectedBehavior{
				makeInjectedBehaviorNamed("m1", "prefer-context-managers", models.BehaviorKindDirective, []string{"python", "filesystem"}, 0.5, "Use context managers for file handles"),
				makeInjectedBehaviorNamed("m2", "avoid-os-walk", models.BehaviorKindDirective, []string{"python", "filesystem"}, 0.3, "Avoid os.walk in favor of pathlib.iterdir"),
			},
			ClusterLabel: "Python Filesystem",
			SharedTags:   []string{"python", "filesystem"},
		},
	}

	output := compiler.CompileCoalesced(individuals, clusters)

	// Should contain the individual behavior.
	if !strings.Contains(output, "Use Go modules") {
		t.Error("expected output to contain individual behavior content")
	}

	// Should contain the cluster heading with member count.
	if !strings.Contains(output, "Python Filesystem") {
		t.Error("expected output to contain cluster label")
	}
	if !strings.Contains(output, "3 behaviors") {
		t.Error("expected output to contain behavior count")
	}

	// Should contain the representative's full content.
	if !strings.Contains(output, "Use pathlib.Path") {
		t.Error("expected output to contain representative's full content")
	}

	// Should contain member names in italic/summary form.
	if !strings.Contains(output, "prefer-context-managers") {
		t.Error("expected output to contain member name")
	}
	if !strings.Contains(output, "avoid-os-walk") {
		t.Error("expected output to contain member name")
	}

	// Should contain the floop show hint.
	if !strings.Contains(output, "floop show") {
		t.Error("expected output to contain floop show hint")
	}
}
