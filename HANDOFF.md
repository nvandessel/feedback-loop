# Session Handoff - 2026-01-28

## What We Accomplished ✅

### 1. Completed Tasks (Parallel Workflow!)
- **feedback-loop-ek2.5**: Added `--global` and `--all` flags to `floop list` command
- **feedback-loop-ek2.6**: Updated query commands (active, show, why, prompt) to use MultiGraphStore
- **feedback-loop-ek2.8**: Updated curation commands (forget, deprecate, restore, merge) to use MultiGraphStore

All three completed in parallel using git worktrees - wall time ~3 minutes for 2 tasks!

### 2. Established Parallel Worktree Workflow
Created and documented a workflow for running multiple agents simultaneously:
- Created worktrees as siblings: `git worktree add -b branch ../sibling-dir main`
- Generated detailed TASK.md files with line numbers, success criteria, commit templates
- Spawned background agents that worked autonomously
- Reviewed and cherry-picked commits back to main
- All changes pushed to origin

### 3. Enhanced Agent Learning System
- Updated AGENTS.md to instruct agents to run `floop prompt --task development` at session start
- Captured 4+ new behaviors about parallel workflows, task selection, worktree patterns
- Kept CLAUDE.md minimal (just "Read AGENTS.md") - open format principle

## Critical Bug Discovered 🐛

**Issue**: `floop learn --scope global` and `--scope both` are **not saving behaviors to the global store**

**Evidence**:
- Corrections ARE captured to corrections.jsonl ✅
- Behaviors ARE extracted (shows "Auto-accepted") ✅
- But behaviors NOT appearing in `~/.floop/nodes.jsonl` ❌
- Running `floop list --global` doesn't show recently learned behaviors ❌

**Impact**:
- Global learnings aren't persisting across projects
- The feedback loop is broken for cross-project behaviors
- AGENTS.md instruction to load behaviors won't work until this is fixed

## Next Task: feedback-loop-ek2.9 (IN PROGRESS)

**Task**: Integration testing for global/local storage
**Status**: Claimed, ready to work
**Estimate**: 1 hour
**Priority**: P1 (blocks documentation)

### Test Scenarios (from beads task)
1. ✅ Initialize both scopes: `floop init && floop init --global`
2. ❌ Learn globally: `floop learn --scope global --wrong X --right Y` (FAILING - this will expose the bug!)
3. Learn locally (override): `floop learn --scope local --wrong Y --right Z`
4. Verify local wins: `floop active --file test.py`
5. Test list commands: `floop list`, `floop list --global`, `floop list --all`
6. Test backwards compatibility: existing .floop/ works unchanged
7. Test edge cases: missing global dir, partial failures, concurrent access

### ⚠️ CRITICAL: Safe Testing Approach

**DO NOT test against real `~/.floop/`!** That's production data.

Instead, use environment variable override:
```bash
# Option 1: Test in isolated directory
export HOME=/tmp/floop-integration-test
mkdir -p $HOME
floop init --global  # Creates /tmp/floop-integration-test/.floop/

# Option 2: Mock the global path (if supported)
# Check internal/store/store.go for GlobalFloopPath() implementation
```

Or modify the test to use `--root` for both local and a test global dir.

### Debugging the Bug

When you run test scenario #2 and it fails, investigate:

1. **Check if behavior is created**:
   ```bash
   cat $HOME/.floop/nodes.jsonl | wc -l  # Before
   floop learn --scope global --wrong "test" --right "test2" --task "test"
   cat $HOME/.floop/nodes.jsonl | wc -l  # After - should increment
   ```

2. **Check MultiGraphStore.AddNode** (internal/store/multi.go):
   - Line 63-94: Does it actually write to globalStore when scope=global?
   - Is Sync() being called?
   - Are there silent errors?

3. **Check LearningLoop** (internal/learning/loop.go):
   - Does it call graphStore.AddNode()?
   - Is the returned error being checked?

4. **Add debug logging**:
   ```go
   fmt.Fprintf(os.Stderr, "DEBUG: Writing to scope=%s, nodeID=%s\n", scope, nodeID)
   ```

## Learnings Captured This Session

1. **Parallel worktree automation** - create setup/cleanup scripts
2. **Task selection criteria** - verify no deps, different functions, similar scope
3. **Worktree location** - use `../sibling` not `subdir`
4. **Agent instructions** - detailed TASK.md with line numbers and templates
5. **CLAUDE.md minimalism** - keep minimal, use AGENTS.md (open format)

## State of the Repo

```
Latest commits:
e3044ef docs: add behavior loading instruction to AGENTS.md
789b61a chore(beads): close ek2.6 and ek2.8 tasks
43430d6 feat(cli): update curation commands to use MultiGraphStore
d4fd3ad feat(cli): update query commands to use MultiGraphStore
f23ca93 feat(cli): add --global and --all flags to list command
```

All tests passing: `go test ./...`
All changes pushed to origin ✅

## Quick Start for Next Agent

```bash
# 1. Load learned behaviors (once AGENTS.md is read)
./floop prompt --task development --format markdown

# 2. Check current task
bd show feedback-loop-ek2.9

# 3. Set up safe test environment (DON'T use real ~/.floop/)
export HOME=/tmp/floop-test-$(date +%s)
mkdir -p $HOME

# 4. Run integration tests from task description
# 5. Find and fix the bug
# 6. Verify fix works
# 7. Close task and push
```

## Questions to Resolve

1. Why isn't MultiGraphStore.AddNode() saving to global when scope=global?
2. Is there error handling swallowing failures?
3. Should we add integration tests as actual Go test files?
4. Do we need a dry-run or test mode for floop commands?

## Resources

- Task details: `bd show feedback-loop-ek2.9`
- MultiGraphStore code: `internal/store/multi.go`
- Learn command: `cmd/floop/main.go` lines 156-300
- Test all commands: `go test ./cmd/floop -v`

---

**Context**: 142k/200k tokens used (71%)
**Time**: ~2 hours productive work
**Mood**: Excited about parallel workflows, concerned about global storage bug 🐛
