---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Plan: 20260918165700-provider-lifecycle-read

<!-- Metadata -->
<!-- Created: 2026-09-19T09:41:11+01:00 -->
<!-- Commit: 2daa13fd86854b67051fa5878280bfeb828a2e45 -->
<!-- Branch: v2 -->
<!-- Repository: git@github.com:jumppad-labs/hclconfig.git -->

## Overview

xcl's apply is being reworked so that a second apply uses the state saved by the last one. Each resource is read through its provider, and only resources whose configuration changed, or whose real counterpart drifted, are updated. Failed resources are rebuilt on the next run, and a failed apply keeps the progress it made instead of forgetting it. Plugin authors get one clear job, implementing `Read(ctx, old, new)` to report the real resource. They also get a documented contract: `ErrNotFound`, provider-owned `computed` fields, default change detection they can embed, and a finished developer guide. Today every apply recreates everything, and plugin authors have no written contract.

## Conventions

- **Testing & Mocking: testify `require`, Mockery mocks, no table-driven tests, positive and negative cases in separate functions, verbose readable tests** — every acceptance criterion becomes one or more parser tests. The mocks for `ProviderAdapter` and `StateStore` must be regenerated with mockery after the interface changes.
- **Patterns & Architecture: dependency injection for testability, composition over inheritance** — `resourceLifecycle` takes its `ProviderResolver`, previous state and options as injected dependencies. `DefaultChanged[T]` is composed into providers by embedding.
- **Go Code Style: small focused interfaces, handle every error, descriptive names, `any` over `interface{}`, stdlib style** — this applies to the reshaped `ResourceProvider[T]`/`ProviderAdapter`, the sentinel `ErrNotFound` checked with `errors.Is`, and fixing the vet-flagged `fmt.Errorf(resp.Error)` calls in the touched gRPC client functions.
- **Development Standards: structured logging** — the warning for a changed configured value is logged with `Logger.Warn` and key/value pairs (resource ID, field).
- **Dependencies: prefer the standard library, pin versions** — `DefaultChanged`, the computed-field reflection and the reference detection use only `reflect`, `encoding/json` and the existing hcl/hclsyntax. No new modules.

## Architecture & Design Decisions

All work lands in the single registered repo, **xclconfig** (`/home/nicj/code/github.com/jumppad-labs/xclconfig`). The plan has three parts. The first is the lifecycle engine in `internal/parser`. The second is the provider contract in `plugins` (interface, adapter, gRPC wire). The third is the `computed` marker, which touches `types`, `internal/parser/validate.go` and the lifecycle engine. Decision-making moves out of the long `callProviderLifecycle` function in `internal/parser/callbacks.go` into a new `resourceLifecycle` component (`internal/parser/lifecycle.go`). It is built once per walk and holds the previous state, the `ProviderResolver`, the parser options and an `applyProgress` recorder. `walkCallback` still decodes the resource, then hands it to `resourceLifecycle.apply(r)`. That function picks exactly one path from the resource's previous-state entry:

- **create**: the resource is not in the previous state.
- **read → changed → update**: the resource was previously `created` or `updated`. If `Read` returns `plugins.ErrNotFound`, the resource is created instead.
- **rebuild**: the resource was previously `failed` or `destroy_failed`. It is destroyed using its old copy, then created.

The previous state that `parseAndValidate` already loads goes straight into the walk as a `*state.State`. Resources are found with `state.FindResource`, which replaces the unused `previousParsed *parsed`. Dependency injection through `ProviderResolver` stays the seam for tests (convention: dependency injection for testability).

A failed apply keeps its progress without any extension to the DAG library. The jumppad-labs/dag walker already skips the dependents of a failed vertex and still runs independent vertices in parallel. It exposes no per-vertex results, so `applyProgress` records each resource's outcome under a mutex: whether it succeeded, or failed in a provider call. When the walk fails, `Parser.Apply` returns a state built from that record *together with* the error. Resources that were reached keep their new values and status. The resource whose provider call failed is kept with status `failed` (or `destroy_failed`). Resources that weren't reached are replaced by their previous-state entry, and dropped if they have none. `Config.Apply` saves any non-nil state it gets back, even on error, and then returns the error. Parse and validation failures still return a nil state and save nothing, which keeps the existing "invalid config saves nothing" tests true. Only a failed *provider call* marks a resource `failed`. A decode failure before any provider call means the resource was never touched, so it counts as unreached.

The provider contract changes in breaking ways, which the spec allows. `ResourceProvider[T].Refresh(ctx, T)` becomes `Read(ctx, old, new T) (T, error)`. That change runs through `ProviderAdapter`, `PluginEntityProvider`/`PluginBase`, `PluginHost`, the direct and gRPC hosts, the gRPC server and `plugin.proto`. The proto change renames `rpc Refresh` to `rpc Read`, adds `old_entity_data`/`new_entity_data`, and adds `bool not_found` to `ReadResponse`. Errors cross gRPC as in-band strings, so the not-found signal needs its own field. The server sets it with `errors.Is(err, ErrNotFound)`, and the host turns it back into an error that wraps `plugins.ErrNotFound`. In-process adapters already pass the error through unchanged. `plugins.DefaultChanged[T]` is an embeddable struct whose `Changed` does a flat comparison. It marshals both copies to JSON maps, drops the `ResourceBase` keys (`meta`, `depends_on`, `disabled`) and uses `reflect.DeepEqual`. It uses the standard library only (convention: prefer the standard library). Because a remote `Changed` runs inside the plugin process, the promoted method works the same in-process and over gRPC. Statuses become typed constants in `types` (`created`, `updated`, `failed`, `destroyed`, `destroy_failed`), replacing the string literals and the stray `destroyed_failed`/`pending`. The event operation `refresh` becomes `read`.

Computed fields use a separate struct tag key, `xcl:"computed"`, on a field that must also be `hcl:",optional"`. An hcl option isn't possible because gohcl panics on unknown field kinds. Plugin schemas already carry the raw tag string, so no schema or proto change is needed. `computed` works at any depth, not just on a resource's own fields. A `network` block nested in a `container` can have a configured `name` and `subnet` next to a computed `id`, and a block inside that block can have computed fields too. This is the pattern jumppad's container uses today for `image.id` and each network's `assigned_address`, carried over by hand-written code. Carrying values over, or checking them, inside a list of blocks means pairing each configured block with its saved counterpart. A block element type can mark its identity fields `xcl:"key"`, for example a network's `id`, and elements are paired by those keys. Without a key they are paired by position, and in maps of blocks by map key. One small reflection helper set in `internal/parser` serves three places:

- The existing empty `validateStructure` stage uses it to report every computed field set in config, at any depth. Each error names the resource, the field's full path (e.g. `network[0].assigned_address`) and the attribute's position.
- `resourceLifecycle` uses it to copy computed values from the old copy onto the configured one before `Read`, recursing into nested blocks and pairing list elements by key or position.
- The "provider changed a configured value" warning uses it. After create, read and update, the check compares a JSON snapshot taken before the call with the result, over non-computed, non-`ResourceBase` fields at every depth. It skips attributes whose expression contains a traversal rooted at `resource` or `module`, found with `hclsyntax.Expression.Variables()`, which covers every expression shape. The warning is logged as a structured `Warn` with the resource and field (convention: structured logs).

The example provider stops rewriting `Description`. It adds a computed field, drops its own `Changed` and embeds `DefaultChanged`. The dead `internal/parser/plugins.go` is deleted, and the plugin developer guide is finished from the draft.

