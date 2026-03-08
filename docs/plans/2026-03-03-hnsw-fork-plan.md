# HNSW Fork Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `google/renameio` with `natefinch/atomic` in a fork of `coder/hnsw`, enabling native Windows HNSW support and removing the brute-force fallback.

**Architecture:** Fork `coder/hnsw` to `nvandessel/hnsw`. The only code change in the fork is the `Save()` method in `encode.go` — replacing `renameio` with `natefinch/atomic` + `io.Pipe` + `bufio` for memory-efficient atomic writes. In floop, wire in via `go.mod` replace directive and delete the Windows build-tag workaround.

**Tech Stack:** Go, `natefinch/atomic`, `io.Pipe`, `bufio`

**Design doc:** `docs/plans/2026-03-03-hnsw-fork-design.md`

---

### Task 1: Create the fork on GitHub

**Step 1: Fork coder/hnsw**

Go to https://github.com/coder/hnsw and fork it to `nvandessel/hnsw` via the GitHub UI or CLI:

```bash
gh repo fork coder/hnsw --clone=false --org="" --fork-name=hnsw
```

**Step 2: Clone the fork locally**

```bash
cd /tmp
git clone git@github.com:nvandessel/hnsw.git
cd hnsw
```

**Step 3: Create a feature branch**

```bash
git checkout -b fix/windows-atomic-save
```

**Step 4: Commit (nothing yet, just verify)**

```bash
git status
```

Expected: clean working tree on `fix/windows-atomic-save` branch.

---

### Task 2: Replace renameio with natefinch/atomic in the fork

**Files:**
- Modify: `encode.go` (lines 2-12 imports, lines 302-327 Save method)
- Modify: `go.mod`

**Step 1: Add natefinch/atomic dependency**

```bash
go get github.com/natefinch/atomic
```

**Step 2: Remove renameio dependency**

```bash
go get -d github.com/google/renameio@none || true
```

**Step 3: Replace the Save() method in encode.go**

Replace the import block — change:
```go
import (
	"bufio"
	"cmp"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/google/renameio"
)
```

To:
```go
import (
	"bufio"
	"cmp"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/natefinch/atomic"
)
```

Replace the `Save()` method (lines 302-327) — change:
```go
// Save writes the graph to the file.
func (g *SavedGraph[K]) Save() error {
	tmp, err := renameio.TempFile("", g.Path)
	if err != nil {
		return err
	}
	defer tmp.Cleanup()

	wr := bufio.NewWriter(tmp)
	err = g.Export(wr)
	if err != nil {
		return fmt.Errorf("exporting: %w", err)
	}

	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("flushing: %w", err)
	}

	err = tmp.CloseAtomicallyReplace()
	if err != nil {
		return fmt.Errorf("closing atomically: %w", err)
	}

	return nil
}
```

To:
```go
// Save writes the graph to the file.
func (g *SavedGraph[K]) Save() error {
	pr, pw := io.Pipe()

	// Export into the pipe writer in a goroutine so that atomic.WriteFile
	// can consume from the pipe reader concurrently.
	go func() {
		wr := bufio.NewWriter(pw)
		err := g.Export(wr)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("exporting: %w", err))
			return
		}
		if err = wr.Flush(); err != nil {
			pw.CloseWithError(fmt.Errorf("flushing: %w", err))
			return
		}
		pw.Close()
	}()

	// atomic.WriteFile reads from pr, writes to a temp file, then
	// atomically renames it to g.Path. bufio.Reader wrapping pr
	// implements WriteTo, so io.Copy inside atomic.WriteFile avoids
	// extra allocation.
	if err := atomic.WriteFile(g.Path, bufio.NewReader(pr)); err != nil {
		return fmt.Errorf("writing atomically: %w", err)
	}

	return nil
}
```

This approach:
- Uses `io.Pipe` so `Export()` streams into `atomic.WriteFile` without buffering the full graph
- `bufio.Writer` on the write side batches small writes into the pipe
- `bufio.Reader` on the read side implements `WriteTo` so `io.Copy` avoids extra copy buffers
- Constant memory overhead (just the bufio buffers), same as the original renameio approach
- Cross-platform: `natefinch/atomic` uses `MoveFileExW` on Windows, `rename(2)` on Unix

**Step 4: Run go mod tidy**

```bash
go mod tidy
```

Expected: `google/renameio` removed from go.mod/go.sum, `natefinch/atomic` present.

**Step 5: Run existing tests**

```bash
go test ./...
```

Expected: all tests pass. The existing `encode_test.go` / integration tests exercise `Save()` and `LoadSavedGraph()`.

**Step 6: Cross-compile to verify Windows builds**

