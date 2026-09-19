---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Context: 20260918165700-provider-lifecycle-read

## Current State Analysis

The lifecycle machinery exists but is mostly unreachable. What this plan starts from:

- **Previous state is loaded but discarded.** `Parser.parseAndValidate` loads it from the store (`internal/parser/parser.go:232-243`) and passes it to `walk` (`:200`). `walk` then hard-codes `previousParsed = nil` with a TODO (`:946-950`), so every resource takes the Create branch of `callProviderLifecycle` (`internal/parser/callbacks.go:299-320`) on every apply.
- **The existing-resource branch is wrong even if reached** (`callbacks.go:321-387`):
  - `Refresh` receives only the configured copy.
  - Its error is swallowed (`:334-336`).
  - `Changed` receives the configured JSON serialised *before* Refresh (`:287, :350`).
  - A resource saved as `failed` stays failed forever.
- **Statuses are inconsistent.** `types/resource.go:43` documents `pending`/`created`/`failed`. The code sets `created`/`updated`/`failed`/`destroyed`/`destroy_failed`/`destroyed_failed` (`callbacks.go:198-250, 308-386`), and nothing sets `pending`.
- **A failed apply saves nothing.** `Parser.Apply` returns `nil, ce` on walk errors (`parser.go:201-206`), and `Config.Apply` saves only on success (`config.go:106-119`).
- **The DAG walker (jumppad-labs/dag fork) already has the right failure semantics.** Dependents of a failed vertex are skipped, and independent vertices run in parallel and finish (`walk.go:376-439`). It exposes no per-vertex results.
- **Provider contract:** `ResourceProvider[T].Refresh(ctx, T)` (`plugins/provider.go:50-59`) threads through the adapter, plugin base, hosts, gRPC server and client, and `plugins/plugin.proto:11,73-82`. gRPC errors travel in-band as strings and are rebuilt with `fmt.Errorf(resp.Error)` (`plugins/grpc_plugin_host.go:105-191`), so sentinel identity is lost.
- **Plugin types are rebuilt on the host by `reflect.StructOf`** from a schema carrying raw tags (`internal/schema/serialize.go:53`, `deserialize.go:63-130`). A new tag key needs no schema change. gohcl panics on unknown hcl tag options.
- **Validation** has an empty `validateStructure` stage (`internal/parser/validate.go:145-147`).
- **Dead code:** `internal/parser/plugins.go` (`callPluginLifecycle`, no callers). `destroyWalkCallback` is defined but never walked.
- **The example provider rewrites its configured `Description`** in Create, Refresh and Update (`plugins/example/pkg/person/provider.go:41,87,137`), and hand-writes `Changed`.
- **Test baseline:**
  - `internal/parser` has `TestParserRefreshEventErrorCallback` failing and `TestDestroyLifecycle` panicking.
  - Unrelated pre-existing failures: the `errors` package, root `TestReadResourceFromFileAtLocation`/`TestLineFromFileAtLocation`, and test build failures in `./example`, `./internal/functions` and `./plugins/registry`.
- **The draft guide** is `docs/plugin-developer-guide.md`, untracked, with its "gaps" list.

## Per-Phase Technical Notes

All paths are in repo **xclconfig** (root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).

**Requirement → files resolution**

| Spec requirement | Files |
|---|---|
| Previous state drives each apply; new created, no read; existing read first; missing recreated; read failures fail; changed updated; unchanged keeps read + status | `internal/parser/lifecycle.go` (new), `internal/parser/callbacks.go`, `internal/parser/parser.go` |
| Read never changes the real resource; destroying a missing resource is the provider's decision | `plugins/provider.go` doc comments, `docs/plugin-developer-guide.md` |
| A failure skips only dependents; failed applies keep progress | `internal/parser/lifecycle.go` (applyProgress), `internal/parser/parser.go`, `config.go` |
| Computed fields: tag, rejected in config, carried over | `internal/parser/computed.go` (new), `internal/parser/validate.go`, `internal/parser/lifecycle.go` |
| Warn when provider changes a configured value | `internal/parser/configured_check.go` (new), `internal/parser/lifecycle.go` |
| Change detection sees reality; default; overridable | `plugins/changed.go` (new), `internal/parser/lifecycle.go` |
| Failed resources rebuilt; failed rebuild whose destroy fails | `internal/parser/lifecycle.go` |
| One set of statuses | `types/resource.go`, `types/status.go` (new), `internal/parser/callbacks.go`, `logger/pretty_printer.go`, docs |
| Events name the read | `internal/parser/events.go`, `internal/parser/lifecycle.go` |
| Refresh → Read(old,new); no compatibility | `plugins/*.go`, `plugins/plugin.proto`, `plugins/proto/*.pb.go`, `plugins/mocks`, `plugins/example`, `plugins/testing` |
| Plugin developer guide | `docs/plugin-developer-guide.md`, `docs/README.md` |

### Phase 1.1: New provider contract, statuses and read events in-process

