---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Plan: 20260919152915-destroy-cycle

<!-- Metadata -->
<!-- Created: 2026-09-19T16:08:02Z -->
<!-- Commit: 3c3de68 -->
<!-- Branch: v2 -->
<!-- Repository: git@github.com:jumppad-labs/hclconfig.git -->

## Overview

Applications built on XCL get a real teardown. `Config.Destroy` removes everything XCL created, working only from the saved state and destroying dependents before the things they depend on. A block removed from the configuration is now destroyed through its provider on the next apply, instead of being silently forgotten while the real resource keeps running. Anything that fails to be destroyed stays in the saved state as `destroy_failed` for a later retry, so developers and their users never lose track of a resource that is still running.

## Conventions

- **Generate test state with a real apply, not a hand-written state file** — every destroy, removal and retry scenario starts from state written by a real apply, a failing destroy or an edited config. The one exception is the Load test for unknown types, which is about the state format itself.
- **Testing & mocking: testify `require`, no table-driven tests, never mix positive and negative cases, favour verbosity** — each acceptance criterion and success metric gets its own named test, with failure cases in separate functions.
- **Patterns & architecture: use dependency injection for testability** — the destroyer takes the provider resolver, type registry and state store through `ParserOptions`, so ordering and failures can be driven by the recording `TestPlugin` and a real file store.
- **Code style: standard Go conventions, handle every error explicitly, descriptive names, `any` over `interface{}`** — save errors, provider errors and Load errors are all returned and joined, never dropped.
- **Development standards: structured logging** — the spec requires destroy messages at debug and errors at error. Examples log destroy events through the structured `eventlog` handler.
- **Never modify dependency packages** — the walker's `Reverse` option in the in-repo `internal/dag` fork is used as is, and nothing in the module cache is touched.
- **Dependencies: prefer the standard library** — no new third-party dependency. Sorting parent IDs uses `slices`/`sort`.

## Architecture & Design Decisions

All work lands in the single registered repo, `xclconfig` (root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).

**One destroy component in the parser, used by both Destroy and Apply.** A new parser component, the *destroyer* (`internal/parser/destroy.go`), destroys a given set of resources held in a working copy of the saved state. It builds the same kind of graph create uses, with edges from parent to child and resources that have no parent hanging off a root. It then walks that graph with `dag.Walker.Reverse = true`, so every child runs before its parents. The walker already skips every vertex upstream of a failed one and still runs unrelated vertices in parallel (`internal/dag/walk.go:375-398`). That gives the spec's two ordering rules for free: dependents are destroyed first, and a parent is never touched once a child's destroy has failed. The per-vertex work is the existing `destroyWalkCallback` (`internal/parser/callbacks.go:183`), reworked rather than duplicated. It already calls the provider, fires `destroy` start/success/error events, and skips builtin and registered types, which get a success event only. It gains a skip for disabled resources, and a hook that, under a mutex, removes each destroyed resource from the working state and saves it through the `StateStore`. A resource whose destroy fails is marked `destroy_failed` and saved too, so the saved state is correct at every step and an interrupted process resumes from it. `Parser.Destroy` runs the destroyer over the whole saved state. `Parser.Apply` runs it over the resources that are in the previous state but no longer in the configuration, before the create walk starts. If a removal fails, Apply returns the working state and the error, and never starts the create walk. Otherwise the create walk proceeds against the previous state minus the removed resources. `Config.Destroy` stops being a stub. It loads the saved state (or uses the in-memory state when no store is configured), calls `Parser.Destroy`, adopts what is left and returns the error. When nothing has been saved, it returns immediately without saving.

**Parents are resolved once, at create time, and recorded in the saved state.** `types.Meta` gains `Parents []string` (`json:"parents,omitempty"`). `buildCreateDAG` (`internal/parser/dag.go:44`) writes it from the dependency set it already resolves for each resource (`getResourceDependencies`, `internal/parser/util.go:505`). That set covers explicit `depends_on`, references, module-wide references expanded to the module's resources, and the implicit "resource in a module depends on its module" edge. The IDs are sorted so the state file is stable, and unresolved (nil) entries are dropped. The destroy graph (`DoYouLikeDags(rp, true)`, name and comment kept) is then built from `Parents` alone, so destroy needs no configuration, and its order can't drift from create's. The old `buildDestroyDAG`, which matched unresolved `depends_on` strings exactly and so found almost no edges, is replaced. Saved state written before this change has no `parents`. Its resources are treated as having no parents, which the spec accepts.

**Guards at the edges.** `FileStateStore.Load` returns an error that names every saved type the registry doesn't know, instead of silently skipping it (user decision). Otherwise the first per-resource save would rewrite the file without those resources and orphan them. This affects Apply as well as Destroy, and both stop before touching anything. `Parser.Apply` rejects a configuration that declares no blocks, after parse and validate and before any destroy, create or save. `Validate` is unchanged. Destroy logs nothing above debug except errors. Events flow through the existing `OnParserEvent` → `xcl.EventHandler` path, and the examples' `eventlog` handler already logs start/success at debug and errors at error. Both examples gain a file state store in a temporary directory, and their `run` applies and then destroys the shared configuration.

