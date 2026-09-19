---
tags: [dependencies, go, module-cache, fork, hcl, licensing]
---

# NEVER modify dependency packages

NEVER edit the source of any package listed in `go.mod`, under any circumstances. This includes:

- files in the Go module cache (`~/go/pkg/mod/...`, i.e. `$GOMODCACHE`)
- vendored copies
- any other local copy of a third-party module

This applies to bulk find-and-replace and rename operations too: scope them to this repository's own files only.

## Why

Edits to the module cache only exist on the machine that made them. The code builds locally, but fails everywhere else (CI, other contributors, downstream importers) because Go downloads the pristine module.

On 2026-07-08 a bulk `HCL` → `XCL` rename edited `hclparse/parser.go` in the cached `github.com/hashicorp/hcl/v2@v2.21.0`, and commit `a748cfe` then called `parser.ParseXCLFile`, a function that doesn't exist upstream.

## HCL is not a dependency

HCL now lives in this repo at `internal/xcl` (imported from v2.21.0) and is edited there like any other code. It is MPL-2.0 licensed inside an Apache-2.0 repo: keep the HashiCorp MPL header on every file, add `// Modifications Copyright (c) Jumppad Labs` to files you change, never move its code into Apache-licensed files, and record changes in `internal/xcl/UPSTREAM.md`.

## If another dependency genuinely needs changing

Fork it under `jumppad-labs` and point to it with a `replace` directive in `go.mod`, as is done for `go-cty` and `dag`:

```
replace github.com/zclconf/go-cty => github.com/jumppad-labs/go-cty <version>
```

## Detection and recovery

- Detect: `go mod verify` reports `dir has been modified` for a tampered module.
- Recover: the cache is read-only, so make it writable first:

  ```sh
  chmod -R u+w ~/go/pkg/mod/<module>@<version>
  rm -rf ~/go/pkg/mod/<module>@<version>
  go mod download <module>
  ```