**File changes**
- `plugins/provider.go:50-59`: replace `Refresh(ctx, resource T) (T, error)` with `Read(ctx context.Context, old T, new T) (T, error)`. The doc comment covers four points:
  - It is only called for resources in previous state, so old is never nil.
  - It fills identity, observed and derived fields into `new`.
  - It never writes config fields, never writes values that change on their own, and never mutates the real resource.
  - It returns `ErrNotFound` when the resource is gone.

  Update the `Create` comment at `:26-29`: it no longer "recreates a failed resource"; the parser destroys then creates. Update the `Destroy` comment at `:38-48`: destroying a missing resource is the provider's decision, and any error fails. Update the `Changed` comment at `:72-80`: `new` has already been through `Read`, and `DefaultChanged` can be embedded.
- `plugins/errors.go` (new): `var ErrNotFound = errors.New("resource not found")`, with a doc comment.
- `plugins/changed.go` (new): `type DefaultChanged[T any] struct{}` with a value-receiver `Changed(ctx, old, new T) (bool, error)`:
  - `json.Marshal` each side, then `json.Unmarshal` into `map[string]any`.
  - Delete the keys `meta`, `depends_on` and `disabled`. `types.ResourceBase` is embedded without a json tag, so its fields are promoted to the top level (`types/resource.go:50-58`).
  - Return `!reflect.DeepEqual(oldMap, newMap)`.
  - A nil pointer marshals to `null`. Treat `null` vs `null` as unchanged and `null` vs a value as changed.
- `plugins/changed_test.go` (new): separate positive and negative functions:
  - `TestDefaultChangedReportsNoChangeForIdenticalResources`
  - `TestDefaultChangedReportsChangeWhenFieldDiffers`
  - `TestDefaultChangedIgnoresMetaDifferences` (File/Line/Status differ)
  - `TestDefaultChangedIgnoresDependsOnAndDisabled`
  - `TestDefaultChangedHandlesNilResources`
- `plugins/adapter.go:27`: interface `Read(ctx, oldEntityData, newEntityData []byte) ([]byte, error)`. Replace `:223-249` (`Refresh`) with `Read`:
  - Unmarshal `old` and `new` separately; if either is nil, use the zero `T`.
  - Call `a.provider.Read(ctx, old, new)`.
  - Return the error unwrapped so `errors.Is` works, then marshal the result.
- `plugins/plugin.go:50` (`PluginEntityProvider.Refresh`) and `:176-184` (`PluginBase.Refresh`): become `Read(ctx, entityType, entitySubType string, oldEntityData, newEntityData []byte) ([]byte, error)`.
- `plugins/plugin_host.go:21-22`, `plugins/direct_plugin_host.go:50-53`: `Read` with the same shape.
- `plugins/grpc_resource_adapter.go:40-42`: `Read(ctx, old, new)`, calling `wrapper.Read`. Temporarily call the gRPC wrapper with the old request fields until Phase 1.2; do 1.1 and 1.2 together if simpler, since the module must compile at the end of each phase.
- `plugins/grpc_plugin_host.go:145-160, 261-267`: rename to `Read` and thread old/new. The wire change itself is Phase 1.2.
- `plugins/grpc_server.go:121-144`: rename to `Read` (wire change in 1.2).
- `types/status.go` (new): `StatusCreated`, `StatusUpdated`, `StatusFailed`, `StatusDestroyed`, `StatusDestroyFailed`. `types/resource.go:42-45`: comment lists exactly these five.
- `internal/parser/callbacks.go`:
  - `:198, :250`: `types.StatusDestroyed`.
  - `:207, :223`: `destroyed_failed` → `types.StatusDestroyFailed`.
  - `:240`: `types.StatusDestroyFailed`.
  - `:308, :320, :369, :381`: constants.
  - `:328-345`: call `adapter.Read(ctx, previousResourceJSON, resourceJSON)` and use event op `"read"`. Behaviour is otherwise unchanged until 2.1, where this function is replaced.
- `internal/parser/events.go:7`: Operation comment lists `create`, `read`, `changed`, `update`, `destroy`.
- `internal/parser/parse_test.go:1032`: allowed ops `read` instead of `refresh`. `:1089-1131`: rename to `TestParserReadEventErrorCallback` and use `"read"` (it keeps failing until 2.1; note that in the commit).
- `logger/pretty_printer.go:85, 119, 161-170, 865-872`: drop `pending`. Map `created`/`updated` → green, `failed`/`destroy_failed` → red, `destroyed` → grey or yellow, using the `types` constants.

**Complexity**: Medium. **Token estimate**: ~35k. **Agent strategy**: 2 parallel agents. Agent A: `plugins` contract, adapter, hosts, `DefaultChanged` and its tests. Agent B: `types` statuses, pretty printer, parser status and event renames. Integrate sequentially and build.

### Phase 1.2: Read over the gRPC wire with a not-found signal

**File changes**
- `plugins/plugin.proto:11`: `rpc Read(ReadRequest) returns (ReadResponse);`. Replace `RefreshRequest`/`RefreshResponse` at `:73-82` with:
  - `ReadRequest{entity_type=1, entity_sub_type=2, old_entity_data=3, new_entity_data=4}`
  - `ReadResponse{entity_data=1, error=2, not_found=3}`