**Why this shape.** Reusing the create graph walked in reverse follows the spec's Technical Approach. It also reuses a walker whose failure semantics already match the "never destroy a parent after a child fails" constraint, with no new graph code. A single destroyer shared by Destroy and Apply means removed resources and a full teardown behave the same way: same order, same events, same per-resource saves, same `destroy_failed` retry. Recording resolved parents at create time is the only way to order a destroy from the saved state alone. Re-resolving the saved `depends_on` strings would need the module structure from the configuration and would duplicate `getResourceDependencies`. Saving after every resource, rather than rebuilding the state once at the end the way `applyProgress.buildState` does, is what makes an interrupted destroy resumable. Rejected options and their evidence are in `research.md#alternatives-considered-and-rejected`.

**Conventions applied.** Tests for every acceptance criterion start from state written by a real apply, per *test state from a real apply*. The one exception is the Load test for unknown types, which is about the state format itself. Tests use testify `require`, never tables, and keep positive and negative cases in separate functions. The provider resolver and type registry stay injected through `ParserOptions`, per *dependency injection*, so destroy ordering can be tested with the recording `TestPlugin`. `internal/dag` is an in-repo fork and needs no changes. No third-party dependency is added, per *prefer the standard library*.

## Component Breakdown

- **Destroyer (new, parser).** Owns destroying a given set of resources held in a working copy of the saved state. It builds the destroy graph, walks it children-first, and after each resource updates the working state and saves it through the state store: a destroyed resource is removed, a failed one is kept and marked `destroy_failed`. It collects every failure into one error that names the failed resources. It is used by Parser Destroy and by Parser Apply's removal step, and it drives the per-resource destroy callback. New because nothing today owns "destroy a set and keep the saved state in step". The existing callback handles one resource and has no notion of state.

- **Per-resource destroy callback (changed, parser).** Still owns one resource's destroy: calling the provider's Destroy with the resource's saved copy, firing `destroy` start/success/error events, and treating builtin and registered types as provider-less (success event only). It now also treats disabled resources as provider-less. It reports its outcome to the destroyer instead of only setting a status, and its error follows the same `destroy failed for <id>: …` wording as the other provider calls.

- **Dependency graph builder (changed, parser).** The create graph builder now also records each resource's resolved parents on the resource. The destroy graph builder (the `destroy` branch of `DoYouLikeDags`) is rebuilt: it connects parent → child from the recorded parents, restricted to the set being destroyed, and hangs resources with no parent in the set off a root. It is the same shape as the create graph, and the destroyer walks it in reverse.

- **Parser Apply (changed).** Rejects a configuration with no blocks. Before the create walk, it finds the resources that are in the previous state but not in the configuration and hands them to the destroyer. If a removal fails, it returns the working state with the error and never starts the create walk. Otherwise it runs the create walk against the previous state minus the removed resources. Its behaviour is unchanged when nothing was removed.

- **Parser Destroy (new method on the existing parser).** Runs the destroyer over every resource in the given saved state and returns what is left, together with any error. It needs no configuration paths.

- **Config Destroy (changed, public API).** Replaces the stub. It takes the saved state from the store when one is configured and exists, otherwise the in-memory state. It succeeds immediately without saving when there is nothing to destroy. It calls Parser Destroy, adopts the remaining state, and returns the error.

- **Resource metadata (changed, `types`).** `Meta` gains the recorded parents, which are saved with every resource so destroy can be ordered from the saved state alone.

- **File state store (changed).** Load fails with an error that names every saved type the registry can't create, instead of silently dropping those entries. This keeps both Destroy and Apply from rewriting the state without them.

- **Examples (changed).** The configuration-only example and the plugin example each get a file state store in a temporary directory. Their run applies the shared configuration, then destroys it. Their tests check that the saved state ends up empty and that destroy events are logged at debug. The event log handler and both example providers are reused unchanged.

- **Documentation (changed).** The state, lifecycle, overview and plugin-developer guides, the README and the CHANGELOG stop describing destroy as a stub. They describe Destroy, removal during Apply, the recorded parents, the empty-configuration error and the Load error for unknown types.

## Data Structures & Interfaces

**`types.Meta.Parents` (new field, saved state format).** Holds the IDs of the resources this resource depends on, resolved when the create graph is built. It covers explicit `depends_on`, references, module-wide references expanded to that module's resources, and the module the resource sits in. The IDs are sorted and omitted when empty. It is the only input the destroy graph needs, which is what lets destroy run without configuration. It is internal and can't be set from configuration, like `links` and `status`.

```go
type Meta struct {
    // ... existing fields ...
    Parents []string `json:"parents,omitempty"`
}
```

Saved entry (excerpt):

```json
{ "meta": { "id": "resource.container.third", "status": "created",
            "parents": ["resource.container.second"] } }
```

**`Config.Destroy` (public API, contract changes, signature unchanged).** Takes no configuration. It destroys everything in the saved state, or in the in-memory state when there's no store, and returns nil when nothing is left. Otherwise it returns an error that names every resource that failed to be destroyed. When nothing has been saved, it returns nil and writes nothing.

```go
func (c *Config) Destroy() error
```

**`Parser.Destroy` (new method).** Destroys every resource in `saved`, saving the state through the configured `StateStore` after each resource. It returns what is left (only `destroy_failed` resources and those never reached because a child failed) together with the error. It never returns a nil state.

```go
func (p *Parser) Destroy(saved *state.State) (*state.State, error)
```

