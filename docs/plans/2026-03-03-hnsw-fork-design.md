# Fork coder/hnsw for cross-platform HNSW support

**Date:** 2026-03-03
**Issue:** #174
**Status:** Approved

## Problem

`coder/hnsw` depends on `google/renameio` which does not compile on Windows.
Windows builds fall back to `BruteForceIndex` via build tags. The upstream repo
is semi-abandoned (last push July 2025, open panic bugs unaddressed, hostile to
AI-generated contributions).

## Decision

Fork `coder/hnsw` to `nvandessel/hnsw`. Replace `google/renameio` with
`natefinch/atomic` using `io.Pipe` + `bufio` for memory-efficient atomic writes.
Wire in via `go.mod` replace directive. Remove Windows build-tag workarounds.

### Alternatives considered

| Option | Verdict |
|--------|---------|
| Roll our own HNSW | Core algorithm doesn't change per use case; high effort, medium risk |
| Vendor/internalize | Similar outcome to fork but loses git history and upstream tracking |
| Keep brute-force fallback | Real Windows users exist; doesn't scale past 1000 vectors |
| Contribute upstream | Maintainer hostile to PRs, repo semi-abandoned |

### Why rebuild-on-mutation is fine

- `Remove()` has zero production callers
- `Add()` with existing key only during merges (rare, backgrounded)
- Typical scale: 50-500 behaviors (below HNSW threshold)
- At 1000 vectors: rebuild ~50-200ms, backgrounded behind RWMutex
- Only safe approach given upstream Delete bug (dangling pointer panics)

## Changes

### In the fork (nvandessel/hnsw)

- Replace `renameio.TempFile` + `CloseAtomicallyReplace` with `atomic.WriteFile`
- Use `io.Pipe` + `bufio.NewWriter` to avoid doubling peak memory
- Remove `google/renameio` dependency, add `natefinch/atomic`

### In floop

- `go.mod`: add `replace github.com/coder/hnsw => github.com/nvandessel/hnsw <ref>`
- Delete `internal/vectorindex/hnsw_windows.go`
- Remove `//go:build !windows` from `hnsw.go` and `hnsw_test.go`
- Existing tests validate correctness (no new tests needed for the swap)

## Future optionality

- Strip unused API surface from fork
- Fix Delete bug properly if incremental mutations ever needed
- Remove replace directive if upstream merges the fix