- Regenerate `plugins/proto/plugin.pb.go` and `plugin_grpc.pb.go` with `make protos` (protoc 3.21.12, protoc-gen-go 1.36.10, protoc-gen-go-grpc 1.5.1, as recorded in the file headers).
- `plugins/grpc_server.go:121-144` → `Read`: on error, set `Error: errorToString(err)` and `NotFound: errors.Is(err, ErrNotFound)`. The missing-registered-type path at `:135` stays an ordinary error.
- `plugins/grpc_plugin_host.go:145-160` (`grpcPluginWrapper.Read`):
  - If `resp.NotFound`, return `fmt.Errorf("%w: %s", ErrNotFound, resp.Error)`.
  - Else if `resp.Error != ""`, return `errors.New(resp.Error)`.
  - Also replace every other `fmt.Errorf(resp.Error)` in this file (`:105, :122, :139, :156, :173, :191`) and `plugins/grpc_clients.go:63, :86` with `errors.New(resp.Error)`. This fixes the vet failures and `%` mangling.
- Regenerate mocks with the installed mockery v3.5.5 (`mockery`, via `.mockery.yml`). `plugins/mocks/mock_provider_adapter.go` loses `Refresh` and gains `Read`. Don't use `make install-mockery`, which installs v2; optionally fix that Makefile line to `github.com/vektra/mockery/v3@v3.5.5`.
- `plugins/example/e2e_test.go:174-187`:
  - Rename to `TestExternalPluginRead`; pass old and new.
  - Add `TestExternalPluginReadNotFoundIsErrNotFound`. It needs the example provider to return `ErrNotFound` on a sentinel input, e.g. `old.Email == "missing@example.com"` (document it in the example as a demonstration), and asserts `errors.Is(err, plugins.ErrNotFound)`.
  - Add `TestInProcessPluginReadNotFoundIsErrNotFound`.

**Complexity**: Medium. **Token estimate**: ~25k. **Agent strategy**: single agent, sequential (proto → regenerate → server/client → mocks → e2e).

### Phase 1.3: Example plugin, test plugin and helpers on the new contract

**File changes**
- `plugins/example/pkg/person/provider.go`:
  - Embed `plugins.DefaultChanged[*Person]` in `ExampleProvider`.
  - Delete `Changed` (`:92-113`).
  - Replace `Refresh` (`:64-90`) with `Read(ctx, old, new *Person) (*Person, error)`. It logs, checks ctx, returns `plugins.ErrNotFound` for the sentinel from 1.2, and otherwise returns `new` unchanged. The computed field is added in 4.2.
  - Remove the `Description` rewrites in `Create` (`:40-41`) and `Update` (`:136-137`).
  - Keep the compile-time assertion `:19`.
- `plugins/example/README.md:116-122`: lifecycle list uses `Read` and default `Changed`.
- `plugins/example/e2e_test.go:88-102, 163-166`:
  - Keep the "identical → false" checks.
  - Add `TestInProcessPluginChangedReportsChangeWhenFieldDiffers`, which covers the success metric that the default is enough.
- `plugins/testing/helpers.go:125-185`: no `Refresh` references. Add a `TestRead(t, host, type, subtype, old, new)` helper alongside `TestChanged` for plugin authors.
- `internal/parser/test_plugin.go`:
  - Rename `RefreshedResources`/`RefreshErrors`/`SetRefreshError`/`GetRefreshedResources` (`:16, :26, :33-35, :82-88`) to `Read*`.
  - `TestResourceProvider.Refresh` (`:248-266`) becomes `Read(ctx, old, new T)`: record the ID and return `new`.
  - `Init` `:108-187` resets the renamed fields.
  - Scenario knobs are added in 2.1.
- `internal/parser/plugins.go`: delete the file (dead `callPluginLifecycle`; no callers).

**Complexity**: Low. **Token estimate**: ~15k. **Agent strategy**: single agent, sequential.

### Phase 2.1: Previous state drives create, read, change detection and update

**File changes**
- `internal/parser/lifecycle.go` (new):
  - `resourceLifecycle{previous *state.State; resolver ProviderResolver; options *ParserOptions; bodies map[string]*hclsyntax.Body; progress *applyProgress}`. `progress` is added in 3.1; leave a nil-safe hook.
  - `apply(r any) error`:
    - Builtin types keep the current early return with a `create` success event (`callbacks.go:263-274`).
    - Resolve the adapter; a missing adapter is an error.
    - Look up `old, err := l.previous.FindResource(meta.ID)`. A `ResourceNotFoundError` means a new resource.
    - Dispatch on `oldMeta.Status`:
      - absent → `create`
      - `StatusCreated`/`StatusUpdated` → `readPath`
      - `StatusFailed`/`StatusDestroyFailed` → `rebuild` (3.2)
      - any other value → treat as `failed`, i.e. rebuild
  - `callProvider(op string, r any, fn func() ([]byte, error)) error`: fires start, success or error events with duration and data (keeping the `events.go` contract), unmarshals a non-empty result into `r`, and wraps errors as `fmt.Errorf("%s failed: %w", op, err)`.
  - `create(r)`: Create. Status `StatusCreated`, or `StatusFailed` plus an error.
  - `readPath(r, old)`:
    1. Marshal old and new, then call `Read`.
    2. `errors.Is(err, plugins.ErrNotFound)` → fire the read `error` event, then `create(r)`.
    3. Any other error → `StatusFailed`, return `read failed: …`.
    4. Unmarshal the result into `r`.
    5. Re-marshal `r`, then call `Changed(oldJSON, readJSON)`. A Changed error → `StatusFailed` plus an error.
    6. If changed → Update → `StatusUpdated`, or `StatusFailed` plus an error. Otherwise `meta.Status = oldMeta.Status`.
  - Previous-state `old` objects are separate pointers loaded from the store (`state/file_state_store.go:80-96`). Never mutate them.