Why this direction: a dedicated lifecycle component with a progress recorder keeps each rule in one testable place, and it works with the walker's existing failure semantics instead of replacing the walker. Growing `callProviderLifecycle` in place would push a function that is already 130 lines past 300, mixing five paths, three kinds of reflection and state assembly. Forking the DAG library to expose per-vertex results is heavier and still needs a status per resource. The rejected alternatives, with evidence, are in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Resource lifecycle (new, parser).** Decides and runs the provider calls for one decoded resource. There are four paths:
  - **create**: the resource isn't in the previous state.
  - **read → changed → update**: the previous status is `created` or `updated`. This path also copies computed values from the old copy onto the configured one before the read. A `Read` that reports not found switches to create.
  - **rebuild**: the previous status is `failed` or `destroy_failed`. The resource is destroyed using its old copy, then created.
  - **unchanged**: nothing differs. The resource keeps what `Read` returned and its previous status.

  It fires the parser events for every provider call and sets the resource's status. After each create, read and update it asks the configured-value checker for warnings. It records each resource's outcome in the apply progress. It depends on the provider resolver, the previous state and the computed-field helpers. Builtin types (variable, output, module) keep today's behaviour: no provider calls and a single `create` success event. This component replaces both the current inline lifecycle function and the unused duplicate in the parser.
- **Apply progress (new, parser).** A concurrency-safe record of what happened to each resource during the walk: completed, or failed in a provider call, with the copy to save. The DAG walker runs vertices in parallel and exposes no per-vertex results, so the lifecycle writes here. When a walk fails, the parser combines this record with the previous state to build the state to save. Unreached resources that existed before keep their previous entry, and unreached new resources are omitted.
- **Walk callback (changed, parser).** It still decodes each resource and handles disabled resources and module variables. It now delegates provider work to the resource lifecycle and passes it the previous state, instead of the nil placeholder it uses today.
- **Parser apply (changed, parser).** Passes the previous state it loads into the walk. On success it returns the current state. When the walk fails it returns the state built from the apply progress *and* the error. Parse and validation failures still return no state.
- **Config apply (changed, root API).** Saves whatever state the parser returns, even when the apply failed, and then returns the apply's error. It still saves nothing when parsing or validation fails.
- **Computed-field helpers (new, parser).** Reflection utilities shared by three consumers:
  - validation, which uses the list of computed fields (by hcl name) on a resource type;
  - the lifecycle, which copies computed values from one resource copy to another;
  - the configured-value checker, which gets the non-computed, non-metadata fields to compare.

  A field is computed when it carries the `xcl:"computed"` tag, at any depth: on the resource itself, or inside nested blocks, pointer blocks, lists of blocks and maps of blocks. Block elements in a list are paired between copies by fields tagged `xcl:"key"`, or by position when there are none.
- **Configured-value checker (new, parser).** Compares a resource copy from before a provider call with the one after it. For each non-computed field whose value changed, it logs a structured warning naming the resource and field. Fields whose configuration expression references another resource or a module are exempt. The reference check uses the resource's retained HCL body. The checker never fails the apply.
- **Validation (changed, parser).** Its structure stage, empty today, reports every computed field set in configuration. Each error names the resource and field at the attribute's position. It also reports a computed field that isn't declared optional, as a type-author error. It runs before any provider is called, so a config with a computed field set reaches no provider.
- **Provider contract (changed, plugins).** `ResourceProvider[T]` loses `Refresh` and gains `Read(ctx, old, new)`. The package exports `ErrNotFound`, which providers return from `Read` for a missing resource. It also exports `DefaultChanged[T]`, an embeddable type whose `Changed` does the flat comparison and ignores xcl's resource metadata. Providers that define their own `Changed` override it through normal method promotion.
- **Provider adapter and plugin plumbing (changed, plugins).** The typed adapter, plugin base, plugin host interface and direct host move from `Refresh(data)` to `Read(oldData, newData)`, passing `ErrNotFound` through unchanged.
- **gRPC wire (changed, plugins).** The proto's `Refresh` rpc and messages become `Read`, carrying old and new data plus a `not_found` flag in the response. The server sets the flag from `ErrNotFound`. The host client turns it back into an error that wraps `ErrNotFound`, so the parser can use `errors.Is` whether the plugin runs in-process or out-of-process. The generated code is regenerated.
- **Resource statuses (changed, types).** Named constants for the five allowed statuses replace the string literals across the parser and the pretty printer. The `Meta.Status` documentation lists exactly these five.
- **Parser events (changed, parser).** The operation name `refresh` becomes `read`. Everything else about events is unchanged.
- **Test plugin (changed, parser tests).** The in-repo test provider gains a renamed `Read(old, new)` that records the old and new copies it received. It also gains per-resource settings: `Read` returns not found, `Read` sets an observed value to simulate drift, `Changed` result overrides, destroy errors, and ordered call recording (so tests can see destroy-then-create).
- **Example plugin (changed, plugins/example).** The person provider stops rewriting its configured `Description`. It gains a computed field set at create, drops its hand-written `Changed` in favour of the embedded `DefaultChanged`, and implements `Read`.
- **Plugin test helpers and generated mocks (changed).** The helpers for plugin authors and the mockery mocks are updated for the new adapter shape.
- **Documentation (changed).** The plugin developer guide is finished from the draft. The parser-lifecycle, state, overview, plugins and example README docs are updated for `Read`, the five statuses, `read` events, failed-rebuild handling and state saved after a failed apply.

## Data Structures & Interfaces

**Provider contract (`plugins`)**: breaking change. `Refresh` is removed.

```go
var ErrNotFound = errors.New("resource not found") // returned by Read when the real resource is gone

type ResourceProvider[T any] interface {
    Init(state State, functions ProviderFunctions, logger Logger) error
    Create(ctx context.Context, resource T) (T, error)
    Read(ctx context.Context, old T, new T) (T, error)   // was Refresh(ctx, resource T)
    Changed(ctx context.Context, old T, new T) (bool, error)
    Update(ctx context.Context, resource T) (T, error)
    Destroy(ctx context.Context, resource T, force bool) error
    Functions() ProviderFunctions
}

// Embed in a provider to get default change detection; defining Changed yourself overrides it.
type DefaultChanged[T any] struct{}
func (DefaultChanged[T]) Changed(ctx context.Context, old T, new T) (bool, error)
```

`DefaultChanged` compares the JSON form of both copies. It ignores xcl's own resource fields (`meta`, `depends_on`, `disabled`).

**Adapter and host shape (`plugins`)**: `ProviderAdapter.Refresh(ctx, data)` becomes `Read(ctx, oldData, newData []byte) ([]byte, error)`. The same rename applies to `PluginEntityProvider`, `PluginHost`, `DirectPluginHost`, `GRPCPluginHost` and `GRPCResourceProviderAdapter`. Callers find a not-found result with `errors.Is(err, plugins.ErrNotFound)`, whether the plugin runs in-process or out-of-process.

**Wire protocol (`plugins/plugin.proto`)**: breaking change.

```proto
rpc Read(ReadRequest) returns (ReadResponse);          // replaces rpc Refresh
message ReadRequest  { string entity_type = 1; string entity_sub_type = 2;
                       bytes old_entity_data = 3; bytes new_entity_data = 4; }
message ReadResponse { bytes entity_data = 1; string error = 2; bool not_found = 3; }
```

`RefreshRequest` and `RefreshResponse` are removed.

**Computed and key markers (resource type authors)**: `xcl` struct tag options, allowed on fields at any depth. A computed field must also be `optional`. `key` marks the identity fields used to pair elements of a list of blocks between the saved and the configured copy.

```go
type Container struct {
    types.ResourceBase `hcl:",remain"`
    ContainerID string              `hcl:"container_id,optional" json:"container_id,omitempty" xcl:"computed"`
    Networks    []NetworkAttachment `hcl:"network,block" json:"networks,omitempty"`
}

type NetworkAttachment struct {
    ID              string `hcl:"id" json:"id" xcl:"key"`                           // user-set, pairs elements
    IPAddress       string `hcl:"ip_address,optional" json:"ip_address,omitempty"`
    AssignedAddress string `hcl:"assigned_address,optional" json:"assigned_address,omitempty" xcl:"computed"`
}
```

**Resource statuses (`types`)**: typed constants. `Meta.Status` holds only these values.

```go
const (
    StatusCreated       = "created"
    StatusUpdated       = "updated"
    StatusFailed        = "failed"
    StatusDestroyed     = "destroyed"
    StatusDestroyFailed = "destroy_failed"
)
```

