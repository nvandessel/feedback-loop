# Cross-Platform CGO Release Pipeline

## Goal

Ship CGO-enabled floop binaries with LanceDB statically linked for all 6 platform/arch targets (linux/darwin/windows x amd64/arm64) on every push to main, using the existing auto-release flow.

## Scope

Three sequential pieces of work:

1. **Land PRs #203 and #204** — Cross-platform CI matrix and Windows test fixes. Establishes native runners on all 3 platforms.
2. **CGO release builds** — Rework `.goreleaser.yml` and `auto-release.yml` to produce CGO-enabled binaries via a per-platform build matrix + goreleaser pre-built binary packaging.
3. **Release notes and docs** — Replace `go install` instructions with a "build from source with LanceDB" guide.

## Architecture

### Release workflow

```
Push to main
    │
    ▼
auto-release.yml
    │
    ├── check-skip (unchanged)
    ├── tag-version (unchanged)
    │
    ├── build-matrix (NEW — replaces goreleaser's build step)
    │   ├── ubuntu-latest (x86_64):    CGO_ENABLED=1 → linux/amd64
    │   ├── ubuntu-24.04-arm (arm64):  CGO_ENABLED=1 → linux/arm64
    │   ├── macos-latest (arm64):      CGO_ENABLED=1 → darwin/arm64
    │   ├── macos-13 (x86_64):         CGO_ENABLED=1 → darwin/amd64
    │   ├── windows-latest (x86_64):   CGO_ENABLED=1 → windows/amd64
    │   └── windows/arm64:             CGO_ENABLED=0 (fallback — no arm64 Windows runner)
    │   Each runner:
    │     1. go mod download
    │     2. Download LanceDB native libs for its platform (.a + .h)
    │     3. Build with CGO + static linking + version ldflags
    │     4. Upload binary as workflow artifact
    │
    └── release (goreleaser in pre-built binary mode)
        1. Download all 6 binaries from workflow artifacts
        2. goreleaser packages into archives, checksums,
           changelog, homebrew cask, GitHub release
```

### Build matrix detail

All builds are **native** — no cross-compilation, no zig. Each runner builds for its own OS and architecture. GitHub provides runners for 5 of the 6 targets. Windows arm64 has no GitHub runner and falls back to `CGO_ENABLED=0`.

| Runner | Target | CGO | Notes |
|--------|--------|-----|-------|
| `ubuntu-latest` | linux/amd64 | Yes | Native x86_64 |
| `ubuntu-24.04-arm` | linux/arm64 | Yes | Native arm64 (free for public repos) |
| `macos-latest` | darwin/arm64 | Yes | Native M-series |
| `macos-13` | darwin/amd64 | Yes | Native Intel (last Intel macOS runner) |
| `windows-latest` | windows/amd64 | Yes | Native x86_64 |
| `windows-latest` | windows/arm64 | No | No arm64 runner; `CGO_ENABLED=0` fallback (BruteForce) |

### Static linking

LanceDB's `download-artifacts.sh` provides `.a` (static) libraries for all supported platforms. We use these exclusively. The binary is self-contained — no runtime library path needed. Users get a single file, same as today.

**Note:** Static linking of the LanceDB Rust libraries has not been validated in this project yet. The exact linker flags for each platform (system deps like `-lm -ldl -lstdc++` on Linux, `-framework Security` on macOS, `-lws2_32` on Windows) need discovery during implementation. A spike task validates this on one platform before committing to the full matrix.

Build flags per platform (to be validated):
- **All platforms**: `CGO_CFLAGS="-I./include"` (LanceDB C header)
- **Linux**: `CGO_LDFLAGS="-L./lib/linux_amd64 -llancedb_go -lm -ldl -lstdc++"`
- **macOS**: `CGO_LDFLAGS="-L./lib/darwin_arm64 -llancedb_go -framework Security -framework CoreFoundation"`
- **Windows**: `CGO_LDFLAGS="-L./lib/windows_amd64 -llancedb_go -lws2_32 -luserenv -lbcrypt"`

### Version injection

The current `.goreleaser.yml` injects version/commit/date via `-ldflags`. Since goreleaser runs in pre-built binary mode (no compilation), the build matrix jobs must pass these ldflags explicitly:

```
-ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"
```

The `VERSION` comes from the tag created in the `tag-version` step (passed as a job output). `COMMIT` and `DATE` are derived from `GITHUB_SHA` and the current timestamp.

### goreleaser pre-built binary mode

goreleaser's `builds.builder: prebuilt` configuration skips compilation. It expects binaries at a templated path and handles everything else: archives, checksums, changelog, homebrew cask, GitHub release.

```yaml
builds:
  - id: floop
    builder: prebuilt
    prebuilt:
      path: "dist/floop_{{ .Os }}_{{ .Arch }}/floop{{ .Ext }}"
    binary: floop
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
```

The build matrix uploads binaries with artifact names matching the template (e.g., `floop-linux-amd64`, `floop-darwin-arm64`). The release job downloads them into the `dist/` directory structure goreleaser expects. Windows builds must produce `floop.exe` (not `floop`) so `{{ .Ext }}` resolves correctly.