**`destroyer` (new, internal to the parser).** It is the contract shared by Destroy and Apply's removal step. `working` is the state being changed. It starts as the saved state and ends as what survives. `destroy` walks the given subset of `working`, children first. `mu` guards `working` and the store's `Save`, because the walk visits unrelated resources concurrently.

```go
type destroyer struct {
    working  *state.State
    store    state.StateStore // may be nil: nothing is persisted
    resolver ProviderResolver
    types    TypeRegistry
    options  *ParserOptions
    mu       sync.Mutex
}

func (d *destroyer) destroy(targets []any) error
```

**Per-resource destroy callback (changed signature).** It receives the destroyer so that it can report each outcome: remove and save on success, mark `destroy_failed` and save on failure.

```go
func destroyWalkCallback(d *destroyer) func(v dag.Vertex) dag.Diagnostics
```

**`DoYouLikeDags(rp, true)` (contract changes, signature kept).** It builds the destroy graph from `Meta.Parents` of the resources `rp` returns: edges run parent → child, and resources with no parent in the set hang off a root. The destroyer walks it with `Reverse = true`. Parents that aren't in the set are ignored.

**`state.UnknownTypesError` (new error).** `FileStateStore.Load` returns it when saved entries have types the registry can't create. It lists every such type, so the user knows which plugin to load.

```go
type UnknownTypesError struct {
    Types []string // sorted, unique
}
// Error(): `saved state holds resources of unknown types: app, postgres (register their types or plugins before loading state)`
```

**`parser.ErrEmptyConfiguration` (new sentinel error).** `Parser.Apply` returns it, and `Config.Apply` passes it through unchanged, when the configuration declares no blocks. Callers can check it with `errors.Is`.

```go
var ErrEmptyConfiguration = errors.New("the configuration declares no blocks, use Destroy to remove everything")
```

**Events (unchanged shape).** `xcl.Event` / `parser.ParserEvent` already carry `Operation: "destroy"`. Provider-handled resources fire `start` followed by `success` or `error`, with `Data` holding the saved copy sent to the provider. Builtin, registered and disabled resources fire `success` only, with nil `Data`.

**Statuses (unchanged set).** `destroy_failed` marks a resource whose destroy failed. `destroyed` is still a valid constant, but it is never saved, because destroyed resources leave the state.

## Implementation Detail

**Destroy mirrors create.** A reader who knows the apply path will recognise the destroy path. Apply builds a graph, attaches a callback and hands the per-resource provider work to the lifecycle, which records outcomes under a mutex. Destroy builds a graph of the same shape, attaches a callback and hands outcomes to the destroyer, which records them under a mutex. The only differences are the walker's `Reverse` flag and where the outcome goes. Apply keeps outcomes in memory and builds the state at the end. Destroy writes each outcome to the working state and saves it straight away. The walker's own rule, that a vertex runs only after everything it waits on has succeeded, is the single mechanism behind every ordering guarantee in the spec. No destroy-specific scheduling code is written.

**One new pattern: persisting state inside the walk.** Until now the parser never touched the store during a walk. `Config.Apply` saved once at the end. The destroyer introduces the first save during a walk. The pattern is "mutate the working state, then save it, both under one lock", so concurrent callbacks never interleave a half-updated state with a save. A save error counts as a failure of that resource's step. The walker then stops going up that branch, because a resource must not be destroyed when the fact that its child is gone can't be recorded. Only the destroyer saves. The callback never sees the store.

**Parents are written where dependencies are resolved.** Recording `Meta.Parents` goes into the create graph builder, at the point where it has already turned references into resource pointers. It adds no second resolution pass and no new parse-time work. The recorded list is exactly the set of create edges into the resource, apart from the root. So "destroy is create backwards" holds by construction, not by keeping two pieces of code in agreement. The destroy branch of `DoYouLikeDags` then becomes a small builder that reads only `Parents`. The function name and its comment are kept, at the author's request in the code.

**Apply gains a removal phase with a clear boundary.** `Parser.Apply` reads as parse & validate → reject empty → destroy removed → create walk. The removal phase is skipped entirely when nothing was removed, so the code path for configurations that remove nothing is the one that exists today. That is what makes the "no regression" metric cheap to hold. A failed removal returns early with the working state, following the existing "return partial state with the error" contract, so `Config.Apply` needs no new branch to save it.

**The per-resource callback is reshaped, not replaced.** The existing destroy callback keeps its event bracketing and its provider-less skip, but its outcome handling moves to the destroyer. It no longer only sets a status on a resource that nobody reads. The statuses it sets become `destroy_failed` on failure, and on success the resource is removed. Its error wording matches `callProvider` (`destroy failed for <id>: …`), so errors from destroy, removal and rebuild read the same. It is reshaped rather than reused as-is because it predates the lifecycle helper and currently has no way to report an outcome.

**Guards stay at the boundaries.** The "unknown type" check lives in the file store's Load, where the information is lost today. The "empty configuration" check lives in `Parser.Apply`, so `Validate` keeps accepting an empty directory. Neither adds logic to the walk.

**Examples show the full cycle.** Both examples keep their `run` shape. They gain a state file path and a destroy after the printed output, so the example code a developer copies shows `NewFileStateStore` + `WithStateStore` + `Apply` + `Destroy` together, with events logged through the existing handler.