**Parser events**: `ParserEvent` is unchanged in shape. `Operation` now takes the values `create`, `read`, `changed`, `update` and `destroy`. `refresh` is gone.

**Parser apply result (`internal/parser`)**: the signature `Apply(paths ...string) (*state.State, error)` is unchanged, but the contract changes. A walk failure returns a **non-nil partial state together with a non-nil error**. Parse and validation failures still return `nil, err`. `Config.Apply` relies on this to decide whether to save.

**Internal parser types (new, unexported)**:

```go
// one per walk; shared by all walk-callback goroutines
type resourceLifecycle struct {
    previous *state.State; resolver ProviderResolver; options *ParserOptions
    bodies   map[string]*hclsyntax.Body   // for the reference exemption
    progress *applyProgress
}
func (l *resourceLifecycle) apply(r any) error

// concurrency-safe record of each resource's outcome
type applyProgress struct { mu sync.Mutex; outcomes map[string]outcome }
type outcome struct { failed bool; saved any } // saved = the copy to persist
func (p *applyProgress) record(id string, o outcome)
func (p *applyProgress) buildState(current, previous *state.State) (*state.State, error)
```

The computed-field helpers keep their responsibilities: list computed fields, copy computed values, and list comparable fields. Their exact signatures are left to implementation.

**Serialization boundaries**: provider calls keep exchanging resources as JSON (`[]byte`). The state file format keeps its shape, but only the five statuses appear. Earlier state files don't have to load (spec constraint).

## Implementation Detail

**One lifecycle decision, written as a table.** Today one function handles the whole provider lifecycle. It branches on "is there a previous resource" with nested error handling, and one path is never reached. A reader of the new lifecycle component sees one small dispatch keyed on the previous-state entry: absent → create; `created`/`updated` → read path; `failed`/`destroy_failed` → rebuild. Each path is its own short method, and all of them use a single helper that wraps a provider call. That helper fires the `start` event, times the call, fires `success` or `error`, and on success unmarshals the provider's result into the resource. Every provider call therefore gets identical event bracketing and error wrapping. The error message always names the operation and the resource, for example `read failed for resource.container.base: …`. Statuses are set in one place per path, using the typed constants.

**The read path in order.** The parser works through these steps:

1. Copy computed values from the old copy onto the configured copy.
2. Snapshot the configured copy.
3. Call `Read(old, new)`. Not found → switch to the create path. Any other error → the resource becomes `failed`.
4. Warn about changed configured values, comparing the snapshot with the read result.
5. Call `Changed(old, read result)`. If it reports a change, call `Update` with the read result, warn about changed configured values again, and set the status to `updated`.
6. Otherwise keep the read result and copy the previous status.

The order matters. Computed values are carried over *before* the read, so a provider with no custom `Read` logic still has them, and the default change detection sees equal computed fields on both sides. A diff is only reported for real configuration edits or for drift that `Read` observed.

**Progress recording instead of walker changes.** The DAG walker stays a black box. It already skips the dependents of a failed resource and lets independent resources finish. The lifecycle writes every outcome to the apply progress record. After a failed walk, the parser builds the state to save from three sources: the current resources the progress record marks as reached, the previous entries of resources that were not reached, and nothing for new resources that were not reached. This is a new pattern in the parser. Previously the walk's only product was the side effect on shared resource pointers plus an error list. Now it also produces an outcome map, and state assembly is a pure function over (current, previous, progress). That function can be unit-tested with no walk at all.

**Reflection lives in one helper set.** The computed marker is read in three places: validation, carry-over and the warning check. All three use one set of helpers that walk a resource type recursively. They flatten embedded structs the same way the existing property-name lookup does, and descend into block structs, pointers to blocks, slices of blocks and maps of blocks. Each computed field is addressed by a path such as `network[id=a].assigned_address`. When walking two copies together, the helpers pair list elements by their `key` fields, or by index when a type declares no key. Map elements are paired by map key, and an element present on only one side is skipped. This replaces the hand-written, per-resource carry-over loops that jumppad's container needs today. Plugin types reach the parser as schema-rebuilt structs whose raw tags are preserved, so the helpers must work purely from `reflect` and never from the provider's own Go type. The configured-value check compares JSON snapshots taken before and after each provider call, the same boundary providers already use. This avoids deep-copying structs. The reference exemption is worked out from the resource's retained HCL body: an attribute is exempt when its expression's variables include a traversal rooted at `resource` or `module`.

**Provider authors see less, not more.** For a plugin author the contract gets smaller in practice. Embed `DefaultChanged`, implement `Read` to fill in observed fields, tag provider-owned fields `computed`, and return `ErrNotFound` when the resource is gone. The example provider is rewritten to show exactly this and nothing more, and it acts as the reference implementation that the developer guide points at. The gRPC layer keeps its existing convention of errors carried in-band as strings. Not-found is the one signal that gets its own response field, because error identity doesn't survive the wire.

**Existing patterns followed.**
- Parser errors go through the existing `ParserError` constructors.
- Validation findings are added to the existing staged validation and gathered, not returned on the first one.
- Tests use the existing `setupParser` and `requireEvent` helpers and the in-repo test plugin.
- Mocks are regenerated with the project's mockery v3 config.
- The proto is regenerated with the project's existing proto generation target.

The dead duplicate lifecycle function in the parser is deleted rather than updated, so there is exactly one lifecycle implementation to read.

## Dependencies

- **Spec `20260918165700-provider-lifecycle-read`**: the source of scope, requirements and acceptance criteria. It is final, so nothing needs to land before this plan starts.
- **Prior work: the validation stage before the walk (plan `20260918075104-validation-phase-before-walk`, already merged)**: provides the staged `validate()` step and its empty structure stage, where the computed-field check goes. No change needed beyond adding the check.
- **Prior work: the `Parse` → `Apply` rename and the `requireEvent` test helper (already merged on `v2`)**: the lifecycle tests build on these. No change needed.
- **`github.com/silas/dag`, replaced by `github.com/jumppad-labs/dag` (pinned pseudo-version)**: the walker. Its skip-dependents / run-independents failure semantics are relied on as-is. No change or upgrade.
- **`github.com/hashicorp/hcl/v2` (v2.21.0), including `gohcl` and `hclsyntax`**: provides body decoding (which is why `computed` must be its own tag key), and `Expression.Variables()` for the reference exemption. No version change.
- **`github.com/creasty/defaults`**: the existing precedent for a separate struct-tag key. Not changed.
- **`google.golang.org/grpc`, `google.golang.org/protobuf`, `hashicorp/go-plugin`**: the plugin wire. The proto is regenerated with the toolchain already recorded in the generated headers (protoc 3.21.12, protoc-gen-go 1.36.10, protoc-gen-go-grpc 1.5.1). No module version bumps.
- **mockery v3 (v3.5.5 is installed locally)**: regenerates the `ProviderAdapter` mock, and any others whose interfaces change. Note that the Makefile's `install-mockery` target installs v2, which doesn't match the v3 config. Use the installed v3 binary. Fixing the Makefile target is optional.
- **Standard library (`reflect`, `encoding/json`, `errors`, `sync`)**: used for `DefaultChanged`, the computed-field helpers, the not-found sentinel and apply progress. No new third-party dependencies.
- **Internal packages changed**:
  - `plugins`: contract, adapter, hosts, gRPC, proto.
  - `internal/parser`: lifecycle, walk, validation, events, test plugin.
  - `types`: statuses.
  - root `xcl`: `Config.Apply` saves state after a failed apply.
  - `logger`: pretty-printer status colours.
  - `plugins/example` and `plugins/testing`: updated for the new contract.
  - `state` itself is unchanged; it's used through `FindResource` and `AppendResource`.

## Testing Approach

**Kinds of test.**