```bash
GOOS=windows GOARCH=amd64 go build ./...
```

Expected: builds successfully (previously failed due to renameio).

**Step 7: Commit**

```bash
git add -A
git commit -m "fix: replace renameio with natefinch/atomic for Windows support

Use io.Pipe + bufio to stream Export into atomic.WriteFile without
buffering the full graph in memory. natefinch/atomic supports Windows
via MoveFileExW.

Closes coder/hnsw#9"
```

**Step 8: Push and tag**

```bash
git push -u origin fix/windows-atomic-save
```

---

### Task 3: Wire floop to use the fork

**Files:**
- Modify: `go.mod` (add replace directive)

**Step 1: Add replace directive to go.mod**

Add this line after the first `require` block:

```
replace github.com/coder/hnsw v0.6.1 => github.com/nvandessel/hnsw fix/windows-atomic-save
```

Note: if the branch ref doesn't resolve cleanly, use the specific commit hash instead:

```
replace github.com/coder/hnsw v0.6.1 => github.com/nvandessel/hnsw <commit-hash>
```

**Step 2: Run go mod tidy**

```bash
GOWORK=off go mod tidy
```

Expected: go.sum updated with nvandessel/hnsw and natefinch/atomic entries. `google/renameio` should be removed from go.sum (no longer transitively needed).

**Step 3: Verify build**

```bash
GOWORK=off go build ./...
```

Expected: builds successfully.

**Step 4: Run tests**

```bash
GOWORK=off go test ./internal/vectorindex/...
```

Expected: all 11 HNSW tests + 7 tiered tests + brute-force tests pass.

**Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "build: point coder/hnsw to nvandessel/hnsw fork

The fork replaces google/renameio with natefinch/atomic for
cross-platform atomic file writes. This is a prerequisite for
removing the Windows brute-force fallback.

Ref #174"
```

---

### Task 4: Remove Windows build-tag workaround

**Files:**
- Delete: `internal/vectorindex/hnsw_windows.go`
- Modify: `internal/vectorindex/hnsw.go` (remove build tag on line 1)
- Modify: `internal/vectorindex/hnsw_test.go` (remove build tag on line 1)

**Step 1: Remove the build tag from hnsw.go**

Delete line 1 (`//go:build !windows`) and the blank line after it from `internal/vectorindex/hnsw.go`.

Before:
```go
//go:build !windows

package vectorindex
```

After:
```go
package vectorindex
```

**Step 2: Remove the build tag from hnsw_test.go**

Delete line 1 (`//go:build !windows`) and the blank line after it from `internal/vectorindex/hnsw_test.go`.

Before:
```go
//go:build !windows

package vectorindex
```

After:
```go
package vectorindex
```

**Step 3: Delete hnsw_windows.go**

```bash
rm internal/vectorindex/hnsw_windows.go
```

**Step 4: Verify it compiles for all platforms**

```bash
GOWORK=off go build ./...
GOWORK=off GOOS=windows GOARCH=amd64 go build ./...
GOWORK=off GOOS=darwin GOARCH=arm64 go build ./...
```

Expected: all three pass. No more build-tag split.

**Step 5: Run the full test suite**

```bash
GOWORK=off go test -race ./...
```

Expected: all tests pass including HNSW persistence tests.

**Step 6: Commit**

```bash
git add internal/vectorindex/hnsw.go internal/vectorindex/hnsw_test.go
git rm internal/vectorindex/hnsw_windows.go
git commit -m "feat: remove Windows HNSW fallback, enable native HNSW on all platforms

With the nvandessel/hnsw fork replacing google/renameio with
natefinch/atomic, the coder/hnsw library now builds on Windows.
Remove the brute-force fallback and build tags.

Closes #174"
```

---

### Task 5: Final validation

**Step 1: Run full CI-equivalent checks**

```bash
GOWORK=off go vet ./...
GOWORK=off go test -race -count=1 ./...
```

**Step 2: Verify no renameio references remain**

```bash
grep -r "renameio" . --include="*.go" || echo "Clean: no renameio references"
grep -r "hnsw_windows" . --include="*.go" || echo "Clean: no hnsw_windows references"
grep -r "go:build.*windows" internal/vectorindex/ || echo "Clean: no windows build tags in vectorindex"
```

Expected: all three print "Clean" messages.

**Step 3: Verify the EMBEDDINGS.md docs are still accurate**

Check `docs/EMBEDDINGS.md` for any references to "Windows falls back to brute-force" and update if present.

**Step 4: Commit docs update if needed**

```bash
git add docs/EMBEDDINGS.md
git commit -m "docs: update EMBEDDINGS.md to reflect cross-platform HNSW support"
```