**Removed code.** The old exact-match destroy graph builder and the commented-out legacy `Parser.Destroy` tests go away. The docs lose every "destroy is a stub / removed blocks are not destroyed" note.

## Dependencies

- **`internal/dag` (in-repo fork of the DAG walker)** — provides `AcyclicGraph`, `Walker.Reverse`, and the skip of vertices upstream of a failed one. It is used as is, with no changes.
- **`internal/parser` lifecycle pieces** (`handledWithoutProvider`, `TypeRegistry`, `ProviderResolver`, `fireParserEvent`, `getResourceDependencies`) — they classify provider-less types, look up providers, fire events and resolve dependencies. They are reused unchanged; only the destroy callback and the graph builder change.
- **`state` package** (`State`, `StateStore`, `FileStateStore`) — holds and persists the working state. `FileStateStore.Load` changes (unknown types error), and a new error type is added. The `StateStore` interface is unchanged.
- **`plugins/registry.PluginRegistry`** — creates typed resources for Load and reports registered types. It is unchanged.
- **`plugins.ProviderAdapter.Destroy(ctx, data, force)`** — the provider contract destroy calls. It is unchanged, and `force` is always false.
- **`types` package** — `Meta` gains `Parents`. The status constants are unchanged.
- **`parser.TestPlugin` and the lifecycle/registered test harnesses** — the recording provider with ordered calls and per-ID destroy errors, and real registry + file-store harnesses. Tests reuse them. Small helpers may be added, such as a store that records each save.
- **`example/eventlog`, `example/resources`, and both example providers** — reused unchanged. Their `Destroy` implementations already exist and already log at debug.
- **Go standard library only** (`sync`, `slices`/`sort`, `errors`, `encoding/json`) — no new third-party modules.
- **Prior plans (already landed, must remain in place):** `20260918165700-provider-lifecycle-read` (rebuild of failed/`destroy_failed` resources, `callProvider`, partial-state-on-error contract) and `20260919120639-config-only-types-and-examples` (registered types without a provider, the two examples). Nothing needs to land before this plan starts.
- **Spec:** `20260919152915-destroy-cycle` — the source of every requirement, acceptance criterion and success metric.

## Testing Approach

**Test types.** Most of the coverage is **scenario tests** that drive the real path end to end. They use a real plugin registry with the recording `TestPlugin`, a real file state store in a temp directory, and real `.xcl` fixtures. They sit at two levels: parser-level tests next to the existing lifecycle tests, and `Config`-level tests next to the existing config tests for the public `Destroy`/`Apply` contract and events. A few **unit tests** pin down contracts that are awkward to reach through a scenario: the recorded parents for module membership and module-wide references, the destroy graph shape built from parents, the `FileStateStore.Load` unknown-types error, and the per-resource callback skipping disabled resources. The **examples' own tests** become end-to-end checks of the apply → destroy cycle, and they run in the normal test suite (the plugin example's test already builds its external binary itself).

**Conventions.** Every earlier state a scenario needs comes from a real apply: a first apply, an apply with an edited configuration (for removals), or a destroy with an injected failure (for `destroy_failed`). No state file is hand-written, except in the Load test for unknown types, which is about the state format. Each acceptance criterion gets its own named test. Positive and negative cases go in separate functions, assertions use testify `require`, there are no tables, and destroy order is read from the recording provider's ordered call list.

**Load-bearing assertions (what the tests guarantee).**
- A full destroy sends exactly one Destroy to every provider-handled resource and leaves the saved state empty. This still holds after the configuration files have been deleted.
- Order: on the chain C → B → A, destroy calls arrive C, B, A, both with the configuration present and with it deleted (order comes from saved parents).
- Failure isolation: when B fails, C and the unrelated D are destroyed, A gets no call, the error names B, the saved state holds exactly B (`destroy_failed`) and A, and a second destroy (with B now succeeding) destroys B then A and empties the state.
- Resumability: each per-resource save leaves the store holding exactly the resources not yet destroyed. Restarting a destroy from a mid-way snapshot, taken from a real run, calls providers only for the rest.
- Provider-less blocks (variables, outputs, modules, a disabled block, registered types) never reach a provider, fire a destroy success event only, and leave the state.
- Removal during Apply: a removed X is destroyed and leaves the state while Y is untouched. X's destroy happens before a new Z's create. Removed Q (child) is destroyed before removed P (parent). A failing removal stops the apply before Z is created, keeps X as `destroy_failed`, and the next apply destroys X and then creates Z.
- Events: provider-handled resources report destroy start → success, or start → error. Variables report success only.
- Guards: an empty configuration fails Apply with no provider call and an unchanged saved state. Destroy with nothing saved succeeds, calls no provider and creates no state file. Load names unknown types and fails instead of dropping them.
- Examples: each applies the shared configuration, destroys it and ends with an empty saved state. Destroy events are logged at debug, and nothing above debug is logged on success.

**Success metrics → verification.**
- *Nothing is orphaned* — **Behavioural test.** Two tests, one for destroy and one for removing a block, each with an injected destroy failure. After the run, every resource the recording provider created has either received a successful Destroy, or is still in the saved state (`destroy_failed`, or not yet reached because a child failed). The check is: set of created IDs − set of successfully destroyed IDs ⊆ IDs in the saved state.
- *A parent is never destroyed after a child fails* — **Behavioural test.** Two tests, one for destroy and one for removing a block. For every resource whose destroy failed, none of its recorded parents (read from the saved state) appears in the provider's destroy calls.
- *No regression* — **Behavioural test.** The full existing test suite and vet checks pass unchanged. A dedicated test re-applies an unchanged configuration and asserts that no destroy call is made and the saved state matches the first apply, apart from the new `parents` field.