1. **Parser scenario tests (the bulk).** These are integration-style unit tests. They drive `Parser.Apply` twice, against the in-repo test plugin, a real plugin registry and a real file state store in a temp directory. Everything between the two applies goes through the real path: schema-rebuilt resource types, JSON round-trips through the typed adapter, computed-tag reflection and state saved to disk and loaded back. **Every earlier state a scenario needs is produced by a real apply, never hand-written.** A test that needs a `failed`, `destroy_failed` or `updated` resource in the saved state gets it by running an apply that fails, or edits the config, in the way that produces that state. The following apply is then checked against it. This keeps test state from drifting away from what the system actually writes, and each scenario exercises the whole lifecycle along the way. Each acceptance criterion gets its own named test, with the positive and negative cases split into separate functions and no table-driven tests. Assertions use testify `require`, through the existing `setupParser`/`requireEvent` style helpers. The test plugin is extended to:
   - record the arguments and the order of each call;
   - return not found or an error from `Read`, per resource;
   - simulate drift by changing an observed field in `Read`;
   - force `Changed` results;
   - fail `Destroy`;
   - change a configured field, to trigger the warning.
2. **Focused unit tests** for pure pieces that don't need a walk:
   - `DefaultChanged`: equal copies, differing values, differences only in metadata, nil pointers.
   - The computed-field helpers: listing, copying, and embedded-struct flattening on a schema-rebuilt type. Nested computed fields in pointer blocks, lists of blocks (paired by key and by position, including reordered and added elements) and maps of blocks.
   - The configured-value checker: warns, stays silent for computed fields, stays silent for fields set by reference.
   - Apply-progress state assembly: reached, failed, unreached-existing and unreached-new resources.
3. **Validation tests** in the existing validation test style:
   - setting a computed field fails, naming the resource and field;
   - the provider receives no calls;
   - a config that doesn't set it passes;
   - a non-optional computed field is reported.
4. **Contract tests for the wire.** In-process and external (built binary) tests of the example plugin cover `Read` with old and new data. They also check that a not-found `Read` comes back through gRPC as an error for which `errors.Is(err, ErrNotFound)` is true. That last check is the main guarantee that the sentinel survives the process boundary.
5. **Root `Config` tests.** A failed apply still calls `Save` with the partial state. Invalid or malformed configuration still never calls `Save`, as the existing tests already assert.
6. **Regression updates.** The existing event tests switch from `refresh` to `read`. The refresh error test becomes the read error test. The existing destroy-lifecycle test expects a removed-resource destroy walk, which is a non-goal, so it is rewritten to cover the failed-resource rebuild instead. The plugin test helpers and example e2e tests move to `Read`.

**Coverage focus.** The lifecycle component gets the most coverage, because every requirement meets there: path selection, carry-over, warnings, statuses, events and progress. State assembly after a failure gets the next most, because a bug there silently loses or duplicates real resources.

**Load-bearing assertions, in plain language:**
- A repeat apply with nothing changed calls no create and no update.
- The first apply never reads.
- `Read` sees both the saved copy, with its provider-assigned ID, and the configured copy.
- Configuration edits and observed drift each produce exactly one update.
- Not found recreates the resource. Any other read error fails the apply and the resource is saved as `failed`.
- A failed resource is destroyed then created, in that order. A failing destroy leaves it `destroy_failed`, with no create.
- A failed apply saves what succeeded, never recreates it on the next apply, and doesn't save unreached new resources.
- Dependents of a failure get no calls, and independent resources complete.
- Only the five statuses ever appear, and read events use `read`.
- Setting a computed field fails validation before any provider call.
- Computed values survive an unchanged apply and stay visible to dependents.

**Success metrics:**
- *A second apply of unchanged configuration makes zero create and zero update calls to providers*: **behavioural test**. Apply the same config twice with the test plugin and assert that the second apply recorded zero creates and zero updates. It also runs against the example plugin with default change detection.
- *A failed apply followed by a successful one never creates a resource that the failed apply had already created*: **behavioural test**. The first resource succeeds and its dependent fails. On the next apply, with the error cleared, assert that the first resource gets no create and the dependent is rebuilt.
- *The example plugin needs no `Changed` of its own and relies on the default change detection*: **behavioural test**, plus a compile-time structural check. The example provider embeds `DefaultChanged` and defines no `Changed` method. An example-plugin test applies unchanged data twice and asserts `Changed` reports no change, then changes a field and asserts it reports a change.

**Deliberate gaps.**
- No test for destroying resources that were removed from configuration: that is a non-goal.
- No test that loads state saved by earlier versions: not required.
- Pre-existing failures in unrelated packages are out of scope and are reported, not fixed: the `errors` package, the root file-location helpers with missing fixtures, and test-only build failures in `example`, `internal/functions` and `plugins/registry`. The "test suite passes" criterion is checked across every package this work touches.

## Milestones & Phases

### Milestone 1: Plugin authors write against the new provider contract

**What changes**: A plugin author now implements `Read(ctx, old, new)` instead of `Refresh`. They return `plugins.ErrNotFound` when the real resource is gone, and it is recognised the same way whether the plugin runs in-process or as a separate binary over gRPC. By embedding `DefaultChanged`, they get change detection without writing any. Every recorded resource status is one of the five agreed values, and parser events name the read operation `read`. The example plugin shows the intended shape. It no longer rewrites a configured field and has no `Changed` of its own. Apply behaviour is otherwise unchanged in this milestone: every apply still creates everything, because previous state doesn't reach the walk until Milestone 2. The unused duplicate lifecycle code is removed. This is a breaking change to the provider interface and wire protocol, which the spec allows.

**Validation point**: The whole module builds. The example plugin's in-process and external-binary tests pass using `Read`. A not-found `Read` over gRPC satisfies `errors.Is(err, plugins.ErrNotFound)`. `DefaultChanged` unit tests pass. No `Refresh` or `refresh` remains in code, the proto or the tests.

#### - [x] Phase 1.1: New provider contract, statuses and read events in-process
**Repo:** xclconfig

Replace `Refresh` with `Read(ctx, old, new)` across the provider interface, the typed adapter, the plugin base and the in-process host. Export the `ErrNotFound` sentinel and the embeddable `DefaultChanged[T]`. Introduce the five status constants, use them everywhere a status is set, and rename the `refresh` event to `read`. The parser keeps its current behaviour and just calls the renamed method, so everything still compiles. The gRPC path follows in the next phase.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-new-provider-contract-statuses-and-read-events-in-process)

**Acceptance criteria**:
- [x] A provider implements `Read(ctx, old, new)`, and no `Refresh` method exists on any provider, adapter or host interface.
- [x] A not-found error returned by an in-process provider's `Read` is still recognisable as `ErrNotFound` when it reaches the parser.
- [x] A provider that embeds `DefaultChanged` and defines no `Changed` satisfies the provider interface. It reports no change for identical resources, a change when a field differs, and no change when only xcl metadata differs.
- [x] Only the five agreed status values are set anywhere in the parser, and the misspelt `destroyed_failed` and the unused `pending` are gone.
- [x] Parser events for reading use the operation name `read`.

#### - [x] Phase 1.2: Read over the gRPC wire with a not-found signal
**Repo:** xclconfig

Change the plugin wire protocol so the `Read` call carries both the saved and the configured resource and reports "not found" as its own field. An out-of-process plugin's not-found result then reaches the parser as the same `ErrNotFound` an in-process one produces. Regenerate the protocol code and the mocks. While here, fix the gRPC client error reconstruction the linter already flags.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-read-over-the-grpc-wire-with-a-not-found-signal)

**Acceptance criteria**:
- [x] An external plugin binary receives both the old and the new resource on `Read`.
- [x] An external plugin returning `ErrNotFound` from `Read` produces an error on the host side that the parser recognises as not found.
- [x] Other external `Read` errors still arrive with their message intact, including messages containing `%`.
- [x] The protocol no longer defines a refresh call.

#### - [x] Phase 1.3: Example plugin, test plugin and helpers on the new contract
**Repo:** xclconfig

Rewrite the example person provider to be the reference implementation of the new contract. It implements `Read`, stops rewriting the configured `Description`, and embeds `DefaultChanged` instead of hand-writing `Changed`. Move the in-repo test plugin, the plugin-author test helpers and the example plugin's in-process and external tests to `Read`. Delete the unused duplicate lifecycle code in the parser.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-example-plugin-test-plugin-and-helpers-on-the-new-contract)