- `internal/parser/callbacks.go`:
  - `walkCallback` signature `:31`: replace `previousParsed *parsed` and `registry ProviderResolver` with `lc *resourceLifecycle`, and replace the `callProviderLifecycle` call at `:150-157` with `lc.apply(r)`. Keep the error-wrapping message shape `provider lifecycle error: …` but include the resource ID.
  - Delete `callProviderLifecycle` (`:256-391`).
- `internal/parser/parser.go:946-950`: build `lc := &resourceLifecycle{previous: previousState, resolver: p.providerResolver, options: &p.options, bodies: p.parsedResources.bodies}` and pass it. Delete the TODO. `previousState` is never nil (`parser.go:241-243`).
- `internal/parser/test_plugin.go` (scenario knobs, each a per-ID map with a setter, reset in `Init`):
  - `ReadNotFound map[string]bool`, which returns `plugins.ErrNotFound`.
  - `ReadObserved map[string]string`: `Read` sets the observed computed field to this value (drift). The field is added to the fixture struct in this phase and tagged computed in 4.1; until then it is a plain optional field.
  - `ChangedResults map[string]bool`, overriding the default. The default becomes `plugins.DefaultChanged` behaviour: embed `plugins.DefaultChanged[T]` in `TestResourceProvider` and delete its hand-written `Changed` at `:290-316`. Keep call recording and `ChangedErrors` by wrapping: a `Changed` method that records, checks errors/overrides, then delegates to the embedded default.
  - `ReadCalls []ReadCall{ID string; Old, New []byte}` to assert both copies.
  - `Calls []string` of `"<op> <id>"` in order, for destroy-then-create.
  - `CreateSetsID bool` (default on): Create sets `ProviderID = "id-" + meta.Name` on the fixture struct, to prove old carries create-time values.
  - Guard shared slices and maps with a mutex, because the walker is parallel.
- `internal/test_fixtures/plugin/structs/`: add fields to `Network`:
  - `ProviderID string \`hcl:"provider_id,optional" json:"provider_id,omitempty"\``
  - `Observed string \`hcl:"observed,optional" json:"observed,omitempty"\``

  Add the same to `ContainerBase` if tests use containers. The `xcl:"computed"` tag is added in 4.1.
- `internal/test_fixtures/config/lifecycle/` (new): `single.xcl` has one `network "one"`. `dependent.xcl` has `network "first"`, plus `container "second"` referencing `resource.network.first.meta.id`, plus an independent `network "independent"`. Add an `edited/` variant with a changed `subnet`.
- `internal/parser/lifecycle_test.go` (new). Helper `setupLifecycleParser(t, dir string)` wraps `setupParser` with `StateStore: state.NewFileStateStore(filepath.Join(dir,"state.json"), registry)`. Build a new parser per apply, sharing the registry, plugin and store. One test per criterion:
  - `TestApplyUnchangedConfigTwiceCreatesOnceAndOnlyReadsOnSecondApply`
  - `TestFirstApplyNeverReads`
  - `TestReadReceivesSavedAndConfiguredCopies` (the old copy carries `ProviderID`)
  - `TestConfigEditTriggersUpdate`
  - `TestDriftTriggersUpdate`
  - `TestMissingResourceIsRecreated`
  - `TestReadFailureFailsApply`
  - `TestDefaultChangeDetectionNoUpdateWhenNothingDiffers`
  - `TestDefaultChangeDetectionUpdatesWhenValueDiffers`
  - `TestDefaultChangeDetectionIgnoresMetadata` (move the fixture file so `meta.file`/`line` differ)
  - `TestOverriddenChangeDetectionIsUsed`
  - `TestUnchangedResourceSavesWhatWasRead`
  - `TestUnchangedResourceKeepsUpdatedStatus`
  - `TestReadEventsUseReadOperation`
- `internal/parser/parse_test.go:1089-1131`: `TestParserReadEventErrorCallback` now passes. Switch it to the file-store helper.

**Complexity**: High. **Token estimate**: ~60k. **Agent strategy**: parallel analysis, sequential integration. One agent writes `lifecycle.go` and rewires the walk. A second, in parallel, extends the test plugin and fixtures. Then write the scenario tests sequentially against the integrated code.

### Phase 3.1: Save progress when an apply fails