The existing `before.hooks` section (`go mod tidy`) should be removed since the release runner no longer compiles.

### Fallback strategy

If a platform's CGO build fails (e.g., LanceDB native lib download fails, linker error), that target falls back to `CGO_ENABLED=0`. This prevents a single platform issue from blocking the entire release.

Implementation: each matrix entry has a fallback step that builds with `CGO_ENABLED=0` if the CGO build exits non-zero.

### test-release.yml

Updated to validate the goreleaser pre-built binary config in snapshot mode. Runs on a single `ubuntu-latest` runner: builds `linux/amd64` natively with CGO as a smoke test, then builds the remaining 5 targets with `CGO_ENABLED=0`, and runs goreleaser snapshot to verify packaging. This avoids the full 5-runner matrix cost on every PR while still validating the goreleaser config.

### Artifact naming

Build matrix uploads use platform-specific artifact names to avoid collisions:
- `floop-linux-amd64`
- `floop-linux-arm64`
- `floop-darwin-amd64`
- `floop-darwin-arm64`
- `floop-windows-amd64`
- `floop-windows-arm64`

The release job downloads all 6 artifacts and arranges them into the `dist/floop_{os}_{arch}/` directory structure goreleaser expects.

## Dependencies

### PR #203: Cross-platform CI matrix
- Adds test + build matrix across Linux, macOS, Windows
- Currently Greptile 3/5 — needs `continue-on-error` on Windows build job (addressed by PR #204)
- Must land first to establish the CI pattern

### PR #204: Windows test fixes
- Fixes the missing `continue-on-error` on Windows build
- Greptile 4/5 — ready to merge after #203
- Must land before CGO release work begins

### LanceDB native library availability
- `download-artifacts.sh` from `lancedb-go@v0.1.2` supports linux, darwin, windows x amd64, arm64
- Static `.a` files available for all 6 targets — must be validated via spike task before full implementation

### Static linking spike
- Before implementing the full matrix, validate static linking on one platform (linux/amd64 locally)
- Determine exact `CGO_LDFLAGS` per platform
- Confirm the resulting binary is self-contained (no runtime library deps)
- This spike can be done locally before writing any CI changes

## What doesn't change

- Semantic versioning via conventional commits + svu
- Auto-release on every push to main
- Homebrew cask install experience (binary is still self-contained)
- CI test matrix from PR #203
- The `test-cgo` CI job in ci.yml (validates LanceDB code on PRs)

## macOS considerations

CGO binaries containing Rust native code may trigger Gatekeeper differently than pure-Go binaries. The existing Homebrew cask post-install hook removes quarantine attributes (`xattr -dr com.apple.quarantine`), which should handle this. If not, ad-hoc code signing (`codesign -s -`) may be needed as a build matrix step for macOS targets. Monitor after first release.

## Documentation changes

### Remove from release notes
- `go install github.com/nvandessel/floop/cmd/floop@latest` instruction

### Add
- "Building from source with LanceDB" guide covering:
  - Prerequisites: Go 1.24+, C compiler (gcc/clang)
  - Downloading native libs: `download-artifacts.sh`
  - Build command with CGO flags and ldflags
  - Verifying LanceDB is linked: `ldd` / `otool -L`
- Note that `go install` produces a BruteForce-only build (functional but without LanceDB persistence)

## Deployment note

The current `auto-release.yml` has `paths-ignore` that includes `.github/**`. A commit that only modifies workflow files and `.goreleaser.yml` won't trigger a release. Ensure the pipeline changes land alongside a code change, or temporarily remove `.github/**` from `paths-ignore` for the deployment commit.

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| LanceDB static lib linker flags vary by platform | High | Spike task validates flags before full implementation; fallback to CGO_ENABLED=0 per target |
| Windows CGO build has undiscovered issues | Medium | `continue-on-error` on Windows initially; fix forward |
| macOS Gatekeeper rejects CGO binary | Low | Quarantine removal hook exists; ad-hoc signing if needed |
| LanceDB native lib download is slow or flaky in CI | Low | Cache with `actions/cache` keyed on `lancedb-{os}-{arch}-{lance_version}` |
| Release time increases from ~2min to ~8min | Certain | Acceptable; matrix runs in parallel |
| `macos-13` (Intel) runner deprecated by GitHub | Low (>1yr) | When deprecated, evaluate zig cross-compilation for darwin/amd64 from arm64 runner |
| Windows arm64 has no CGO (BruteForce only) | Certain | Acceptable for now; revisit when GitHub adds arm64 Windows runners |

## Out of scope

- Nightly/scheduled builds (can add later if auto-release becomes too slow)
- GoReleaser Pro features (split/merge)
- Docker image builds
- zig cross-compilation (eliminated by using native runners per target)
- Windows arm64 CGO support (no runner available)
- LanceDB tombstone compaction (blocked by upstream API — separate hardening plan exists)