**Acceptance criteria**:
- [x] The example provider has no `Changed` method of its own, and its create, read and update leave configured fields alone.
- [x] The example plugin's in-process and external tests pass using `Read`, including a check that unchanged data reports no change and changed data reports a change through the default detection.
- [x] There is exactly one lifecycle implementation in the parser.
- [x] No reference to `Refresh` or `refresh` remains in code, tests or the proto (the unrelated, unused data-source interface excepted).

### Milestone 2: Repeat applies change only what really changed

**What changes**: Applying a configuration a second time now uses the state saved by the last apply:
- A resource that is unchanged gets a read and nothing else, and keeps its status.
- A configuration edit, or drift the provider observes in `Read` (a stopped container, a changed file), leads to exactly one update.
- A resource the provider reports as missing is created again.
- Any other read failure fails the apply with an error naming the resource. Saving that resource as `failed` lands in Milestone 3, together with saving state after any failed apply.

**Validation point**: Parser scenario tests that apply twice through a real file state store pass for the following cases:
- unchanged config;
- first apply never reads;
- `Read` receives both copies;
- a config edit leads to an update;
- drift leads to an update;
- a missing resource is recreated;
- a read failure fails the apply and makes no update;
- the default change detection;
- overridden change detection;
- an unchanged resource saves what was read;
- an unchanged resource keeps its status;
- read events.

#### - [x] Phase 2.1: Previous state drives create, read, change detection and update
**Repo:** xclconfig

Introduce the resource lifecycle component and feed it the state saved by the last apply, which the walk currently throws away. New resources are created without a read. Existing resources are read with both copies. A not-found read recreates the resource, and any other read failure fails the apply. Change detection runs on the read result, and an update happens only when it reports a change. An unchanged resource keeps what was read and its previous status. Extend the test plugin so the scenarios can be driven and observed.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-previous-state-drives-create-read-change-detection-and-update)

**Acceptance criteria**:
- [x] Applying the same configuration twice creates each resource once. The second apply only reads, and the saved status stays `created`.
- [x] The first apply with no saved state never calls read.
- [x] On the second apply, read receives the saved resource, including values the provider set at create, and the configured resource.
- [x] Editing a configured value leads to one update and a saved status of `updated`, with the new value saved.
- [x] Drift observed by read, with unchanged configuration, leads to one update.
- [x] A read reporting the resource missing leads to a create and no update, saved as `created`.
- [x] Any other read error fails the apply with an error naming the resource, and there is no update. The resource's status is set to `failed`; saving it is checked in Phase 3.1.
- [x] A provider overriding change detection has its own answer used.
- [x] After an unchanged apply, the saved resource holds what read returned. A resource saved as `updated` stays `updated`.
- [x] Read events carry operation `read` with `start`, then `success` or `error`.

### Milestone 3: A failed apply keeps what it built, and failed resources are rebuilt

**What changes**: When an apply fails part-way, xcl now saves state instead of throwing everything away:
- Resources that succeeded are saved with their new values and status.
- The resource that failed is saved as `failed`.
- Resources that existed before but weren't reached keep their previous entry.
- New resources that were never reached are left out.

Resources that depend on a failed one aren't processed, and independent resources still complete. The next apply therefore picks up where the failed one stopped. It never creates again something that was already built. A resource saved as `failed` is destroyed and then created on the next apply. If that destroy fails, the resource is recorded as `destroy_failed` and retried the time after.

**Validation point**: Scenario tests pass for:
- failure skips only dependents;
- a failed apply saves its progress, and the next apply doesn't recreate the first resource;
- a read failure is saved as `failed`;
- a failed resource is destroyed then created;
- a rebuild whose destroy fails is saved as `destroy_failed` and retried;
- only the agreed statuses appear.

Every failed or `destroy_failed` state these tests start from is produced by a real failing apply.

Root `Config` tests confirm that a failed apply calls `Save`, and that invalid configuration still never does.

#### - [x] Phase 3.1: Save progress when an apply fails
**Repo:** xclconfig

Record each resource's outcome during the walk. When the walk fails, build the state to save from that record:
- succeeded resources keep their new values and status;
- the failing resource is kept as `failed` (or `destroy_failed`);
- unreached resources that existed before keep their previous entry;
- unreached new resources are left out.

The parser returns this state together with the error, and the top-level apply saves it before returning the error. Invalid configuration still saves nothing.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-save-progress-when-an-apply-fails)

**Acceptance criteria**:
- [x] When one resource's create fails, a resource depending on it receives no calls. An independent resource is created and saved, and new resources never reached are absent from the saved state.
- [x] When the second of two dependent resources fails to create, the apply returns an error and the saved state holds the first as `created` and the second as `failed`.
- [x] The next apply, with the failure cleared, doesn't create the first resource again.
- [x] A read that fails with an error other than not found leaves the resource saved as `failed`.
- [x] A previously saved resource that the failed apply never reached is saved unchanged.
- [x] The top-level apply saves state after a provider failure, and still never saves when configuration is invalid or malformed.
- [x] Across all lifecycle scenarios, every saved status is one of the five agreed values.

#### - [x] Phase 3.2: Failed resources are rebuilt on the next apply
**Repo:** xclconfig

A resource saved as `failed` or `destroy_failed` is destroyed using its saved copy and then created, whether or not its configuration changed. If that destroy fails, the resource is recorded as `destroy_failed`, no create happens, and the next apply tries again. Every test here starts from state written by a real failing apply: first an apply whose create fails, then one whose destroy fails if needed. It never starts from a hand-built state file. The existing destroy test, which targets a removed-resource destroy walk that is out of scope, is replaced by these tests.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-failed-resources-are-rebuilt-on-the-next-apply)

**Acceptance criteria**:
- [x] After an apply in which a resource's create fails, the next apply, with no configuration change, gives that resource a destroy and then a create, in that order, and saves it as `created`.
- [x] When the rebuild's destroy fails, the resource is saved as `destroy_failed` and receives no create. The following apply gives it destroy then create.
- [x] No lifecycle test is skipped or commented out to make the suite pass.

### Milestone 4: Provider-owned computed fields, and a guide that explains the contract

**What changes**: A resource type can now mark fields as `computed` with a struct tag. These are values only the provider sets, such as IDs, connection strings or checksums:
- Setting one in configuration is a validation error that names the resource and field. It is reported before any provider is called.
- Computed values saved by the last apply are carried onto the configured resource before `Read`. They therefore survive an unchanged apply and stay visible to dependents, even when a provider's `Read` adds nothing.
- If a provider changes a value the user configured literally (not set by reference), xcl logs a warning naming the resource and field, and the apply carries on.

The example plugin gains a computed field. The plugin developer guide is completed. It covers the lifecycle, each operation's inputs, what it may change and what it returns, the read rules, computed fields, the change warning, failed-resource handling and destroying a missing resource. The lifecycle, state and plugin docs are brought in line.

**Validation point**: Tests pass for:
- setting a computed field fails validation;
- computed values survive an unchanged apply;
- a changed configured value warns;
- computed and reference-set fields don't warn.

The success metric that the example plugin relies on default change detection is shown by a test. The guide covers every item listed in the spec's "Guide covers the contract" criterion. The full suite for every touched package passes with no lifecycle tests skipped.

#### - [x] Phase 4.1: Computed fields and their validation
**Repo:** xclconfig

Add the `computed` and `key` struct tag options and the helpers that find computed fields at any depth on any resource type, including plugin types rebuilt from a schema. For example, a computed `assigned_address` inside a container's `network` blocks. Validation reports every computed field set in configuration, naming the resource and the field's path at the attribute's location, before any provider is called. It also reports a computed field that isn't declared optional.