**File changes**
- `internal/parser/lifecycle.go` (or `progress.go`, new):
  - `applyProgress{mu sync.Mutex; outcomes map[string]outcome}` and `outcome{failed bool; saved any}`, with `record` and `buildState(current, previous *state.State) (*state.State, error)`.
  - `buildState`: iterate `current.GetResources()`:
    - with an outcome → append `outcome.saved`;
    - without an outcome and `previous.FindResource(id)` succeeds → append the previous resource;
    - otherwise skip.

    Use `state.NewState()` plus `AppendResource`. AppendResource rewrites `meta.ID` from the FQRN (`state/state.go:186-209`), which is consistent.
  - The lifecycle records outcomes: success (`saved = r`), a provider-call failure (`failed=true, saved = r` or the old copy for `destroy_failed`), and builtins (`saved = r`).
  - `walkCallback` records `saved = r` for the root, disabled resources and module/builtin types that return nil early (`callbacks.go:42-55, 97-100`), so they count as reached.
  - Decode and context errors before `lc.apply` record nothing, so the resource counts as unreached (see the assumptions).
- `internal/parser/parser.go`:
  - `walk` (`:927-965`): return `(*applyProgress, []error)`.
  - `Apply` (`:187-209`): on walk errors, `partial, err := progress.buildState(currentState, previousState)`, then `return partial, ce`. If `buildState` itself fails, append that error to `ce` and return `nil, ce`. Update the doc comment `:171-186`.
- `config.go:93-122`:
  - `newState, err := p.Apply(...)`.
  - If `newState != nil`, set `c.currentState = newState` and save it. On a save error, return `errors.Join(err, fmt.Errorf("failed to save state: %w", saveErr))`.
  - Then return `err`.
  - Fix the doc comment at `:88-92`, which claims per-operation saves.
- Tests:
  - `internal/parser/progress_test.go` (new), unit tests for `buildState`:
    - `TestBuildStateKeepsReachedResources`
    - `TestBuildStateKeepsFailedResourceAsFailed`
    - `TestBuildStateKeepsPreviousEntryForUnreachedExistingResource`
    - `TestBuildStateOmitsUnreachedNewResource`
  - `internal/parser/lifecycle_test.go`:
    - `TestFailureSkipsOnlyDependents` (fixture `dependent.xcl`, `SetCreateError` on `first`)
    - `TestFailedApplySavesProgress` (error on `second`)
    - `TestNextApplyAfterFailureDoesNotRecreateSucceededResource` (covers the success metric)
    - `TestUnreachedPreviousResourceIsSavedUnchanged`
    - `TestOnlyAgreedStatusesAreSaved`: scan the state file after each scenario helper, or a combined scenario, and assert each status ∈ the five.
  - `config_validate_test.go`: add `TestApplySavesStateWhenProviderFails` (`SetCreateError` on the test plugin; `Save` called with a state containing the failed resource). The existing `:260-297` tests still assert that `Save` is not called.

**Complexity**: Medium. **Token estimate**: ~30k. **Agent strategy**: 2 parallel agents. Agent A: progress, `buildState` and its unit tests. Agent B: `Config.Apply` and the root test. Then integrate the walk and `Apply` wiring, and the scenario tests, sequentially.

### Phase 3.2: Failed resources are rebuilt on the next apply

**File changes**
- `internal/parser/lifecycle.go` `rebuild(r, old)`:
  1. Marshal old, then call `Destroy(ctx, oldJSON, false)` with `destroy` events.
  2. On error, set the status to `StatusDestroyFailed` on **old's** values: unmarshal the old JSON into `r` (keep `r`'s Meta file/line from config), set the status, and return `destroy failed: …`. The saved copy is then the one holding identity.
  3. On success, call `create(r)`.
  4. Record the saved copy in progress.
- `internal/parser/test_plugin.go`: `DestroyErrors`/`SetDestroyError` already exist (`:24, :59-66`). Make sure `Destroy` appends to `Calls`.
- `internal/parser/lifecycle_test.go`:
  - `TestFailedResourceIsDestroyedThenCreated`: first apply with `SetCreateError`, then `ClearErrors`, second apply; assert the `Calls` order `destroy X` then `create X`, and a status of `created`.
  - `TestFailedRebuildWithFailingDestroyIsSavedDestroyFailed`: no create call.
  - `TestDestroyFailedResourceIsRetriedOnNextApply`.

  Each test produces its starting state with a real apply, never a hand-written state file. First apply: `SetCreateError(X)`, which fails, and 3.1 saves `X` as `failed`. For the destroy-failure tests, the second apply runs with `ClearErrors` plus `SetDestroyError(X)`, which saves `destroy_failed`. The final apply clears errors and asserts the call order and status.
- `internal/parser/parse_test.go:1201-1248` (`TestDestroyLifecycle`, which panics today on the missing `Exists` stub and targets the non-goal destroy walk): delete it, since it is replaced by the tests above. Leave the commented-out destroy tests at `:1286-1479` untouched; they aren't skipped tests, they are dead comments, and removing them is optional cleanup.

**Complexity**: Medium. **Token estimate**: ~20k. **Agent strategy**: single agent, sequential.

### Phase 4.1: Computed fields and their validation

