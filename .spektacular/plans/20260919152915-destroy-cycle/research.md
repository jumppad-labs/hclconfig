---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Research: 20260919152915-destroy-cycle

## Alternatives considered and rejected

- **Reuse `buildDestroyDAG` as is (edges from `depends_on` strings).** Rejected: `buildDestroyDAG` (`internal/parser/dag.go:101-167`) matches `types.GetDependencies` entries against resource IDs by exact string. Saved `depends_on`/`meta.links` hold unresolved, module-relative references with attribute paths (`resource.network.onprem.meta.name`, asserted in `internal/parser/parse_test.go:161-172`). Module-level references are expanded, and the "resource in a module depends on its module" edge is added, only while the create graph is built (`internal/parser/util.go:529-575`). Neither is persisted. For a block with an explicit `depends_on`, decode overwrites `DependsOn` with only the explicit list (`internal/parser/registered_types_test.go:400`). Exact matching therefore misses most real edges, and destroy order would be wrong.
- **Resolve parents at destroy time from the saved `depends_on`/`links`.** Rejected: this would re-run the create graph's resolution against the saved state. The spec says "Destroy needs no configuration", and module expansion needs the module structure (`FindModuleResources` with the parent-module prefix, `util.go:535-539`). It would duplicate `getResourceDependencies` and drift from it. The spec constraint says each resource must *record* its parents in the saved state.
- **Build a separate reversed graph (edges child → parent) and walk it forwards.** Rejected in favour of building the same parent → child graph that create uses and walking it with `dag.Walker.Reverse = true` (`internal/dag/walk.go:36-38`, `edgeParts` at `:306`). The spec's Technical Approach says "walks the same dependency graph as create, backwards". The walker already skips everything upstream of a failed vertex (`walk.go:375-398`), so a parent is never visited after a child fails.
- **Save progress by rebuilding state at the end (the `applyProgress.buildState` style).** Rejected for destroy: the spec requires a save as *each* resource is destroyed, so an interrupted process resumes. `applyProgress` (`internal/parser/progress.go`) only builds state after a failed walk, and `Config.Apply` saves once (`config.go:127-131`).
- **Load keeps unknown-type entries as raw JSON / leaves Load as is.** Rejected (user decision): `FileStateStore.Load` silently skips entries whose type isn't registered (`state/file_state_store.go:80-84`). The first per-resource save during a destroy would then rewrite the file without them, orphaning real resources. The user chose "Load fails on unknown types": Load returns an error naming them, and Destroy and Apply stop before touching anything.
- **Put the empty-configuration check in `parseAndValidate`.** Rejected: `Validate` shares `parseAndValidate`, and the spec only makes *applying* an empty configuration an error. The check belongs in `Parser.Apply` after parse/validate and before any destroy, create or save.

## Chosen approach — evidence

- `destroyWalkCallback` (`internal/parser/callbacks.go:183-260`) already calls `adapter.Destroy(ctx, json, false)`, fires destroy start/success/error events, and skips builtin and registered types via `handledWithoutProvider` (`internal/parser/lifecycle.go:356-365`), which fire only a success event. Its only caller is the unit test `internal/parser/registered_types_test.go:403-427`. It still needs a skip for disabled resources and a per-resource "remove from state and save" hook.
- `dag.Walker.Reverse` (`internal/dag/walk.go:36`): the source of an edge depends on its target. With parent → child edges and `Reverse = true`, children run before parents, and a failed child marks its parents as upstream-failed (`walk.go:375-398`). Independent vertices still run in parallel.
- `buildCreateDAG` (`internal/parser/dag.go:44-97`) has the resolved dependency set (`deps` from `getResourceDependencies`, `util.go:505-576`) for each resource at `:76-94`. This is where the resolved parent IDs can be written to a new `Meta.Parents` field. `dep` can be nil when a reference points at a missing or disabled resource (`util.go:555-557`), so nils must be filtered.
- `types.Meta` (`types/resource.go:5-47`) is serialized inside `"meta"`, so a new `Parents []string \`json:"parents,omitempty"\`` field round-trips through `FileStateStore` without any other change (`state/file_state_store.go:87-96`).
- `Parser.Apply` (`internal/parser/parser.go:205-240`) has `currentState` and `previousState` in hand after `parseAndValidate`. The resources to remove are `previous − current`. On failure it already returns a partial state with the error, and `Config.Apply` saves it (`config.go:118-133`), so a failed removal can return "previous minus destroyed, failed one marked `destroy_failed`".
- `resourceLifecycle.rebuild` (`lifecycle.go:233-259`) shows the house pattern for a destroy that keeps the saved copy on failure. `callProvider` (`lifecycle.go:321-350`) is the single event/timing/error wrapper, and its error format is `"<op> failed for <id>: ..."`.
- Events: `xcl.Event` already documents the `destroy` operation (`events.go:13-14`). `example/eventlog/eventlog.go:21-42` logs start/success at Debug and error at Error, with no change needed for destroy.
- Both example providers already implement `Destroy` and log at debug (`example/plugin/internal/plugin.go:87-91`, `example/plugin/external/main.go:85-89`).
- Test support: `parser.TestPlugin` records ordered `Calls` (`"<op> <id>"`) and supports `SetDestroyError` per ID (`internal/parser/test_plugin.go:29-39,138,379-396`). The call is recorded even when the destroy fails. Errors must be set after `RegisterPlugin`, because `Init` resets everything. The lifecycle harness (`internal/parser/lifecycle_test.go:52-155`) gives a real registry and a real file state store, and applies and saves for real. The `dependent` fixture has the chain `network.first` ← `container.second` ← `container.third` plus `network.independent` (`lifecycle_test.go:43-46`).

