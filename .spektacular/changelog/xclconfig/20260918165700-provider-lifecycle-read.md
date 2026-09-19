---
created_date: "2026-09-19"
document_status: draft
project: xclconfig
spec: 20260918165700-provider-lifecycle-read
plan: 20260918165700-provider-lifecycle-read
---

# Provider lifecycle: Read, computed fields and failed-apply recovery

Applying the same configuration twice no longer recreates everything. xcl reads each existing resource through its provider and updates only what changed or drifted. When an apply fails, xcl saves the progress it made and rebuilds failed resources on the next run. Plugin authors implement `Read(ctx, old, new)` in place of `Refresh`, and can mark provider-owned fields as computed.

> Derived from project xclconfig, spec/plan 20260918165700-provider-lifecycle-read. See the project-level record for the full feature.

## What changed in this repo

- **Lifecycle** (`internal/parser`):
  - A new resource lifecycle picks create, read→changed→update, or rebuild (destroy then create) for each resource from the state saved by the last apply.
  - A read that reports not found recreates the resource. Any other read error marks the resource `failed` and fails the apply.
- **Failed applies** (`internal/parser`, `config.go`): each resource's outcome is recorded. When the apply fails, reached resources, the failed one and unreached previous entries are saved. New resources that weren't reached are left out. `Config.Apply` saves that state before returning the error.
- **Provider contract** (`plugins`, breaking):
  - `Read(ctx, old, new)` replaces `Refresh`.
  - `plugins.ErrNotFound` still matches with `errors.Is` after crossing gRPC, through a new `not_found` field on `ReadResponse`.
  - `plugins.DefaultChanged[T]` can be embedded for change detection.
  - The proto and mocks are regenerated.
- **Statuses and events** (`types`, `logger`, `internal/parser`):
  - The five status constants are `created`, `updated`, `failed`, `destroyed` and `destroy_failed`.
  - The pretty printer colours match them.
  - Parser events use `read`.
- **Computed fields and warnings** (`internal/parser`):
  - The `xcl:"computed"` and `xcl:"key"` tags work at any depth.
  - Validation rejects configured computed fields and computed fields that aren't optional.
  - Computed values carry over before `Read`.
  - A structured warning is logged when a provider changes a configured value.
- **Example plugin** (`plugins/example`): now the reference provider, with `DefaultChanged`, a computed `PersonID` and a not-found demonstration.
- **Docs**: the finished plugin developer guide, plus updated lifecycle, state, overview and plugin docs.

## Why

Previously the saved state was loaded and thrown away, so every apply recreated every resource, and a failed apply lost all of its progress. Plugin authors had no written contract for reading real resources, reporting missing ones or owning computed values.

## Deviations from the plan

- Some work moved to earlier phases so the module builds at every phase boundary.
- When `Read` reports not found, the resource is reset to its configured copy before create.
- Parallel walks exposed unsynchronised event collection in the tests, so the event tests now lock it.
- Destroying resources removed from configuration is still not implemented.