**File changes**
- `internal/parser/computed.go` (new):
  - `xclOptions(field) (computed, key bool)`: parse `field.Tag.Get("xcl")`, split on commas.
  - `walkFields(t reflect.Type, visit func(path fieldPath, field reflect.StructField))`: recursive walk. It flattens anonymous and `,remain` embedded structs like `collectPropertyNames` (`internal/parser/types.go:50-76`), using `hclTagName` (`:81-90`). It descends into struct, `*struct`, `[]struct`, `[]*struct` and `map[string]struct` field types (block and object attributes), and doesn't descend into `types.ResourceBase`, `cty.Value` or scalars. It guards against recursive types with a visited set, as `collectPropertyNames` does.
  - `computedPaths(t)`: every computed field with its hcl path. Paths look like `network.assigned_address`, `image.id` and `network.dns.address` for nested lists.
  - `copyComputed(dst, src reflect.Value)`: recursive and same-type.
    - Struct: copy computed fields, recurse into non-computed composite fields.
    - Pointer: recurse when both are non-nil.
    - Slice of structs: if the element type has `key` fields, build `src` elements by key tuple and, for each `dst` element, find the src element with an equal key and recurse. Otherwise pair by index up to `min(len)`.
    - Map: pair by map key.
    - Unpaired elements are left as configured.
    - A computed field whose own type is composite (e.g. a computed `[]NetworkAttachment`) is copied whole.
  - `comparableValues(v reflect.Value)`: a copy of the JSON view with computed fields removed at every depth, used by the configured-value checker. The implementation zeroes computed fields on an unmarshalled copy, recursively, and then compares per path.
  - `isOptional(field)`: `hcl` tag kind == `optional`.

  Works on schema-rebuilt types, whose raw tags are preserved (`internal/schema/deserialize.go:63-130`).
- `internal/parser/validate.go:145-147` (`validateStructure`): for each ID in `sortedResourceIDs()`, take `computedFields(reflect.TypeOf(r))`:
  - A computed field that isn't optional → error at the resource position: `resource '<id>' field '<name>' is computed and must be optional`.
  - Walk `bodies[id]` alongside the type. For each attribute whose field is computed → `errors.NewParserError(attr.SrcRange.Filename, attr.NameRange.Start.Line, attr.NameRange.Start.Column, "resource '<id>' sets computed field '<path>', computed fields are set by the provider and cannot be configured")`, where `<path>` includes the block path and index (e.g. `network[1].assigned_address`). Recurse into `body.Blocks` whose type maps to a block field, following the block's element type. A block whose field is itself computed is reported the same way. Object-valued attributes (`hcl:"x,optional"` with a struct type, e.g. `networkobj`): check the keys of an `hclsyntax.ObjectConsExpr` against computed fields of the struct.
  - Gather all errors. Stage ordering is unchanged (`validate.go:25-47`).
- `internal/test_fixtures/plugin/structs/`: tag `ProviderID` and `Observed` with `xcl:"computed"`. In `container.go:51-56` `NetworkAttachment`, tag `ID` with `xcl:"key"` and add `AssignedAddress string \`hcl:"assigned_address,optional" json:"assigned_address,omitempty" xcl:"computed"\``. In `Build` (`:88-93`, a pointer block), add `ImageID string \`hcl:"image_id,optional" json:"image_id,omitempty" xcl:"computed"\``. Check that existing fixtures under `internal/test_fixtures/config` don't set these. Add an unregistered-in-production test type with a non-optional computed field for the negative test, registered only in that test.
- `internal/test_fixtures/config/computed_set/` (new): `top.xcl` sets `provider_id` on a network. `nested.xcl` sets `assigned_address` inside a container's second `network` block.
- `internal/parser/validate_test.go`:
  - `TestValidateRejectsConfiguredComputedField` (asserts the message names the resource and field, and that the test plugin recorded zero calls)
  - `TestValidateRejectsConfiguredNestedComputedField` (message contains `network[1].assigned_address`)
  - `TestValidateAcceptsUnsetComputedField`
  - `TestValidateRejectsNonOptionalComputedField`
- `internal/parser/computed_test.go` (new):
  - `TestComputedFieldsFindsTaggedFieldsOnSchemaRebuiltType` (create via registry)
  - `TestComputedFieldsIncludesPromotedEmbeddedFields`
  - `TestCopyComputedCopiesOnlyComputedValues`
  - `TestComputedFieldsFindsNestedBlockFields`
  - `TestCopyComputedCopiesIntoPointerBlock`
  - `TestCopyComputedPairsListElementsByKey` (config reorders two networks)
  - `TestCopyComputedPairsListElementsByPositionWithoutKey`
  - `TestCopyComputedLeavesUnpairedElementsAlone` (a network added in config)
  - `TestCopyComputedPairsMapElementsByKey`
- `config_validate_test.go`: add `TestValidateRejectsConfiguredComputedField` at the `Config` level.

**Complexity**: High. **Token estimate**: ~45k. **Agent strategy**: parallel analysis, sequential integration. One agent writes `computed.go` (the walk, pairing and copy) with its unit tests. A second writes the validation walk over bodies with its tests. Integrate, then run the Config-level test.