## Files examined

- `config.go:103-134` — Apply adopts and saves the partial or new state once; `:136-142` Destroy is a stub returning nil.
- `events.go:11-67` — public `Event`, operations include destroy; `parserEventHandler` adapter.
- `options.go:22` — `WithStateStore`; with no store nothing is persisted.
- `internal/parser/parser.go:205-240` — `Parser.Apply`; `:258-361` `parseAndValidate` loads previous state (`Exists` then `Load`); `:1004-1049` `walk` builds the create DAG and runs `dag.Walker` with `Reverse=false`, and `log.SetOutput(io.Discard)`.
- `internal/parser/dag.go:34-42` — `DoYouLikeDags(rp, destroy)`, name and comment must be kept (author's note); `:44-97` `buildCreateDAG`; `:101-167` `buildDestroyDAG`, exact-string matching.
- `internal/parser/util.go:505-576` — `getResourceDependencies`: module expansion, parent-module prefix, implicit module-parent edge, nil dep for a missing ref.
- `internal/parser/callbacks.go:183-260` — `destroyWalkCallback`, unused.
- `internal/parser/lifecycle.go:48-106` — apply/run dispatch on previous status; `:233-259` rebuild; `:321-350` callProvider; `:356-365` handledWithoutProvider.
- `internal/parser/progress.go` — `applyProgress`, per-resource outcomes under a mutex; `buildState` iterates only current resources, so removed ones are dropped.
- `internal/dag/walk.go:36,306,375-398` — Reverse option; upstream failure cascade.
- `types/resource.go:5-58` — Meta and ResourceBase JSON shape; no parents field.
- `types/status.go` — five status constants.
- `types/resource_helpers.go:55-130` — Get/Set/AppendUniqueDependency.
- `state/state.go` — flat slice, `AppendResource` sets ID, `RemoveResource` matches name/type/module, `GetResources` has no lock.
- `state/file_state_store.go:16-29` — creates an empty file on construction; `:32-107` Load silently skips unknown/malformed entries; `:124-144` Save removes then writes (not atomic).
- `plugins/registry/plugin_registry.go:72-85` — `CreateResource`: builtin → registered → plugin types.
- `internal/parser/test_plugin.go` — recording provider with ordered Calls and SetDestroyError.
- `internal/parser/lifecycle_test.go:43-155,641-675,708-848` — harness, statuses check, rebuild/destroy tests.
- `internal/parser/registered_types_test.go:39-135,275-291,403-427` — registered harness; removed-block test (today drops silently); destroy callback unit test.
- `internal/parser/parse_test.go:1235-1251` — requireEvent/requireBefore; `:1268-1462` commented-out legacy `p.Destroy()` tests for an API that no longer exists.
- `config_validate_test.go:27-56,263-299` — `setupConfig` with a mock store; saved state read from mock `Save` calls.
- `config_events_test.go:18-43` — `eventRecorder` for Config-level events.
- `config_test.go:33-456` — commented-out legacy Config tests.
- `example/configonly/main.go:39-91`, `main_test.go` — `run(out, log, dir)`, no state store, recordingLogger; "nothing above debug" test.
- `example/plugin/main.go:55-118`, `main_test.go:31-51` — `TestMain` builds the external binary into a temp dir; `run(out, log, dir, externalPlugin)`, no state store.
- `example/eventlog/eventlog.go:21-42` — debug for start/success, error for error.
- `example/config/main.xcl`, `modules/db/db.xcl` — 10 resources including a module, explicit `depends_on`, and cross-module refs.
- `docs/state.md`, `docs/parser-lifecycle.md` (#destroy), `docs/overview.md:83-89`, `docs/plugin-developer-guide.md:98-131,212-224` — all state that destroy is a stub and removed resources are not destroyed; they need updating.

## External references

- None needed. The DAG walker is the in-repo fork `internal/dag` (from hashicorp/terraform's dag); its Reverse semantics were read from source.

## Prior plans / specs consulted

- `20260918165700-provider-lifecycle-read` (plan) — introduced rebuild (destroy saved copy then create), `destroy_failed` retry, `callProvider`, `applyProgress`, and the "state from a real apply" test style. It explicitly made removed-resource destroy a non-goal, which this plan now delivers.
- `20260919120639-config-only-types-and-examples` (plan) — introduced `TypeRegistry`/`handledWithoutProvider`, made `destroyWalkCallback` skip registered types, and built both examples with `run(...)` and recordingLogger tests. It listed Config.Destroy and removed-resource destroy as deliberate gaps.
- `20260919152915-destroy-cycle` (spec) — source of truth for this plan.

## Open assumptions

- The walker's upstream-failure cascade applies with `Reverse = true` exactly as with forward walks (it is edge-direction agnostic in `walkVertex`/`waitDeps`). Confirm with a test before building on it.
- `dag.Walker` needs at least a root vertex. An empty destroy graph (nothing saved) is short-circuited before walking.
- Old saved state without `meta.parents` is not supported for ordering (the spec gives no guarantee). Such resources are treated as having no parents.
- Making `FileStateStore.Load` fail on unknown types also makes `Apply` fail when a previously used plugin is no longer registered. This is accepted, since the user chose it for both.
- Disabled resources are never passed to a provider on destroy, even if an earlier apply created them while enabled. Disabling an existing resource today already replaces its saved copy (`callbacks.go:60-63`), and that behaviour is out of scope.
- Resources saved as `failed` (create failed) are passed to the provider's Destroy, like `created`/`updated`/`destroy_failed`, because a partial create may exist. This matches rebuild, which destroys `failed` resources.

## Drafting assumptions

### Chosen direction: one parser destroyer, create graph walked in reverse (architecture)
- **Decision**: A parser `destroyer` builds a parent→child graph from saved `meta.parents`, walks it with `dag.Walker.Reverse = true` using the reworked `destroyWalkCallback`, and saves state after each resource. `Parser.Destroy` (whole state) and `Parser.Apply` (removed resources, before the create walk) both use it. `Config.Destroy` loads the state and delegates.
- **Rationale**: Follows the spec's "same graph, backwards" direction. The walker's upstream-failure skip enforces "never destroy a parent after a child fails". One component gives identical order, events and retry for destroy and removal.
- **Rejected**: (B) A Config-level sequential topological loop: duplicates graph logic and loses parallelism. (C) Extending `applyProgress` to build the state at the end: breaks the per-resource save requirement.
- **Key design decisions**: Parents are resolved once at create time and stored in `meta.parents` (sorted, nils dropped). Removals also save per resource through the store, so an interrupted apply doesn't re-destroy. A failed removal returns the working state and never starts the create walk. The empty-config check lives in `Parser.Apply` only. `FileStateStore.Load` errors on unknown types.

### Parents recorded as a new Meta.Parents field (discovery)
- **Decision**: Record resolved parent IDs in a new `meta.parents` field, written from the resolved dependency set while the create graph is built.
- **Rationale**: The spec requires the saved state to record parents. The existing `depends_on`/`links` hold unresolved references with attribute paths, and the module-parent edge is never stored.
- **Rejected**: Re-resolving `depends_on` at destroy time (needs config structure, duplicates resolution logic); reusing `buildDestroyDAG` exact matching (misses most edges).

### Failed and destroy_failed resources get a provider Destroy (discovery)
- **Decision**: During destroy, every provider-handled, non-disabled resource is passed to Destroy whatever its status.
- **Rationale**: A failed create may have left something behind. Rebuild already destroys failed resources.
- **Rejected**: Skipping `failed` resources (risks orphans).

### Disabled resources never reach a provider on destroy (discovery)
- **Decision**: Disabled resources are removed from state without a provider call.
- **Rationale**: The spec lists disabled blocks as provider-less for destroy.
- **Rejected**: Destroying disabled resources that were created earlier (that orphan comes from how apply treats disabling, which is out of scope).

### Removal during Apply saves per resource too (architecture)
- **Decision**: The destroyer run for removed resources during Apply saves the state after each resource, as Destroy does. Config.Apply still saves the final state.
- **Rationale**: One shared behaviour. If an apply is interrupted mid-removal, the next apply doesn't send a second Destroy for resources that are already gone.
- **Rejected**: Saving only at the end of Apply: simpler, but a crash would re-destroy resources that were already removed.

### Load errors only on unknown types, malformed entries unchanged (architecture)
- **Decision**: `FileStateStore.Load` returns an error naming the unknown types. Entries that are malformed or have no meta keep today's skip behaviour.
- **Rationale**: The user's decision covered unknown types, the realistic case (plugin not loaded). Malformed entries can't be destroyed anyway.
- **Rejected**: Failing on malformed entries too: beyond what was decided, and it may break existing tolerance.

### Conventions selected (architecture)
- **Decision**: Applied: test-state-from-real-apply, testing-and-mocking, patterns-and-architecture (DI), code-style, development-standards (structured logs), never-modify-dependencies, dependencies. Dropped: database-and-external-services (no DB), project-structure (no new top-level dirs).
- **Rationale**: These are the ones this feature's code and tests actually touch.
- **Rejected**: Listing all conventions.

### Legacy commented-out destroy tests removed (architecture)
- **Decision**: Delete the commented-out `p.Destroy()` block in `internal/parser/parse_test.go:1268-1462`, which new tests replace.
- **Rationale**: It targets an API that no longer exists, and its TODO asks for it to be rewritten.
- **Rejected**: Leaving it (dead, misleading).

### Parser.Destroy takes the saved state from its caller (components)
- **Decision**: `Parser.Destroy` receives the state to destroy. `Config.Destroy` chooses it (the store's state when the store exists, else the in-memory state) and handles the "nothing saved" case without saving.
- **Rationale**: Config already owns the in-memory state, and the parser still saves per resource through the store it was given. This keeps the no-store case working.
- **Rejected**: Parser.Destroy loading from the store itself: it couldn't serve a Config with no store.

### Typed errors for empty configuration and unknown types (data_structures)
- **Decision**: Add a sentinel `parser.ErrEmptyConfiguration` and a typed `state.UnknownTypesError` listing the unknown types.
- **Rationale**: Callers and tests can match them with errors.Is/As. This matches the existing typed errors in `state/errors.go`.
- **Rejected**: Plain fmt errors: can't be checked reliably.

### Disabled resources get a success-only destroy event (data_structures)
- **Decision**: Disabled resources fire a destroy success event only, like builtins and registered types.
- **Rationale**: The spec says blocks without a provider report success only, and it lists disabled blocks among them.
- **Rejected**: No event for disabled blocks.

### A failed save stops the branch (implementation_detail)
- **Decision**: If saving the state after a resource's destroy fails, that step counts as failed, so the walker doesn't destroy that resource's parents.
- **Rationale**: If a child's removal can't be recorded, destroying its parent risks state that says the child exists while its parent is gone, which would violate the parent/child constraint on resume.
- **Rejected**: Ignoring save errors until the end: an interrupted destroy could then resume from wrong state.

### Interrupted destroy tested from a real mid-way snapshot (testing_approach)
- **Decision**: A store wrapper records every save during a real destroy. The test restores the snapshot taken after the first save and runs a second destroy from it.
- **Rationale**: Simulates a process that stopped, without a hand-written state (the convention) or killing processes.
- **Rejected**: Killing a subprocess mid-walk (flaky); hand-writing partial state (breaks the convention).

### Examples' run returns the applied resources and takes a state path (phases)
- **Decision**: `run` gains a `statePath` parameter, applies, prints, captures the applied resources, destroys, and returns the applied resources. `main` uses a temp dir.
- **Rationale**: Existing tests assert on the returned resources, and tests need to control where the state file goes.
- **Rejected**: Returning post-destroy resources (always empty, breaks existing assertions); a fixed state path in the repo (leaves files behind).

### Empty-config fixture is a comment-only .xcl file (phases)
- **Decision**: `internal/test_fixtures/config/empty/empty.xcl` contains only a comment.
- **Rationale**: Git doesn't keep empty directories, and a file with no blocks is the case the spec describes.
- **Rejected**: A directory with a non-.xcl placeholder (tests file discovery rather than "declares no blocks").

### Atomic state writes left out (out_of_scope)
- **Decision**: Don't change `FileStateStore.Save` (remove, then write) in this plan.
- **Rationale**: The spec doesn't ask for it. Per-resource saves make the window more frequent but not new.
- **Rejected**: Switching to temp file + rename now (scope creep; could be a quick follow-up).

## Rehydration cues

- `spektacular spec file read 20260919152915-destroy-cycle.md`
- Re-read `internal/parser/parser.go:205-240,1004-1049`, `internal/parser/dag.go`, `internal/parser/callbacks.go:183-260`, `internal/parser/lifecycle.go:233-365`, `internal/parser/progress.go`, `config.go`, `state/file_state_store.go`, `internal/dag/walk.go:20-60,300-400`.
- Tests: `internal/parser/lifecycle_test.go:43-155`, `internal/parser/test_plugin.go:29-140,379-396`, `config_validate_test.go:27-56`, `example/*/main_test.go`.
- `spektacular knowledge always-applied --tier repo --filter xclconfig` (conventions: test state from a real apply, no table tests, split positive/negative).