**Deliberate gaps.** Nothing is added for concurrent destroys against the same state (state locking is a non-goal). Nothing is added for old state without `parents` (the spec gives no ordering guarantee there). No external-plugin destroy test beyond the plugin example, which already exercises destroy over gRPC through its external `app` provider. Force destroy is not exposed (`force` is always false).

## Milestones & Phases

### Milestone 1: Destroy tears down everything XCL created

**What changes**: Calling `Destroy` on a config really removes what was applied. It no longer reports success while doing nothing. Every resource in the saved state is destroyed through its provider, dependents before the things they depend on, and it works even after the configuration files are gone, because each resource now records its parents in the saved state. A resource that fails to be destroyed stays in the saved state as `destroy_failed`, together with everything it depends on, while unrelated resources are still destroyed. The error names what failed, and running `Destroy` again retries it. The saved state is updated after every resource, so a destroy that is interrupted picks up where it stopped. Variables, outputs, modules, disabled blocks and configuration-only types are cleared from the state without ever reaching a provider. Every step is reported through the event handler, like creates and updates. Destroying with nothing saved succeeds and does nothing. Loading a saved state that contains types that aren't registered now fails with an error naming those types, instead of silently dropping them.

**Validation point**: The Destroy scenario tests pass: full teardown, no configuration needed, C → B → A order, failure isolation and retry, resume from a mid-way snapshot, provider-less blocks, events, nothing saved. So do the "nothing orphaned" and "no parent after a failed child" checks for destroy, and the Load unknown-types test. The existing test suite and vet checks still pass.

#### - [x] Phase 1.1: Record parents in the saved state and refuse unknown types on load
**Repo:** xclconfig

Every resource saved by an apply now records the IDs of the resources it depends on, resolved exactly as the create order resolves them: explicit `depends_on`, references, module-wide references, and the module the resource sits in. This is what lets a destroy be ordered from the saved state alone. Loading a saved state also stops silently dropping entries whose type isn't registered, and fails with an error that names those types, so nothing can be erased from the state by accident.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-record-parents-in-the-saved-state-and-refuse-unknown-types-on-load)

**Acceptance criteria**:
- [x] After an apply, each saved resource lists the resources it depends on as its parents, and a resource with no dependencies lists none.
- [x] A resource inside a module lists its module as a parent, and a reference to a whole module lists every resource in that module.
- [x] Re-applying an unchanged configuration makes no provider destroy call and saves the same resources and parents as before.
- [x] Loading a saved state that contains a type nobody registered fails with an error naming that type, instead of returning a state without it.

#### - [x] Phase 1.2: Destroy resources children-first from the saved state
**Repo:** xclconfig

A new parser destroyer walks the saved resources in the reverse of their create order. The existing per-resource destroy step calls the provider and reports events, and after each resource the saved state is updated and written. A failure keeps that resource as `destroy_failed` and leaves everything it depends on untouched, while unrelated resources are still destroyed. Builtin, disabled and configuration-only blocks are cleared without a provider call. This phase delivers the parser's Destroy and its scenario tests.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-destroy-resources-children-first-from-the-saved-state)

**Acceptance criteria**:
- [x] Destroying an applied chain where C depends on B and B depends on A calls the provider in the order C, B, A, and leaves nothing in the saved state.
- [x] When B fails to be destroyed, C and an unrelated D are destroyed, A gets no destroy call, the error names B, and the saved state holds B (marked failed to destroy) and A only.
- [x] A second destroy after B's failure, with B now succeeding, destroys B and then A and empties the saved state.
- [x] The saved state after each destroyed resource holds exactly the resources not yet destroyed, and a destroy restarted from any of those points calls providers only for what remains.
- [x] Variables, outputs, modules, disabled blocks and configuration-only types are removed from the saved state without any provider being asked to destroy them.
- [x] No resource ever receives a destroy call after destroying one of its children has failed, and every resource a provider created is either destroyed or still in the saved state.

#### - [x] Phase 1.3: Make Config.Destroy real
**Repo:** xclconfig

