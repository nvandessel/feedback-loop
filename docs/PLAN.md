# Feedback Loop - Implementation Plan

## Overview

This document tracks the implementation plan for `floop`, broken down into phases with concrete tasks and acceptance criteria.

## Current Status

**Phase**: Foundation (Phase 1)
**Goal**: Working CLI that can `init`, `version`, `list`, and store behaviors in memory.

## Phase 1: Foundation

### Task Order (with dependencies)

```
feedback-loop-mz2: Core data models
         ↓
feedback-loop-0k8: GraphStore + InMemoryGraphStore
         ↓
feedback-loop-0z3: CLI (version, init, list)
         ↓
feedback-loop-jq3: Minimal dogfooding skeleton (learn stub)
```

### 1. Core Data Models (`feedback-loop-mz2`)

**Files**:
- `internal/models/behavior.go`
- `internal/models/correction.go`
- `internal/models/context.go`
- `internal/models/provenance.go`

**Acceptance Criteria**:
- [ ] `Behavior` struct with ID, Name, Kind, When, Content, Provenance, Confidence, Priority, relationships
- [ ] `BehaviorKind` enum: directive, constraint, procedure, preference
- [ ] `BehaviorContent` with Canonical, Expanded, Structured fields
- [ ] `Correction` struct with context, agent action, human response, corrected action
- [ ] `ContextSnapshot` with Matches() method for predicate evaluation
- [ ] `Provenance` with SourceType enum (authored, learned, imported)
- [ ] All structs have proper JSON/YAML tags

### 2. GraphStore Interface (`feedback-loop-0k8`)

**Files**:
- `internal/store/store.go`
- `internal/store/memory.go`
- `internal/store/memory_test.go`

**Acceptance Criteria**:
- [ ] `GraphStore` interface with Node/Edge CRUD operations
- [ ] `Node` and `Edge` types for graph storage
- [ ] `Direction` type for edge traversal
- [ ] `InMemoryGraphStore` implementation passing basic tests
- [ ] Tests for: AddNode, GetNode, QueryNodes, AddEdge, GetEdges, Traverse

### 3. CLI Commands (`feedback-loop-0z3`)

**Files**:
- `cmd/floop/main.go`

**Acceptance Criteria**:
- [ ] `floop version` prints version string
- [ ] `floop init` creates `.floop/` directory with `manifest.yaml`
- [ ] `floop list` shows all behaviors (empty initially)
- [ ] `floop list --json` outputs JSON for agent consumption
- [ ] Global `--json` flag support
- [ ] Global `--root` flag for project root

**Success Test**:
```bash
go build ./cmd/floop && ./floop init && ./floop list --json
```

### 4. Dogfooding Skeleton (`feedback-loop-jq3`)

**Purpose**: Enable using floop during development before full implementation.

**Acceptance Criteria**:
- [ ] `floop learn --wrong "X" --right "Y"` captures correction (stub - logs to file)
- [ ] Can be invoked by agent during development
- [ ] Foundation for iterative enhancement

---

## Phase 2: Learning Loop (Future)

Dependencies: Phase 1 complete

**Tasks**:
- `internal/learning/capture.go` - CorrectionCapture
- `internal/learning/extract.go` - BehaviorExtractor
- `internal/learning/place.go` - GraphPlacer
- `internal/learning/loop.go` - LearningLoop orchestrator
- `floop learn` command fully implemented

---

## Phase 3: Activation (Future)

Dependencies: Phase 2 complete

**Tasks**:
- `internal/activation/context.go` - ContextBuilder
- `internal/activation/evaluate.go` - Predicate evaluation
- `internal/activation/resolve.go` - Conflict resolution
- `floop active`, `floop show`, `floop why` commands

---

## Phase 4: Persistence (Future)

Dependencies: Phase 3 complete

**Tasks**:
- `internal/store/beads.go` - BeadsGraphStore implementation
- Switch default storage from memory to Beads

---

## Notes

- **Dogfooding**: We use floop while building floop
- **Beads**: All work tracked via `bd` commands
- **Iteration**: Each phase builds on the previous, tested incrementally