### Phase 4.2: Computed values carry over, and changed configured values warn

**File changes**
- `internal/parser/lifecycle.go` `readPath`: before marshalling `new` for `Read`, call `copyComputed(r, old)`.
- `internal/parser/configured_check.go` (new): `warnChangedConfiguredValues(logger, id string, body *hclsyntax.Body, before, after []byte, t reflect.Type)`:
  - Unmarshal both into fresh `reflect.New(t.Elem())`.
  - Walk both copies together with the same pairing rules as `copyComputed`: key, then position, then map key. For every non-computed leaf field (and each whole slice or map of scalars) whose values differ and that isn't reference-set, call `logger.Warn("provider changed a configured value", "resource", id, "field", path)`, where `path` is like `network[0].ip_address`. An element added or removed by the provider in a list of blocks is reported once, at the list's path.
  - A field is reference-set when `body.Attributes[name]` exists and its `Expr.Variables()` includes a traversal with `RootName()` of `resource` or `module`. For nested fields, resolve the attribute in the matching nested block of the body (the block of that type at the element's position) and apply the same test.
  - `before` is the JSON snapshot taken just before each provider call; `after` is the result.
  - Call it from `lifecycle.go` after a successful create, read and update. The logger is `l.options.Logger` (`parser.go:65`); it never returns an error.
- `plugins/example/pkg/person/resource.go`: add `ID string \`hcl:"id,optional" json:"id,omitempty" xcl:"computed"\``. Check for a clash with `Meta.ID`'s JSON key: `meta` is nested, so there is no clash, but prefer `PersonID`/`person_id` for clarity. `Create` in `provider.go` sets it. `Read` leaves it alone (carried over).
- Test logger: `logger.NewTestLogger(t)` (`logger/test_logger.go`). Add a capturing option, or a small `recordingLogger` in the parser tests that implements `logger.Logger` and stores `Warn` calls, passed through `ParserOptions.Logger`.
- `internal/parser/test_plugin.go`: add `MutateConfigured map[string]string` (sets `Subnet` on the network in Create/Read/Update) and `SetComputedOnCreate` (already done through `ProviderID`).
- `internal/test_fixtures/config/lifecycle/reference.xcl`: a network `b` whose `subnet = resource.network.a.subnet`, for the exemption test.
- `internal/parser/lifecycle_test.go`:
  - `TestComputedValuesSurviveUnchangedApply` (no custom read, default Changed; a dependent `container` referencing `resource.network.one.provider_id` sees `id-one` on the second apply)
  - `TestProviderChangingConfiguredValueWarns`
  - `TestProviderSettingComputedFieldDoesNotWarn`
  - `TestReferenceSetFieldDoesNotWarn`
  - `TestNestedComputedValueSurvivesUnchangedApply` (the test provider's Create sets `AssignedAddress` on each network block; the second apply has no update, and a dependent referencing `resource.container.base.network[0].assigned_address` sees it)
  - `TestNestedComputedValueFollowsKeyWhenBlocksReordered`
  - `TestProviderChangingNestedConfiguredValueWarns`
- `internal/parser/configured_check_test.go` (new): unit tests for the checker, with positive and negative cases split.
- `plugins/example/e2e_test.go` or a new `plugins/example/apply_test.go`: `TestExampleProviderSecondApplyMakesNoCreateOrUpdate`. It uses `parser` from the same module (allowed: same module, `internal` is importable from inside `github.com/jumppad-labs/xcl/...`), or `xcl.Config` with `WithPlugin` and a file store, counting calls via `OnParserEvent`. Since `Config` doesn't expose events, use the parser directly. This covers success metric 1 and the example-plugin metric.

**Complexity**: High. **Token estimate**: ~40k. **Agent strategy**: parallel analysis, sequential integration. One agent writes `configured_check.go` with its unit tests, another the example plugin and its test. The lifecycle hook and scenario tests are integrated last.

### Phase 4.3: Plugin developer guide and lifecycle docs

**File changes**
- `docs/plugin-developer-guide.md`: rewrite from the draft.
  - Remove "rough/draft" and the "Gaps" section.
  - Keep "two copies", "kinds of field" (Identity/Observed/Derived become `computed`), lifecycle and methods.
  - Add:
    - a **prominent callout** against recording values that change on their own;
    - a `computed` tag section (optional, can't be set by users, carried over before Read);
    - the "changing a non-computed value → warning + update on the next apply" section;
    - failed-resource handling (destroy then create; `destroy_failed`);
    - "destroying a missing resource is your decision; any error fails";
    - an `ErrNotFound` section, including the gRPC note;
    - `DefaultChanged` embedding;
    - the saved state after a failed apply;
    - the five statuses.
  - Point to the example provider.
- `docs/README.md:15-20`: the lifecycle entry says `Read`; the guide entry drops "rough draft".
- `docs/parser-lifecycle.md`:
  - `:26-27`, `:61-80` (lifecycle diagram): read/changed/update, not-found, rebuild.
  - `:84-86`: the destroy walk is still unused; the rebuild destroy is described.
  - `:125-190` events: `read`; the "Errors and control flow" section at `:178-184` says a read error fails the apply.
  - Add a "state saved after a failed apply" subsection.
- `docs/state.md:15-17`: the five statuses. `:56-59`: `Parser.Apply` naming, and state saved after a failed apply.
- `docs/overview.md:58, :96`: statuses and when state is saved.
- `docs/plugins.md:35, :57, :89`: `Read`, `ErrNotFound`, `DefaultChanged`, the proto `not_found`.
- `plugins/example/README.md`: already updated in 1.3; add the computed field.
- `TODO.md:56-101, 195-199`: strike or update the items this plan completes.
- Final verification: run tests for every touched package (`.`, `./internal/parser/...`, `./plugins/...`, `./state/...`, `./types/...`, `./logger/...`) and confirm no `t.Skip` was added. Report the pre-existing failures in `./errors`, the root `TestReadResourceFromFileAtLocation`/`TestLineFromFileAtLocation`, and the test build failures in `./example`, `./internal/functions` and `./plugins/registry` as unchanged.

**Complexity**: Low. **Token estimate**: ~20k. **Agent strategy**: single agent, sequential.

## Testing Strategy

Per phase (the full strategy is in plan.md § Testing Approach):

- **1.1**: `plugins/changed_test.go` unit tests for `DefaultChanged` (identical, differing, meta-only, depends_on/disabled, nil). The parser event tests switch to `read`. The whole module builds.
- **1.2**: example e2e tests, in-process and external binary. `Read` with old and new; not-found satisfies `errors.Is(err, plugins.ErrNotFound)` across gRPC; ordinary errors keep their message.
- **1.3**: example e2e tests for default change detection, both positive and negative. The test plugin compiles against `Read`.
- **2.1**: `internal/parser/lifecycle_test.go` scenario tests through a real `FileStateStore` in `t.TempDir()`, one per acceptance criterion; see Phase 2.1 for the names. `TestParserReadEventErrorCallback` passes.
- **3.1**: `progress_test.go` unit tests for `buildState`. Lifecycle scenario tests for failure skipping only dependents, saved progress, no recreation of a succeeded resource, and statuses. Root `config_validate_test.go` covers save on provider failure. A read failure is saved as `failed`.
- **3.2**: rebuild scenario tests whose `failed`/`destroy_failed` starting state is produced by real failing applies. `TestDestroyLifecycle` is replaced.
- **4.1**: `validate_test.go` and `computed_test.go`, covering top-level and nested computed fields, key/position/map pairing, and pointer blocks. A `Config`-level validation test.
- **4.2**: `configured_check_test.go`, lifecycle scenarios for carry-over and warnings (with a recording logger), and the example plugin's second-apply test.
- **4.3**: final run of every touched package; confirm no new `t.Skip`; report the pre-existing unrelated failures.

All scenario state is produced by real applies (a first apply writes the state; later applies check against it), never hand-written state files. Every test follows the conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, mocks from mockery v3.

## Project References

- Spec: `20260918165700-provider-lifecycle-read` (final, 2026-09-19).
- Knowledge: `architecture/ux-flow.md` (Config/Parser/State relationships; Apply validates first), `decisions/parser-refactor-analysis.md`.
- Prior plan: `20260918075104-validation-phase-before-walk` (introduced the validation stages).
- Draft guide: `docs/plugin-developer-guide.md`.
- DAG walker source: `$(go env GOMODCACHE)/github.com/jumppad-labs/dag@v0.0.0-20220518035006-a7e85ada93c5/walk.go`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Phases 2.1 and 4.2 are High. All others are Low or Medium. Keep `internal/parser/parse_test.go` (1800+ lines) out of agent context except for the ranges cited. New scenario tests go in the new `lifecycle_test.go`.

## Migration Notes

- **Breaking for plugin authors:** rename `Refresh(ctx, r)` to `Read(ctx, old, new)`, and return `plugins.ErrNotFound` for missing resources. Embed `plugins.DefaultChanged[T]` unless custom change detection is needed. Tag provider-owned fields `xcl:"computed"` and make them `hcl:",optional"`. Stop writing configured fields in Create, Read or Update. External plugins must be rebuilt against the new proto.
- **State:** state files written by earlier versions don't have to load (spec constraint). A previous entry with an unrecognised or empty status is rebuilt, i.e. destroyed then created.
- **Tooling:** regenerate the proto with `make protos` (local protoc-gen-go v1.36.11 vs v1.36.10 in the headers is accepted). Regenerate mocks with mockery v3; the Makefile's `install-mockery` installs v2 and shouldn't be used.

## Performance Considerations

- Each existing resource now costs a `Read` plus `Changed` provider call per apply, where the current code creates it again. For real providers this is far cheaper than recreating.
- `DefaultChanged` and the configured-value check each do JSON marshal/unmarshal and reflect comparison per resource per call. That is negligible at config scale.
- `applyProgress` takes one mutex per resource outcome. The walker's parallelism is unaffected.
- `state.FindResource` is a linear scan (`state/state.go:212-231`), so a previous-state lookup per resource is O(n²) overall. That is acceptable at config scale and unchanged from what `State` already does elsewhere. Don't add an index in this plan.