The public `Destroy` stops being a stub. It picks up the saved state (or the in-memory state when there's no store), runs the parser's destroy, keeps what is left and returns any failure. With nothing saved it succeeds without writing anything, and it needs no configuration paths. Destroy events reach the application's event handler exactly like create and update events.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-make-configdestroy-real)

**Acceptance criteria**:
- [x] Destroying an applied configuration calls each provider-handled resource's destroy exactly once and leaves the saved state empty.
- [x] Destroy still succeeds and destroys everything in the right order after the configuration files have been deleted.
- [x] Destroying when nothing was ever applied succeeds, calls no provider and creates no saved state.
- [x] With an event handler registered, a provider-handled resource reports destroy start then success, a failing one reports start then error, and a variable reports only success.
- [x] Destroy logs nothing above debug unless something fails.

### Milestone 2: Removing a block from the configuration removes the real thing

**What changes**: When a block that was applied before is taken out of the configuration, the next `Apply` destroys the real resource through its provider instead of silently forgetting it. Removed resources are destroyed first, children before parents, before anything is created or updated, so names and ports they held are free for their replacements. If destroying a removed resource fails, the apply stops before creating or changing anything. The resource is kept in the saved state as `destroy_failed`, and the next apply retries it first. Applying a configuration that declares no blocks is now an error that touches nothing, because `Destroy` is the way to remove everything. Applies that remove nothing behave exactly as before.

**Validation point**: The removal scenario tests pass: X destroyed while Y is unchanged, X destroyed before Z is created, Q before P, a failed removal stops the apply and is retried. So do the empty-configuration test, the "nothing orphaned" and "no parent after a failed child" checks for removal, and the unchanged re-apply regression test. The full test suite and vet checks pass.

#### - [x] Phase 2.1: Destroy removed resources first during Apply, and reject empty configurations
**Repo:** xclconfig

Apply works out which previously applied resources are no longer in the configuration. Before creating or updating anything, it destroys them with the same destroyer, children first. If one of those destroys fails, the apply stops before creating or changing anything, keeps the resource as `destroy_failed`, and the next apply retries it first. Applying a configuration with no blocks is rejected before anything is touched. Applies that remove nothing behave exactly as they do today.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-destroy-removed-resources-first-during-apply-and-reject-empty-configurations)

**Acceptance criteria**:
- [x] After X and Y were applied, removing X and applying again destroys X through its provider, removes it from the saved state and leaves Y unchanged.
- [x] An apply that removes X and adds Z destroys X before creating Z.
- [x] Removing Q and P together, where Q depends on P, destroys Q before P.
- [x] When destroying a removed X fails in an apply that also adds Z, the apply fails naming X, Z is never created, and X stays saved as failed to destroy. The next apply, with X's destroy now succeeding, destroys X and then creates Z.
- [x] Applying a directory with no blocks fails, calls no provider and leaves the saved state as it was.
- [x] During removals, no resource receives a destroy call after one of its children failed, and nothing a provider created goes missing from the saved state.

### Milestone 3: The examples and docs show the full apply-and-destroy cycle

**What changes**: The configuration-only example and the plugin example now keep their state in a file and, after applying the shared configuration and printing the result, destroy it and end with an empty saved state. This gives developers a copyable example of the whole lifecycle, with destroy events logged at debug like every other event. The documentation and changelog describe how destroy works, removal during apply, the recorded parents, the empty-configuration error and the unknown-types load error, and no longer say destroy is a stub.

**Validation point**: Both examples' tests pass as part of the normal test suite. They show apply then destroy, an empty saved state, destroy events at debug and nothing above debug on success. No doc still describes destroy as unimplemented or says removed blocks are not destroyed.

#### - [x] Phase 3.1: Examples apply and then destroy
**Repo:** xclconfig

Both examples keep their state in a file. After applying the shared configuration and printing what they built, they destroy it and end with an empty saved state. Their tests run this full cycle as part of the normal test suite and check that destroy events are logged at debug.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-examples-apply-and-then-destroy)

**Acceptance criteria**:
- [x] Running the configuration-only example applies the shared configuration, destroys it and leaves an empty saved state.
- [x] Running the plugin example does the same, and every postgres and app resource is destroyed through its provider.
- [x] Both examples log destroy events at debug and log nothing above debug when everything succeeds.

#### - [x] Phase 3.2: Documentation describes destroy
**Repo:** xclconfig

The state, lifecycle, overview and plugin-developer guides and the README describe how destroy works. They cover the reverse order, `destroy_failed` retry, per-resource saves, removal during apply, recorded parents, the empty-configuration error and the unknown-types load error. Every note that says destroy is a stub or that removed blocks are forgotten is removed.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-documentation-describes-destroy)

**Acceptance criteria**:
- [x] No document says that destroy is unimplemented or that removed blocks are not destroyed.
- [x] A plugin author can read when their provider's Destroy is called: on destroy, on removal, and on rebuild.
- [x] The README shows an application calling Apply and then Destroy with a file state store.

## Open Questions

- **Does any existing test or code path rely on `FileStateStore.Load` silently dropping unknown types?** This depends on how the full suite behaves once Load starts failing on unknown types. A test may save state with a plugin registered and then load it through a registry without one, which a static read can't reliably find. **What to do:** if it is a test, register the missing type or plugin in that test's registry and carry on. If a non-test code path depends on the silent skip (for example, loading state before plugins are registered), STOP and ask the user before changing that path.

No other open questions. Every other decision is recorded in the assumption log.

## Out of Scope

