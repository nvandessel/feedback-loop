# LanceDB Hardening Follow-ups

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all deferred review feedback from the LanceDB migration (PR #208) — performance, test coverage, incremental sync, and CI hardening.

**Architecture:** Five independent improvements to the LanceDB vector index. Tasks 1-3 are code changes in `internal/vectorindex/` and `internal/mcp/`. Tasks 4-5 are CI-only. Two additional items (tombstone compaction, non-atomic upsert) are documented as known limitations blocked by upstream API gaps. Each task is independently mergeable.

**Tech Stack:** Go 1.24+, LanceDB Go SDK v0.1.2, Apache Arrow Go v17, GitHub Actions

---

## Critical Files

| File | Role |
|------|------|
| `internal/vectorindex/lancedb.go` | LanceDB VectorIndex implementation (CGO) |
| `internal/vectorindex/lancedb_test.go` | LanceDB unit tests (12 existing) |
| `internal/mcp/server.go:242-296` | `initVectorIndex` — startup sync logic |
| `.github/workflows/ci.yml:95-118` | CGO CI job |

---

## Chunk 1: Code Quality

### Task 1: Cache Arrow schema in LanceDBIndex struct

**Why:** `buildRecord` allocates a new `arrow.Schema` and `arrow.FixedSizeListType` on every `Add` call. These are immutable once the index is created. Caching reduces GC pressure under write-heavy workloads.

**Files:**
- Modify: `internal/vectorindex/lancedb.go:26-31` (struct fields)
- Modify: `internal/vectorindex/lancedb.go:121` (constructor — cache after creation)
- Modify: `internal/vectorindex/lancedb.go:124-153` (buildRecord — use cached fields)

- [ ] **Step 1: Add cached fields to LanceDBIndex struct**

```go
type LanceDBIndex struct {
	mu    sync.RWMutex
	db    contracts.IConnection
	table contracts.ITable
	dims  int

	// Cached Arrow schema and vector type — immutable after construction.
	arrowSchema *arrow.Schema
	vectorType  *arrow.FixedSizeListType
}
```

- [ ] **Step 2: Initialize cached fields in NewLanceDBIndex**

After `return &LanceDBIndex{...}` on line 121, add the cached fields:

```go
vectorType := arrow.FixedSizeListOf(int32(cfg.Dims), arrow.PrimitiveTypes.Float32)
arrowSchema := arrow.NewSchema([]arrow.Field{
	{Name: "id", Type: arrow.BinaryTypes.String},
	{Name: "vector", Type: vectorType},
}, nil)

return &LanceDBIndex{
	db:          db,
	table:       table,
	dims:        cfg.Dims,
	arrowSchema: arrowSchema,
	vectorType:  vectorType,
}, nil
```

- [ ] **Step 3: Update buildRecord to use cached fields**

Replace the schema/type construction in `buildRecord` with `l.arrowSchema` and `l.vectorType`:

```go
func (l *LanceDBIndex) buildRecord(behaviorID string, vector []float32) (arrow.Record, error) {
	pool := memory.NewGoAllocator()

	idBuilder := array.NewStringBuilder(pool)
	defer idBuilder.Release()
	idBuilder.Append(behaviorID)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	floatBuilder := array.NewFloat32Builder(pool)
	defer floatBuilder.Release()
	floatBuilder.AppendValues(vector, nil)
	floatArray := floatBuilder.NewArray()
	defer floatArray.Release()

	vectorData := array.NewData(l.vectorType, 1, []*memory.Buffer{nil}, []arrow.ArrayData{floatArray.Data()}, 0, 0)
	defer vectorData.Release()
	vectorArray := array.NewFixedSizeListData(vectorData)
	defer vectorArray.Release()

	rec := array.NewRecord(l.arrowSchema, []arrow.Array{idArray, vectorArray}, 1)
	return rec, nil
}
```

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=0 go build ./...`
Expected: compiles cleanly (verifies no-CGO stub still builds)

Note: The CGO-enabled tests (`CGO_ENABLED=1 go test ./internal/vectorindex/...`) exercise the changed code but require native LanceDB libraries. If CGO is available locally, run them. Otherwise, the existing 12 LanceDB tests in CI will validate this change.

- [ ] **Step 5: Commit**

```bash
git add internal/vectorindex/lancedb.go
git commit -m "perf(vectorindex): cache Arrow schema in LanceDBIndex struct"
```

---

### Task 2: Add dimension-mismatch test

**Why:** The schema validation on table reopen (checking that on-disk dims match configured dims) has no unit test. This is the only major code path without test coverage.

**Files:**
- Modify: `internal/vectorindex/lancedb_test.go`

- [ ] **Step 1: Write the test**

Add after `TestLanceDBIndex_Persistence`:

```go
func TestLanceDBIndex_DimensionMismatchOnReopen(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create index with 8 dimensions and add a vector.
	idx, err := NewLanceDBIndex(LanceDBConfig{Dir: dir, Dims: 8})
	if err != nil {
		t.Fatalf("NewLanceDBIndex: %v", err)
	}
	mustAdd(t, idx, ctx, "b1", []float32{1, 0, 0, 0, 0, 0, 0, 0})
	if err := idx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen with different dimensions — should fail.
	_, err = NewLanceDBIndex(LanceDBConfig{Dir: dir, Dims: 4})
	if err == nil {
		t.Fatal("expected error for dimension mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "vector dimension mismatch") {
		t.Errorf("expected dimension mismatch error, got: %v", err)
	}
}
```

- [ ] **Step 2: Add `"strings"` to imports if not already present**

Check the import block in `lancedb_test.go`. Add `"strings"` if missing.

- [ ] **Step 3: Run test to verify it passes**

Run: `CGO_ENABLED=1 go test -v -run TestLanceDBIndex_DimensionMismatchOnReopen ./internal/vectorindex/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/vectorindex/lancedb_test.go
git commit -m "test(vectorindex): add dimension-mismatch-on-reopen test"
```

---

## Chunk 2: Incremental Sync

### Task 3: Incremental sync on startup (replace Len()==0 guard)

**Why:** The current `idx.Len() == 0` guard prevents tombstone churn on restart, but also prevents recovery when vectors are missing from LanceDB (e.g., crash after SQLite write but before LanceDB write, or after a non-atomic upsert failure).

**Approach:** Use a count-based heuristic. If LanceDB count matches SQLite count, skip (no churn). If LanceDB has fewer, re-add all from SQLite. The upsert (delete+add) creates tombstones for existing entries, but this only triggers on recovery — not on every restart.

**Trade-off:** On recovery, existing entries get tombstones from the re-upsert. This is acceptable because (a) recovery is rare, (b) the alternative (per-ID existence checks) is not supported by LanceDB's Go SDK, and (c) the tombstone cost is bounded by the total embedding count.

**Note:** If LanceDB has *more* entries than SQLite (e.g., SQLite entries were deleted outside of floop), no cleanup happens. This is intentional — LanceDB is the index, not the source of truth. Stale entries degrade search quality slightly but don't cause errors. A manual `.floop/vectors/` delete resolves it.

**Files:**
- Modify: `internal/mcp/server.go:284-296` (replace sync logic)

- [ ] **Step 1: Replace the sync logic in initVectorIndex**

Replace lines 284-296 in `server.go`:

```go
// Sync SQLite embeddings to LanceDB.
// - Empty table (first run or after wipe): bulk add all embeddings.
// - Count mismatch: some vectors are missing — re-add all. The upsert
//   (delete+add) creates tombstones for existing entries, but this only
//   happens on recovery, not on every restart.
// - Counts match: skip sync entirely (no tombstone churn).
if loadErr == nil {
	lanceCount := idx.Len()
	sqliteCount := len(allEmb)
	if lanceCount < sqliteCount {
		var addErrs int
		for _, emb := range allEmb {
			if err := idx.Add(context.Background(), emb.BehaviorID, emb.Embedding); err != nil {
				addErrs++
			}
		}
		if addErrs > 0 {
			s.logger.Warn("some embeddings failed to load into vector index",
				"errors", addErrs, "total", sqliteCount)
		}
		if lanceCount > 0 {
			s.logger.Info("recovered missing vectors from SQLite",
				"before", lanceCount, "after", idx.Len(), "sqlite_total", sqliteCount)
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `CGO_ENABLED=0 go test ./internal/mcp/...`
Expected: PASS

Run: `CGO_ENABLED=0 go build ./...`
Expected: compiles cleanly

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/server.go
git commit -m "fix(mcp): incremental vector sync recovers missing embeddings on restart"
```

---

## Chunk 3: CI Hardening

### Task 4: Add macOS CGO CI job

**Why:** LanceDB ships platform-specific native libraries. The current `test-cgo` job only runs on Linux. Platform-specific issues with download scripts, CGO flags, or linker paths would go undetected.

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add macOS job**

Add after the existing `test-cgo` job:

```yaml
  test-cgo-macos:
    name: Test (CGO + LanceDB, macOS)
    needs: changes
    if: needs.changes.outputs.code == 'true'
    runs-on: macos-latest
    env:
      CGO_ENABLED: "1"
      CGO_CFLAGS: "-I${{ github.workspace }}/include"
      CGO_LDFLAGS: "-L${{ github.workspace }}/lib/darwin_arm64 -llancedb_go"
      DYLD_LIBRARY_PATH: "${{ github.workspace }}/lib/darwin_arm64"
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version-file: "go.mod"
      - name: Download Go modules
        run: go mod download
      - name: Download LanceDB native libraries
        run: |
          LANCE_VERSION=$(go list -m -f '{{.Version}}' github.com/lancedb/lancedb-go)
          bash "$(go env GOMODCACHE)/github.com/lancedb/lancedb-go@${LANCE_VERSION}/scripts/download-artifacts.sh" "${LANCE_VERSION}"
      - name: Run CGO tests
        run: go test -race -count=1 ./...
```

**Note:** The `lib/darwin_arm64` path and `DYLD_LIBRARY_PATH` env var are macOS-specific. Verify the actual directory name output by `download-artifacts.sh` — it may be `darwin_amd64` on Intel runners. If `macos-latest` is ARM64 (M-series), use `darwin_arm64`. Check `download-artifacts.sh` for the exact output directory structure.

- [ ] **Step 2: Verify CI YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
Expected: no error

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add macOS CGO test job for LanceDB cross-platform validation"
```

**Note:** This job may fail on the first run if the download script or library paths differ on macOS. Monitor the first CI run and fix paths as needed. Consider `continue-on-error: true` initially if the macOS library paths need investigation.

---

### Task 5: Cache LanceDB native library artifacts in CI

**Why:** `download-artifacts.sh` fetches pre-built Rust binaries from an external server on every CI run. Caching reduces CI time and adds a degree of reproducibility (same artifact reused across runs with the same `go.sum`).

**Files:**
- Modify: `.github/workflows/ci.yml` (both `test-cgo` and `test-cgo-macos` jobs)

- [ ] **Step 1: Add cache step before download**

Add before the "Download LanceDB native libraries" step in each CGO job:

```yaml
      - name: Cache LanceDB native libraries
        uses: actions/cache@v4
        with:
          path: |
            lib/
            include/
          key: lancedb-native-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('go.sum') }}
```

- [ ] **Step 2: Make download step conditional**

Wrap the download in a check so it only runs on cache miss:

```yaml
      - name: Download LanceDB native libraries
        run: |
          if [ ! -d "lib/" ]; then
            LANCE_VERSION=$(go list -m -f '{{.Version}}' github.com/lancedb/lancedb-go)
            bash "$(go env GOMODCACHE)/github.com/lancedb/lancedb-go@${LANCE_VERSION}/scripts/download-artifacts.sh" "${LANCE_VERSION}"
          fi
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: cache LanceDB native library artifacts across CI runs"
```

---

## Not Actionable (Documented)

### Tombstone accumulation without compaction

**Status:** Known limitation. LanceDB v0.1.2 does not expose `Optimize()` or `Compact()` APIs. Documented in `lancedb.go` Add method comment.

**Recovery:** Delete `.floop/vectors/` to force a full rebuild from SQLite.

**Follow-up:** Monitor `lancedb-go` releases for compaction API. When available, add a post-startup or periodic `Optimize()` call.

### Non-atomic delete+add upsert

**Status:** Known limitation. LanceDB v0.1.2 has no transaction support. Documented in `lancedb.go` Add method comment.

**Recovery:** Delete `.floop/vectors/` to force a full rebuild from SQLite.

**Impact:** Extremely narrow failure window (two sequential calls within the same mutex hold). Only triggers on context cancellation or I/O error between delete and add.

---

## Execution Strategy

Tasks 1-2 are independent and can run in parallel (both touch `internal/vectorindex/`, but different files).

Task 3 depends on nothing but should be reviewed carefully (changes startup behavior).

Tasks 4-5 are CI-only and can run together.

Recommended branch strategy:
```
main
 └── fix/lancedb-hardening  ← Tasks 1-5 as individual commits
```

Single branch, one PR. Tasks are small enough to not need stacking.