*Technical detail:* [context.md#phase-41](./context.md#phase-41-computed-fields-and-their-validation)

**Acceptance criteria**:
- [x] Configuration that sets a computed field fails validation with an error naming the resource and the field, and the provider receives no calls.
- [x] Setting a computed field inside a nested block (e.g. a container's `network` block) also fails validation, naming the nested field.
- [x] Configuration that leaves computed fields unset passes validation.
- [x] A resource type declaring a computed field that isn't optional is reported as an error.

#### - [x] Phase 4.2: Computed values carry over, and changed configured values warn
**Repo:** xclconfig

Before read, copy the computed values saved last time onto the configured resource, including those inside nested blocks. List elements are paired by their key fields, or by position. Unchanged resources then keep them, and dependents can reference them even when a provider's read adds nothing. After each create, read and update, log a warning naming the resource and field whenever the provider returned a different value for a non-computed field. Fields set by reference to another resource are exempt, and the apply continues. Give the example plugin a computed field set at create.

*Technical detail:* [context.md#phase-42](./context.md#phase-42-computed-values-carry-over-and-changed-configured-values-warn)

**Acceptance criteria**:
- [x] With a provider that has no custom read and uses the default change detection, a second apply of unchanged configuration performs no update. A dependent that references a computed value still sees the value from the first apply.
- [x] A computed value inside a nested block, such as a network attachment's assigned address, is carried over to the matching block even when the blocks are reordered in configuration, provided the element type declares a key.
- [x] A provider changing a literally configured, non-computed field produces a warning naming the resource and field, and the apply succeeds.
- [x] A provider setting a computed field produces no warning, and neither does a field set by reference to another resource.
- [x] A second apply of unchanged configuration against the example plugin makes no create and no update calls.

#### - [x] Phase 4.3: Plugin developer guide and lifecycle docs
**Repo:** xclconfig

Turn the rough draft into the plugin developer guide and bring the other docs in line with the shipped behaviour. The guide covers:
- the lifecycle;
- each operation's inputs, what it may change and what it returns;
- the read rules, with the rule against recording self-changing values given prominence;
- computed fields;
- the warning and next-apply update caused by changing a non-computed value;
- failed-resource handling;
- that destroying a missing resource is the provider's decision.

The lifecycle, state, overview and plugin docs, the docs index and the example README drop `Refresh` and list exactly the five statuses.

*Technical detail:* [context.md#phase-43](./context.md#phase-43-plugin-developer-guide-and-lifecycle-docs)

**Acceptance criteria**:
- [x] The guide describes each provider operation's inputs, what it may change and what it returns.
- [x] The guide states the read rules (don't record self-changing values, prominently; don't change the real resource; report missing resources as not found). It also covers computed fields, the warning and follow-up update, failed-resource handling, and that destroying a missing resource is up to the provider.
- [x] The guide no longer calls itself a rough draft or lists gaps against the code.
- [x] The resource-status documentation lists exactly `created`, `updated`, `failed`, `destroyed` and `destroy_failed`.
- [x] No doc describes `Refresh`, a swallowed read error, or state being discarded on a failed apply.
- [x] Every package touched by this plan passes its tests with no lifecycle tests skipped. Pre-existing failures in untouched packages are reported.

## Open Questions

Only one uncertainty can't be settled until the code runs. Everything else is decided in the architecture and recorded in the assumption log.

- **Does the default comparison settle on real resource types?** `DefaultChanged` compares the JSON form of the saved copy with the configured-plus-read copy. These two copies take different routes to the comparison:
  - The saved copy comes from the state file, loaded into a schema-rebuilt struct.
  - The configured copy comes from HCL decode plus `creasty/defaults`.

  Fields without `omitempty` could come out as `null` on one side and `[]`/`{}` on the other, for example an absent block decoded as nil versus an empty slice restored from JSON. The same applies to maps, and to defaults applied on one path but not the other. Any such case would make the comparison report a change on every apply, breaking the "second apply does nothing" criterion.
  - *Depends on:* running the two-apply scenario tests against the test fixture types (containers with blocks, maps and defaults) and against the example plugin.
  - *What to do:* if a spurious difference appears, normalise inside `DefaultChanged` by treating `null`, empty slices and empty maps as equal. That's within the spec's "flat comparison ignoring metadata" intent, so record it as an assumption and carry on. If the difference comes from something else, for example a default the provider re-applies or a value the provider's JSON round-trip changes, **stop and ask the user**. The fix would then change the contract or the guide's advice.

## Out of Scope

- **Destroying resources removed from configuration.** Apply doesn't destroy anything that disappeared from config. The only destroy added is the one that rebuilds a failed resource. The existing destroy walk callback stays unused, and `Config.Destroy` stays a stub. *(Spec non-goal; no follow-up spec yet.)*
- **A plan or diff preview** of what an apply would change before it runs. *(Spec non-goal: "we can add a Diff later".)*
- **Parser events through the public `Config` API.** Events remain internal to the module, and tests reach them through the parser directly. *(Spec non-goal.)*
- **A drift-only check command or operation.** Reading happens only as part of an apply. *(Spec non-goal.)*
- **Retrying a failed provider call within the same apply.** Failed resources are dealt with on the next apply. *(Spec non-goal.)*
- **Compatibility with existing plugins or previously saved state.** The provider interface and wire protocol change without a shim, and old state files don't have to load. *(Spec constraints.)*
- **An XCL-owned fork of the HCL library with XCL-specific struct tags.** The intended direction is to fork HCL completely into an XCL version, accept that it will no longer receive upstream updates (and may be rewritten later), and let resource types use XCL tags such as `xcl:"name,optional,computed"` directly, with no separate `hcl` tag. This plan does not fork anything. It keeps the existing `hcl`/`json` tags and adds `computed` as a separate `xcl` tag key, which works on upstream HCL today and can be folded into the forked tag syntax later. *(User decision at walkthrough: follow-up spec.)*
- **The unused data-source provider interface.** Its own `Refresh(ctx)` method is untouched, because nothing registers or calls data sources. *(Design choice; rename it when data sources are implemented.)*
- **Pre-existing test failures in untouched packages.** These are reported, not fixed:
  - the `errors` package highlighting tests;
  - the root file-location helper tests with missing `.hcl` fixtures;
  - the test-only build failures in `example`, `internal/functions` and `plugins/registry`.

  *(Predate this spec; no follow-up tracked yet.)*
- **Commented-out legacy destroy tests** in the parser and config test files. They aren't skipped tests and are left as they are, unless deleting them is trivially safe. *(Cleanup, optional.)*
- **Making `FileStateStore.Save` atomic** and reporting resources dropped during state load because their type isn't registered. *(Existing state-store behaviour, unchanged here.)*

## Changelog

### 2026-09-19 — Phase 1.1: New provider contract, statuses and read events in-process

**What was done**: `ResourceProvider[T].Refresh` is replaced by `Read(ctx, old, new)` through the typed adapter, `PluginEntityProvider`/`PluginBase`, `PluginHost` and the direct and gRPC hosts. The package now exports `plugins.ErrNotFound`, which in-process adapters pass through unwrapped, and `plugins.DefaultChanged[T]`, which does a JSON comparison ignoring `meta`, `depends_on` and `disabled`. The five status constants live in `types/status.go` and are used by the parser and pretty printer. The parser event operation `refresh` is now `read`.

**Deviations**:
- The dead `internal/parser/plugins.go` was deleted here instead of in 1.3, because it called `adapter.Refresh` and the module must build at the end of every phase.
- Mocks were regenerated here instead of in 1.2.
- The `fmt.Errorf(resp.Error)` → `errors.New(resp.Error)` fix in `grpc_plugin_host.go` and `grpc_clients.go` was pulled forward from 1.2. The new `plugins` tests make `go test` run vet on that package, and vet fails on those calls.
- The example provider and the test plugin only had their signature renamed. Their rework stays in 1.3.
- The gRPC wrapper still sends `RefreshRequest` on the wire, carrying only the new data, until 1.2.

**Files changed**:
- `plugins/provider.go`
- `plugins/errors.go` (new)
- `plugins/changed.go` (new)
- `plugins/changed_test.go` (new)
- `plugins/adapter.go`
- `plugins/adapter_test.go` (new)
- `plugins/plugin.go`
- `plugins/plugin_host.go`
- `plugins/direct_plugin_host.go`
- `plugins/grpc_resource_adapter.go`
- `plugins/grpc_plugin_host.go`
- `plugins/grpc_server.go`
- `plugins/grpc_clients.go`
- `plugins/mocks/mock_provider_adapter.go`
- `plugins/example/pkg/person/provider.go`
- `plugins/example/e2e_test.go`
- `types/status.go` (new)
- `types/resource.go`
- `internal/parser/callbacks.go`
- `internal/parser/events.go`
- `internal/parser/parser.go`
- `internal/parser/test_plugin.go`
- `internal/parser/parse_test.go`
- `internal/parser/plugins.go` (deleted)
- `logger/pretty_printer.go`

**Discoveries**:
- The installed mockery v3.5.5 was built with go1.25. Under the local go1.27 it fails with `internal error: package "context" without types`, and `go run …@v3.5.5` fails the same way. Run it with a cached go1.25 toolchain: `PATH=$(go env GOMODCACHE)/golang.org/toolchain@v0.0.1-go1.25.6.linux-amd64/bin:$PATH GOTOOLCHAIN=local mockery`.
- `TestPluginResourceCreationWithFallback` already panics at 2daa13f (nil `pluginRegistry` under `DefaultOptions()`). It is missing from the plan's baseline failure list.
- `plugins/mocks` imports `plugins`, so tests inside package `plugins` can't use `mocks.MockState`. Use an in-test fake instead.

### 2026-09-19 — Phase 1.2: Read over the gRPC wire with a not-found signal

**What was done**: The proto's `rpc Refresh` is now `rpc Read`. `ReadRequest` carries the old and new entity data, and `ReadResponse` has a `not_found` flag. The server sets the flag with `errors.Is(err, ErrNotFound)`, and the host rebuilds the error as `fmt.Errorf("%w: %s", ErrNotFound, msg)`, so `errors.Is` works across the process boundary. The example provider's `Read` returns `ErrNotFound` when the saved email is `MissingPersonEmail` (`missing@example.com`).

**Deviations**:
- Mock regeneration and the `errors.New(resp.Error)` fix had already landed in 1.1.
- The example provider's `Read` got its final shape here (it logs, checks ctx, handles the not-found sentinel and returns `new` unchanged), not in 1.3.

**Files changed**:
- `plugins/plugin.proto`
- `plugins/proto/plugin.pb.go`
- `plugins/proto/plugin_grpc.pb.go`
- `plugins/grpc_server.go`
- `plugins/grpc_plugin_host.go`
- `plugins/grpc_plugin_host_test.go` (new)
- `plugins/example/pkg/person/provider.go`
- `plugins/example/e2e_test.go`

**Discoveries**: `make protos` with the local protoc-gen-go writes v1.36.11 into the generated file headers. The plan's research accepted this drift from v1.36.10.

### 2026-09-19 — Phase 1.3: Example plugin, test plugin and helpers on the new contract

**What was done**: The example `ExampleProvider` now embeds `plugins.DefaultChanged[*Person]` and has no `Changed` of its own. Its Create, Read and Update no longer rewrite `Description`. The example README lists `Read`, `Update` and the default `Changed`. Plugin authors get a `TestRead` helper in `plugins/testing`. The in-repo test plugin's `Refresh*` fields and methods are renamed to `Read*` (`ReadResources`, `ReadErrors`, `SetReadError`, `GetReadResources`).

**Deviations**:
- The dead `internal/parser/plugins.go` had already been deleted in 1.1.
- `TestRead` is a method on `TestPluginOperations`, next to `TestChanged`, instead of a free function.
- The leftover `refresh` wording in `callProviderLifecycle` was renamed, even though 2.1 deletes that function.

**Files changed**:
- `plugins/example/pkg/person/provider.go`
- `plugins/example/README.md`
- `plugins/example/e2e_test.go`
- `plugins/testing/helpers.go`
- `internal/parser/test_plugin.go`
- `internal/parser/parse_test.go`
- `internal/parser/callbacks.go`

**Discoveries**: The external e2e tests rebuild `plugins/example/build/example`, and that binary is tracked in git, so every test run leaves it modified.

### 2026-09-19 — Phase 2.1: Previous state drives create, read, change detection and update

**What was done**: The new `internal/parser/lifecycle.go` adds `resourceLifecycle`. It is built once per walk from the previous state and chooses the provider calls for each resource from that resource's previous entry:
- not in the previous state: create, with no read;
- previously `created` or `updated`: `Read(old, new)`. Not-found switches to create; any other error marks the resource `failed` and fails the apply. Then `Changed(old, read result)`, and `Update` only when it reports a change. An unchanged resource keeps the read result and its previous status.

A single `callProvider` helper fires the events for every provider call and wraps errors as `<op> failed for <id>: …`. `callProviderLifecycle` and the `previousParsed` TODO are gone. The test plugin can now be configured per resource (read not-found, observed drift, `Changed` overrides), records call order and the copies passed to `Read`, guards its maps with a mutex and uses the default change detection. Scenario tests use a real `FileStateStore` in a temp dir.

**Deviations**:
- A previous status other than `created`/`updated` takes the create path until 3.2 adds rebuild.
- The test plugin's `Read` reports the network's `ProviderID` from the saved copy, as a real provider would. Without that, `DefaultChanged` correctly saw the dropped identity field as a change on every apply. Phase 4.2's computed carry-over will make this automatic.
- Fixtures are in `lifecycle/original`, `lifecycle/edited` and `lifecycle/moved`. The plan's `dependent.xcl` is left for 3.1, which uses it first.

**Files changed**:
- `internal/parser/lifecycle.go` (new)
- `internal/parser/lifecycle_test.go` (new)
- `internal/parser/callbacks.go`
- `internal/parser/parser.go`
- `internal/parser/test_plugin.go`
- `internal/parser/types_test.go`
- `internal/test_fixtures/plugin/structs/network.go`
- `internal/test_fixtures/config/lifecycle/original/single.xcl` (new)
- `internal/test_fixtures/config/lifecycle/edited/single.xcl` (new)
- `internal/test_fixtures/config/lifecycle/moved/network.xcl` (new)

**Discoveries**:
- The plan's open question about spurious `DefaultChanged` differences (`null` vs `[]`) did not come up for networks. The only difference was a provider-set field that the provider's `Read` failed to return. The lesson: `Read` must return identity fields, or they must be computed and carried over.
- `-race` on `internal/parser` already reports data races at the base commit, all from tests' `OnParserEvent` closures appending to slices without locking.
- `Meta.File` is stored as the path given to Apply (it can be relative), even though its comment says absolute.

### 2026-09-19 — Phase 3.1: Save progress when an apply fails

**What was done**: The new `internal/parser/progress.go` adds `applyProgress`, a mutex-guarded record of each reached resource's outcome, and `buildState(current, previous)`. `resourceLifecycle.apply` records a success, or a failure when a provider call set the status to `failed`/`destroy_failed`. Errors before any provider call record nothing. `walkCallback` records the root node and disabled resources as reached. `Parser.walk` returns the progress, and `Parser.Apply` returns the state built from it together with the error when the walk fails. `Config.Apply` saves any non-nil state, even when the apply failed, and then returns the error. If the save also fails, the two errors are joined with `errors.Join`.

**Deviations**: None of substance. The progress type lives in its own `progress.go`, which the plan allowed.

**Files changed**:
- `internal/parser/progress.go` (new)
- `internal/parser/progress_test.go` (new)
- `internal/parser/lifecycle.go`
- `internal/parser/lifecycle_test.go`
- `internal/parser/callbacks.go`
- `internal/parser/parser.go`
- `config.go`
- `config_validate_test.go`
- `internal/test_fixtures/config/lifecycle/dependent/dependent.xcl` (new)

**Discoveries**:
- A resource saved as `failed` is the configured copy as it stood at the failing call. Provider-set fields that aren't computed, and so aren't carried over before `Read`, are lost. Phase 4.2's carry-over of computed values before `Read` is what keeps identity on failed resources, so providers must tag identity fields `computed`.
- Builtin `variable`/`output` entries are saved with no `status` key, and the root node is not saved.

### 2026-09-19 — Phase 3.2: Failed resources are rebuilt on the next apply

**What was done**: `resourceLifecycle` gained a `rebuild` path for a resource whose previous status is `failed`, `destroy_failed` or anything unrecognised. It calls `Destroy` with the saved copy, firing `destroy` events, then `create`. When the destroy fails, the resource's values are replaced with the saved copy, so it keeps the identity needed to retry, while its configured `Meta` is kept. The status becomes `destroy_failed` and no create happens. The apply-progress recorder saves that copy, and the next apply tries destroy then create again. `TestDestroyLifecycle`, which targeted the out-of-scope removed-resource destroy walk and panicked, is replaced by rebuild scenario tests that start from state written by real failing applies.

**Deviations**: None.

**Files changed**:
- `internal/parser/lifecycle.go`
- `internal/parser/lifecycle_test.go`
- `internal/parser/parse_test.go`

**Discoveries**: `Apply` errors are formatted diagnostics that wrap long lines. A test that matches a long substring of `err.Error()` can break on the wrap. Match a short prefix, or check the underlying error through the event's `Error`.

### 2026-09-19 — Phase 4.1: Computed fields and their validation

**What was done**: The new `internal/parser/computed.go` reads the `xcl` struct tag options `computed` and `key`, and provides one set of reflection helpers:
- `structFields` flattens embedded structs and skips `ResourceBase`;
- `blockElement` finds the struct inside struct, pointer, list and map fields;
- `computedFields` returns computed fields at any depth, with dotted paths;
- `copyComputed` pairs list elements by `key` fields or by position, and map elements by map key.

`validateStructure`, which was empty, now reports two things. A computed field that isn't `optional` is a type-definition error. A computed field set in configuration is reported at its attribute or block position, with its full path (e.g. `network[1].assigned_address`, `networkobj.provider_id`), before any provider is called. On the test fixtures, network `provider_id`/`observed`, `network.assigned_address` and `build.image_id` are computed, and `NetworkAttachment.id` is a key.

**Deviations**:
- The object-valued attribute check handles maps and lists of objects as well as single objects.
- Unrelated to the phase: `parse_test.go`'s event and order tests now guard their collected slices with a mutex. With every resource running through the lifecycle in parallel, `TestParserCreateEventErrorCallback` was intermittently losing events (about 2 in 200 runs).

**Files changed**:
- `internal/parser/computed.go` (new)
- `internal/parser/computed_test.go` (new)
- `internal/parser/validate.go`
- `internal/parser/validate_test.go`
- `internal/parser/parse_test.go`
- `internal/test_fixtures/plugin/structs/network.go`
- `internal/test_fixtures/plugin/structs/container.go`
- `internal/test_fixtures/config/computed_set/top.xcl` (new)
- `internal/test_fixtures/config/computed_set/nested.xcl` (new)
- `internal/test_fixtures/config/computed_set/unset.xcl` (new)
- `internal/test_fixtures/config/computed_set/non_optional.xcl` (new)
- `config_validate_test.go`

**Discoveries**:
- Raw `xcl` tags survive schema rebuilding, so the helpers work on registry-created plugin types without any schema change.
- `walkCallback` has an older data race: a disabled module calls `types.SetDisabled` on its child resources while other goroutines read them (`callbacks.go` ~95 vs ~49/109). It shows up under `-race` in `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled`. Out of scope here.

### 2026-09-19 — Phase 4.2: Computed values carry over, and changed configured values warn

**What was done**:
- **Carry-over.** Before `Read`, the lifecycle copies the saved copy's computed values onto the configured resource at any depth. List elements are paired by `key` fields or by position.
- **Warning.** The new `internal/parser/configured_check.go` compares JSON snapshots taken before and after each create, read and update. It logs `Warn("provider changed a configured value", "resource", id, "field", path)` for each changed non-computed field. Fields or blocks set by a `resource.`/`module.` reference are exempt, and a list whose length changed is reported once.
- **Test plugin.** `Read` adds nothing now, so carry-over alone keeps `ProviderID`. Container Create sets each block's `assigned_address`. A new `SetMutateConfigured` setting lets a provider change a configured value.
- **Example plugin.** It gained a computed `PersonID` set at Create.

**Deviations**:
- When `Read` reports not found, the resource is reset to its configured copy (computed values dropped) before Create, so a stale provider ID is never sent to Create. The plan didn't say this explicitly.
- `TestProviderChangingNestedConfiguredValueWarns` is covered by unit tests in `configured_check_test.go` instead of a scenario, because the test plugin only changes a network's subnet.
- The example plugin's two-apply test uses a new `.xcl` fixture (`plugins/example/testdata/people.xcl`), because the parser only discovers `.xcl` files.

**Files changed**:
- `internal/parser/configured_check.go` (new)
- `internal/parser/configured_check_test.go` (new)
- `internal/parser/recording_logger_test.go` (new)
- `internal/parser/lifecycle.go`
- `internal/parser/lifecycle_test.go`
- `internal/parser/test_plugin.go`
- `internal/test_fixtures/config/lifecycle/computed_ref/computed_ref.xcl` (new)
- `internal/test_fixtures/config/lifecycle/nested/nested.xcl` (new)
- `internal/test_fixtures/config/lifecycle/reordered/original/web.xcl` (new)
- `internal/test_fixtures/config/lifecycle/reordered/swapped/web.xcl` (new)
- `internal/test_fixtures/config/lifecycle/reference/reference.xcl` (new)
- `plugins/example/pkg/person/resource.go`
- `plugins/example/pkg/person/provider.go`
- `plugins/example/apply_test.go` (new)
- `plugins/example/testdata/people.xcl` (new)

**Discoveries**:
- The plan's open question is answered. A second apply of unchanged config made no update for networks, containers with nested blocks and maps, and the example plugin. No `null` vs `[]`/`{}` normalisation was needed in `DefaultChanged`.
- Reordering keyed blocks in config carries computed values correctly. `DefaultChanged` still sees the list order change and reports a change, which is a real configuration change.

### 2026-09-19 — Phase 4.3: Plugin developer guide and lifecycle docs

**What was done**: `docs/plugin-developer-guide.md` is now the finished guide. It no longer calls itself a draft and has no gaps list. It covers:
- the contract, the two copies and the kinds of field;
- the three lifecycle paths;
- each method's inputs, what it may change and what it returns;
- the read rules, with a prominent callout against recording values that change on their own;
- `DefaultChanged`, computed fields and `key`;
- the configured-value warning and the update it causes on the next apply;
- `ErrNotFound` over gRPC, failed-apply handling, the five statuses and events;
- that destroying a missing resource is the provider's decision.

`docs/README.md`, `parser-lifecycle.md`, `state.md`, `overview.md`, `plugins.md`, the example README and `TODO.md` now describe `Read`, the five statuses, `read` events, failed-resource rebuild and state saved after a failed apply. They also note that removed-resource destroy is still not implemented.

**Deviations**: None. The TODO entry is titled "Provider Lifecycle Read" rather than carrying this plan's phase number.

**Files changed**:
- `docs/plugin-developer-guide.md`
- `docs/README.md`
- `docs/parser-lifecycle.md`
- `docs/state.md`
- `docs/overview.md`
- `docs/plugins.md`
- `plugins/example/README.md`
- `TODO.md`

**Discoveries**: Final `go test ./...`: every touched package passes (`internal/parser` with the older `TestPluginResourceCreationWithFallback` panic skipped). The only other failures were there before this work:
- `errors` highlighting tests;
- root `TestReadResourceFromFileAtLocation`/`TestLineFromFileAtLocation`;
- test build failures in `example`, `internal/functions` and `plugins/registry`;
- `TestPluginResourceCreationWithFallback`.

The only `t.Skip` in the repo is an older Windows-only skip in `plugins/registry`.