- **Targeted destroy.** Destroying only some resources, such as one resource and its children, is not done. Destroy always covers everything in the saved state. (Spec non-goal.)
- **Create-before-destroy.** Replacements are never created before the old resource is removed. Removed resources are always destroyed first. (Spec non-goal; the trade-off is accepted in the spec's Technical Approach.)
- **Destroy limited to a given configuration.** `Destroy` takes no configuration paths. (Spec non-goal.)
- **Replacing resources whose settings changed.** Changed resources keep today's read/changed/update behaviour. Nothing is destroyed and recreated because its settings changed. (Spec non-goal.)
- **State locking.** Nothing stops two applies or destroys from running against the same saved state at once. (Spec non-goal.)
- **Ordering for state saved before this change.** Resources saved without `parents` are destroyed as if they had no parents. No migration or re-resolution is done. (Agreed during the spec: no guarantee.)
- **Destroying a resource when it becomes disabled.** A block that was created and is later disabled is still handled as today during Apply, and a disabled block is never passed to a provider on destroy. Tearing down resources when they are disabled would be its own spec.
- **Failing Load on malformed state entries.** Only unknown types become an error. Malformed entries, and entries with no meta, keep today's skip behaviour.
- **Atomic state writes.** `FileStateStore.Save` still removes the file and then writes it. The per-resource saves make that window more frequent, but making writes atomic (temp file + rename) is left for a follow-up.
- **Exposing `force` destroy.** Providers always receive `force = false`, and no public option sets it.

## Changelog

### 2026-09-19 — Phase 1.1: Record parents in the saved state and refuse unknown types on load

**What was done**: `types.Meta` gained `Parents []string` (`json:"parents,omitempty"`), and `buildCreateDAG` records each resource's resolved, sorted, non-nil parent IDs while it builds the create graph. `FileStateStore.Load` now returns the new `state.UnknownTypesError` (sorted, unique types) instead of silently dropping entries whose type the registry cannot create.

**Deviations**:
- `buildCreateDAG` uses a `connected` flag instead of `len(deps) == 0` for the root edge. Otherwise a resource whose only dependencies were unresolved (nil) would be connected to nothing.
- The plan expected `network.independent` to have no parents. In fact it lists `variable.independent_subnet`, because variable references count as dependencies. The test asserts the real value.
- A module-wide reference needed a new fixture, `internal/test_fixtures/config/lifecycle/module_reference/`, because no existing fixture usable by the harnesses has `depends_on = ["module.x"]`.
- The golden schema `internal/schema/test_fixtures/embedded.go` was updated with the new `Parents` field, which `TestSerializeEmbedded` required.

**Files changed**:
- `types/resource.go`
- `internal/parser/dag.go`
- `state/errors.go`
- `state/file_state_store.go`
- `state/file_state_store_test.go`
- `internal/parser/parents_test.go`
- `internal/test_fixtures/config/lifecycle/module_reference/main.xcl`
- `internal/test_fixtures/config/lifecycle/module_reference/module/networks.xcl`
- `internal/schema/test_fixtures/embedded.go`

**Discoveries**:
- Any new field on `types.Meta` must also be added to the golden schema `EmbeddedJson` in `internal/schema/test_fixtures/embedded.go`.
- A module-wide reference resolves to the module's resources, not to the module itself. Resources inside a module get the module as a parent.
- No non-test code relied on Load's silent skip of unknown types. The only non-test caller is `parseAndValidate` (`internal/parser/parser.go:267`).

### 2026-09-19 — Phase 1.2: Destroy resources children-first from the saved state

**What was done**: The new `destroyer` in `internal/parser/destroy.go` builds the destroy graph from each resource's recorded parents and walks it in reverse, so children are destroyed first. After every resource it saves the working state: a destroyed resource is removed, and a failed one is kept as `destroy_failed`. `destroyWalkCallback` now takes the destroyer and treats disabled blocks as provider-less. Its error wording is now `destroy failed for <id>: …`. `Parser.Destroy(saved)` runs the destroyer over the whole saved state. `buildDestroyDAG` is rebuilt on `Meta.Parents`.

**Deviations**:
- The tests add a fixture, `internal/test_fixtures/config/lifecycle/disabled/disabled.xcl`, so a disabled block of a provider type can be checked too.
- The resume test restarts from every real per-resource snapshot in one harness, rather than from one snapshot in a second harness.
- The existing `TestDestroyWalkSkipsProviderForRegisteredType` now asserts that the resource was removed from the working state. The status is no longer `destroyed`.

**Files changed**:
- `internal/parser/destroy.go`
- `internal/parser/dag.go`
- `internal/parser/callbacks.go`
- `internal/parser/parser.go`
- `internal/parser/parse_test.go`
- `internal/parser/registered_types_test.go`
- `internal/parser/destroy_test.go`
- `internal/test_fixtures/config/lifecycle/disabled/disabled.xcl`

**Discoveries**:
- In `internal/dag`, `Walker.Reverse` makes an edge's source wait on its target. Parent→child edges walked with `Reverse` therefore visit children first. The root→target edges make the root run last, and the upstream-failure cascade works in reverse as well.
- `State.GetResources` returns the live backing slice, and `RemoveResource` mutates it. Copy the slice before iterating while removing.

### 2026-09-19 — Phase 1.3: Make Config.Destroy real

**What was done**: `Config.Destroy` now does the destroy. It loads the saved state, or uses the in-memory state when no store is configured, and calls `Parser.Destroy`. It keeps the state that remains and returns the error. It returns nil without writing anything when no state was saved or the saved state is empty. A load failure, including `state.UnknownTypesError`, is wrapped as `failed to load state: …`. The event-handler docs in `config.go`, `events.go` and `options.go` now cover Destroy as well as Apply.

**Deviations**:
- The docs for `WithEventHandler` in `options.go` and for `Event` in `events.go` were updated too, not only the `EventHandler` comment.
- Extra tests were added: a Config with no store, a real store still holding the initial empty `[]`, and a separate retry test.

**Files changed**:
- `config.go`
- `events.go`
- `options.go`
- `config_destroy_test.go`

**Discoveries**:
- Nothing in the xcl or parser destroy path logs anything. The criterion "nothing above debug" can therefore only be observed at Config level as an absence of log lines. The meaningful check is at example level in Phase 3.1, where providers and the eventlog handler log.
- `FileStateStore` writes `[]` when it is constructed, so `Exists()` is always true with a real store. The "nothing saved" path is reached through the empty-state check.

### 2026-09-19 — Phase 2.1: Destroy removed resources first during Apply, and reject empty configurations

**What was done**: `Parser.Apply` now rejects a configuration with no blocks with `ErrEmptyConfiguration` before touching anything. It then finds the resources that are in the previous state but no longer in the configuration, and destroys them children-first with the shared `destroyer`, saving after each one. If a removal fails, Apply returns the previous state minus what was destroyed, with the failures kept as `destroy_failed`, and never starts the create walk. Otherwise the create walk runs against the previous state minus the removed resources. The Apply docs in `parser.go` and `config.go` describe the new steps.

**Deviations**:
- `ErrEmptyConfiguration` lives in a new file, `internal/parser/errors.go`, which imports the standard `errors` package, so it doesn't clash with the parser's `xcl/errors` import.
- The removed-resource "unchanged" assertion leaves out `meta.file`, `meta.line` and `meta.column`, which legitimately change when a fixture's source file changes.
- A separate test, `TestApplyAfterRemovingRegisteredBlockNeverCallsProvider`, was added alongside the extended registered-removal test.

**Files changed**:
- `internal/parser/parser.go`
- `internal/parser/errors.go`
- `config.go`
- `internal/parser/removal_test.go`
- `internal/parser/registered_types_test.go`
- `config_destroy_test.go`
- `internal/test_fixtures/config/lifecycle/removal/before/main.xcl`
- `internal/test_fixtures/config/lifecycle/removal/without_x/main.xcl`
- `internal/test_fixtures/config/lifecycle/removal/without_x_with_z/main.xcl`
- `internal/test_fixtures/config/lifecycle/removal/without_pq/main.xcl`
- `internal/test_fixtures/config/empty/empty.xcl`

**Discoveries**:
- A directory whose only `.xcl` file holds a comment parses and validates cleanly into an empty state. The empty-configuration check therefore belongs after `parseAndValidate`, as planned.
- The parser package's `errors` identifier is `github.com/jumppad-labs/xcl/errors`. Tests that need `errors.Is` must alias the standard library package.

### 2026-09-19 — Phase 3.1: Examples apply and then destroy

**What was done**: Both examples keep their state in a file. They use `state.NewFileStateStore` with `xcl.WithStateStore`, and `run` gained a `statePath` parameter; `main` uses a temp dir that is removed at exit. After applying and printing, each example calls `c.Destroy()` and prints `## Destroyed` with the number of resources left, then returns the resources it applied. The tests check that the saved state ends empty, that providers log `destroy` for each postgres and app resource, and the destroy event phases: start and success for provider types, success only for provider-less blocks. The existing "nothing above debug" tests still pass with destroy included.

**Deviations**:
- The tests also check the printed `## Destroyed` / `0 resources remaining` output.
- `eventPhases` in the plugin test now takes an operation argument.
- New tests follow each file's `TestConfigOnlyExample…` / `TestPluginExample…` naming.

**Files changed**:
- `example/configonly/main.go`
- `example/configonly/main_test.go`
- `example/plugin/main.go`
- `example/plugin/main_test.go`

**Discoveries**:
- `force=false` arrives as a real bool at the external plugin after crossing gRPC.
- In the plugin example's destroy order, `app.web` is destroyed before `postgres.main`, which it references, and unrelated postgres resources run in parallel.

### 2026-09-19 — Phase 3.2: Documentation describes destroy

**What was done**: The lifecycle, state, overview, plugin-developer and plugins guides, `docs/README.md` and the README now describe destroy. They cover `Config.Destroy`, the children-first walk over `meta.parents`, the per-resource saves and resume, retrying `destroy_failed` resources, removal during Apply, the empty-configuration error, and the unknown-types Load error. Every "destroy is a stub" and "removed blocks are not destroyed" note is gone. The README has a new "State and Destroy" section showing `NewFileStateStore` + `WithStateStore` + `Apply` + `Destroy`.

**Deviations**:
- `ErrEmptyConfiguration` is re-exported as `xcl.ErrEmptyConfiguration` in `config.go`, because `internal/parser` can't be imported by applications. The README, docs and the Config-level test use the public name.
- `docs/parser-lifecycle.md`'s "Who can subscribe" wrongly said Config had no way to set an event handler. It now describes `xcl.WithEventHandler`.
- The docs' stale file:line links were refreshed.
- `CHANGELOG.md` is left to the workflow's feature-changelog step.

**Files changed**:
- `docs/parser-lifecycle.md`
- `docs/state.md`
- `docs/overview.md`
- `docs/plugin-developer-guide.md`
- `docs/plugins.md`
- `docs/README.md`
- `README.md`
- `config.go`
- `config_destroy_test.go`

**Discoveries**:
- Sentinel errors defined in `internal/parser` must be re-exported from the root `xcl` package, or applications can't match them with `errors.Is`.
