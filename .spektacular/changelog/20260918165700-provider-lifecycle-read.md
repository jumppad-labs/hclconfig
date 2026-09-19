---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Provider lifecycle: Read, computed fields and failed-apply recovery

## What was built

xcl's apply now uses the state saved by the last apply. Before this change, every apply recreated every resource. Previous state was loaded and then thrown away, and a failed apply saved nothing.

**Lifecycle.** A new resource lifecycle component chooses the provider calls for each resource from that resource's entry in the previous state:

- **New resource**: `Create`. It is never read first.
- **Saved as `created` or `updated`**:
  1. Computed values saved last time are carried onto the configured resource.
  2. The provider's `Read(old, new)` reports the real resource.
  3. `Changed(old, read result)` decides whether to `Update`.
  4. An unchanged resource keeps what was read and its previous status.
  5. `Read` returning `plugins.ErrNotFound` recreates the resource. Any other read error fails the apply.
- **Saved as `failed` or `destroy_failed`**: the resource is rebuilt. It is destroyed using its saved copy, then created. If that destroy fails, the resource is saved as `destroy_failed` and retried on the next apply.

**Failed applies keep their progress.** The walker already skips the dependents of a failed resource and lets independent ones finish. Each resource's outcome is now recorded as it is reached. When the apply fails, the saved state holds:
- succeeded resources, with their new values and status;
- the failing resource, as `failed`;
- previously existing resources that weren't reached, with their previous entry.

New resources that were never reached are left out. `Config.Apply` saves that state and then returns the error. Invalid configuration still saves nothing.

**Provider contract (breaking).**
- `Refresh(ctx, r)` is replaced by `Read(ctx, old, new)` through the provider interface, adapters, hosts and the gRPC protocol. `rpc Read` carries both copies, and `ReadResponse` has a `not_found` flag, so `errors.Is(err, plugins.ErrNotFound)` works for in-process and external plugins alike.
- `plugins.DefaultChanged[T]` can be embedded to get change detection: a JSON comparison that ignores xcl's `meta`, `depends_on` and `disabled`.
- Resource statuses are exactly `created`, `updated`, `failed`, `destroyed` and `destroy_failed`, defined as constants in `types`.
- Parser events name the read operation `read`.

**Computed fields.**
- **Tagging.** Resource types can tag provider-owned fields `xcl:"computed"` (they must also be `optional`), at any depth, and mark list-element identity with `xcl:"key"`.
- **Validation.** Setting a computed field in configuration is a validation error that names the resource and the field path, reported before any provider call.
- **Carry-over.** Saved computed values are carried over before `Read`, so they survive unchanged applies and dependents can reference them.

**Configured-value warning.** When a provider changes a literally configured, non-computed value in Create, Read or Update, xcl logs a structured warning naming the resource and field. Fields set by reference are exempt.

**Example plugin and docs.**
- The example provider is now the reference implementation. It embeds `DefaultChanged`, never changes configured fields, has a computed `PersonID`, and shows `ErrNotFound`.
- The plugin developer guide is finished, and the parser lifecycle, state, overview and plugin docs match the shipped behaviour.

## Why it matters

A second apply of unchanged configuration now makes no create or update calls. Configuration edits and drift that `Read` observes each lead to exactly one update. A failed apply no longer forgets what it built: the next apply picks up where it stopped and never recreates something that already exists. Plugin authors get one clear job, implementing `Read`, plus a written contract covering `ErrNotFound`, computed fields and default change detection.

## Deviations from the plan

- **Work moved between phases** so the module builds at every phase boundary: the dead `internal/parser/plugins.go` was deleted and the mocks regenerated in 1.1, and the gRPC `errors.New(resp.Error)` fix landed early.
- **Not-found handling:** when `Read` reports not found, the resource is reset to its configured copy (computed values dropped) before `Create`, so stale provider identity is never sent to `Create`.
- **Test fixes:** the event tests in `parse_test.go` now guard their collected slices with a mutex. Parallel walks made them lose events now and then.
- **Nested-field warning:** the nested configured-value warning is covered by unit tests rather than a scenario test.
- **Open question closed:** `DefaultChanged` needed no `null`/empty normalisation.
- **Still out of scope:** destroying resources removed from configuration.

## Known pre-existing issues (not changed)

- `TestPluginResourceCreationWithFallback` panics on a nil plugin registry.
- The `errors` highlighting tests fail.
- The root file-location tests are missing fixtures.
- The test packages in `example`, `internal/functions` and `plugins/registry` don't build.
- There is a data race in `walkCallback`'s module-disabled handling.
- The mockery binary installed locally needs a go1.25 toolchain to run.
