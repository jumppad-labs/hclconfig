---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Context: 20260919152915-destroy-cycle

## Current State Analysis

- `Config.Destroy` (`config.go:136-142`) is a stub that returns nil, so a caller is told everything was torn down when nothing was.
- `Parser.Apply` (`internal/parser/parser.go:205-240`) only ever walks the configured resources. A resource that is in the previous state but no longer in the configuration is left out of the new state and never passed to its provider's Destroy, so the real resource is orphaned. `progress.buildState` (`internal/parser/progress.go:45-77`) also only iterates current resources.
- `destroyWalkCallback` (`internal/parser/callbacks.go:183-260`) calls the provider's Destroy, fires destroy events and skips builtin and registered types, but nothing calls it except one unit test (`internal/parser/registered_types_test.go:403-427`).
- `buildDestroyDAG` (`internal/parser/dag.go:101-167`) matches saved `depends_on` strings exactly against IDs. Those strings are unresolved, module-relative references with attribute paths (`internal/parser/parse_test.go:161-172`). Module expansion and the "resource depends on its module" edge exist only while the create graph is built (`internal/parser/util.go:529-575`). The saved state therefore has no usable parent information.
- The only Destroy that runs today is in `resourceLifecycle.rebuild` (`internal/parser/lifecycle.go:233-259`), for resources saved as `failed`/`destroy_failed`.
- State is saved once per apply by `Config.Apply` (`config.go:127-131`), never per resource.
- `FileStateStore.Load` (`state/file_state_store.go:32-107`) silently skips entries whose type the registry can't create.
- `dag.Walker` (`internal/dag/walk.go:36`) supports `Reverse`, and skips vertices upstream of a failed one (`walk.go:375-398`).
- Both examples (`example/configonly/main.go`, `example/plugin/main.go`) apply without a state store and never destroy. Both example providers already implement Destroy with debug logging.

## Per-Phase Technical Notes

All paths are in the single registered repo `xclconfig` (root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).

### Phase 1.1: Record parents in the saved state and refuse unknown types on load

**File changes**
- `types/resource.go:5-47` — add `Parents []string \`json:"parents,omitempty"\`` to `Meta`, after `Links`, with a comment saying it is internal and holds resolved dependency IDs. Keep the `xcl` tag absent, so it can't be set from HCL, like `Links`/`Status`.
- `internal/parser/dag.go:44-97` (`buildCreateDAG`) — after `deps, err := getResourceDependencies(...)` (`:76`), collect the IDs of the non-nil deps (`types.GetMeta(d).ID`), sort them (`slices.Sort`), and assign them to `resourceMeta.Parents`. Nil entries come from unresolved refs (`util.go:555-557`). Also skip nil in the `graph.Connect` loop (`:86-88`), which today can connect a nil vertex. Set `Parents` to nil when there are none, so `omitempty` drops the field.
- `state/errors.go` — add `UnknownTypesError{Types []string}` with an `Error()` listing the types, comma-separated, and advising the user to register the types/plugins.
- `state/file_state_store.go:80-84` (`Load`) — when `fs.registry.CreateResource` fails, collect the type (unique) instead of `continue`. After the loop, if any were collected, return `nil, UnknownTypesError{Types: sorted}`. Other skips (malformed JSON, missing meta/type/name) are unchanged.
- Check that nothing else relies on the silent skip: grep the tests for states saved with a plugin that is later not registered (e.g. `state/file_state_store_test.go`, `internal/parser/*_test.go`). Adjust any such test to register the type.

**Tests**
- `internal/parser/lifecycle_test.go` (or a new `internal/parser/parents_test.go`) — using `setupLifecycle` + `applyAndSave` with the `dependent` fixture: `TestApplyRecordsParentsOfEachResource` (third → [second], second → [first], first and independent → none, output `first_name` → [network.first]).
- `TestApplyRecordsModuleAsParentOfItsResources` and `TestApplyRecordsEveryModuleResourceForModuleReference`, using the existing module fixtures (`internal/test_fixtures/config/lifecycle/nested` or `registered/basic` with `module/db.xcl`). Assert on the loaded saved state.
- `TestReapplyWithoutChangesDestroysNothingAndKeepsParents` — the no-regression metric (it becomes meaningful after 2.1, but is written here). Apply twice with the same fixture, assert no `destroy` in `TestPlugin.Calls`, and that the saved IDs and parents are equal.
- `state/file_state_store_test.go` — `TestLoadFailsWhenStateHoldsUnknownType`. A hand-written state file is allowed here, because the test is about the state format. The registry has no plugin for `postgres`, and the test asserts `errors.As(err, &state.UnknownTypesError{})` with `Types == ["postgres"]`. `TestLoadReturnsResourcesOfKnownTypes` is the positive case (the existing `TestLoadStateContainsResources` may already cover it).

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution.

### Phase 1.2: Destroy resources children-first from the saved state

**File changes**
- `internal/parser/dag.go:34-42,99-167` — keep `DoYouLikeDags` and its comment verbatim. Replace `buildDestroyDAG(toDestroy []any)`:
  - add a `destroy_root` vertex and every target;
  - index the targets by ID;
  - for each target, for each ID in `Meta.Parents` that is in the index, `graph.Connect(dagpkg.BasicEdge(parent, target))` (parent → child, like create);
  - if a target has no parent in the set, `graph.Connect(dagpkg.BasicEdge(root, target))`;
  - with an empty target list, return an empty graph.
  Update the comment: "same shape as the create graph, walked with Reverse".
- `internal/parser/destroy.go` (new) — define `destroyer` (fields in the Data Structures section: `working *state.State`, `store state.StateStore`, `resolver`, `types`, `options`, `mu sync.Mutex`, plus `failed []string`). Methods:
  - `destroy(targets []any) error`:
    - return nil if there are no targets;
    - build the graph with `buildDestroyDAG(targets)`, then `TransitiveReduction()` and `Validate()`;
    - run a `dag.Walker{Callback: destroyWalkCallback(d), Reverse: true}` with `log.SetOutput(io.Discard)` (as in `parser.go:1036`), then `Update` and `Wait`;
    - collect the errors like `walk` does (`parser.go:1040-1044`, via `errwrap.Wrapper`) into an `errors.NewConfigError()`, so that the returned error names every failed resource.
  - `destroyed(r any) error`: under `mu`, `working.RemoveResource(r)`, then `save()`.
  - `failedToDestroy(r any) error`: under `mu`, set `meta.Status = types.StatusDestroyFailed`, then `save()`. The resource is already the saved copy, since it came from state.
  - `save()`: a no-op when `store == nil`, otherwise `store.Save(working)`, with the error wrapped as `unable to save state after destroying <id>: …`.
- `internal/parser/callbacks.go:181-260` — change `destroyWalkCallback` to `destroyWalkCallback(d *destroyer) func(v dag.Vertex) dag.Diagnostics`:
  - skip `TypeRoot`;
  - if `types.GetDisabled(r)` or `handledWithoutProvider(d.types, meta.Type)`: fire a `destroy` success event (0 duration, nil data), then `d.destroyed(r)`. A save error is appended to diags;
  - otherwise resolve the adapter. If it is nil, call `d.failedToDestroy` and return the error `no provider found for resource type`;
  - marshal the resource, fire start, call `adapter.Destroy(ctx, data, false)`, fire error or success. On error, call `d.failedToDestroy(r)` and return `errors.NewParserErrorFromResource(r, fmt.Sprintf("destroy failed for %s: %s", id, err))`. On success, call `d.destroyed(r)`, and append its error if the save failed, so the parents are not visited.
  - Stop setting `StatusDestroyed`.
- `internal/parser/parser.go` — add `func (p *Parser) Destroy(saved *state.State) (*state.State, error)`:
  - if `saved` is nil, use `state.NewState()`;
  - `working := state.NewState()`, then append every saved resource. AppendResource re-derives the ID, which is fine;
  - build `destroyer{working, p.stateStore, p.providerResolver, p.typeRegistry, &p.options}`;
  - `err := d.destroy(working.GetResources())`, taking a copy of the slice first, because RemoveResource mutates the backing slice;
  - return `working, err`.
- `internal/parser/parse_test.go:1268-1462` — delete the commented-out legacy destroy block and its TODO.
- `internal/parser/registered_types_test.go:403-427` — rewrite `TestDestroyWalkSkipsProviderForRegisteredType` for the new callback signature. Build a `destroyer` with a working state that holds the db and `store: nil`, then assert the resource was removed from `working` and the mock resolver was never called.

**Tests** (new `internal/parser/destroy_test.go`; the harness is `setupLifecycle`, the fixture `dependent`, where first ← second ← third is A ← B ← C and `network.independent` is D)
- Add a harness helper `destroyAll(t, h, onEvent) (*state.State, error)`: build a parser with `h.newParser`, load `h.store`, then call `p.Destroy(loaded)`.
- `TestDestroyCallsProvidersChildrenFirst` — calls filtered to `destroy` are third, second, first, and independent is anywhere. Assert with `requireBefore`.
- `TestDestroyRemovesEveryResourceFromSavedState` — `h.loadSaved` shows `ResourceCount() == 0`.
- `TestDestroyCallsEachProviderResourceExactlyOnce`.
- `TestDestroyFailureLeavesParentsAndDestroysUnrelated`: set `SetDestroyError(secondID, …)` after registration. Assert third and independent were destroyed, first got no call, the error contains secondID, and the saved IDs are exactly {second, first}, with second at `destroy_failed`.
- `TestDestroyRetriesFailedResource`: first destroy fails on second, then `ClearErrors`, then a second destroy calls second before first and leaves the state empty.
- `TestDestroySavesStateAfterEachResource`: wrap `h.store` in a small `recordingStore` test type (it implements `state.StateStore`, delegates, and appends `st.Bytes()` snapshots on every Save). Assert the number of snapshots equals the number of resources, and that snapshot k holds exactly the resources not yet destroyed.
- `TestInterruptedDestroyResumesFromSavedState`: write snapshot 1 (from a real run over a fresh apply, via a second harness or by re-applying) to the store file, run destroy again, and assert that the provider calls cover only the IDs in the snapshot.
- `TestDestroyNeverCallsProviderForProviderlessBlocks`: use `setupRegisteredTypes`, which has a registry with no plugin, and the `registered/basic` + `registered/disabled` fixtures (variables, outputs, a module, a disabled block, registered types). Apply and save, then destroy with a `mocks.NewMockProviderResolver(t)` that has no expectations. Assert the state is empty.
- `TestDestroyOrphansNothingWhenAResourceFails` (metric 1, destroy): `created := TestPlugin.CreatedResources()`, `destroyedOK` = the destroy calls minus the failed IDs. Assert `created − destroyedOK ⊆ saved IDs`.
- `TestDestroyNeverDestroysParentOfFailedChild` (metric 2, destroy): for each saved resource with `destroy_failed`, none of its `Meta.Parents` appears in the destroy calls.
- `TestDestroyWithEmptyStateCallsNoProvider` (parser level).

**Complexity**: High
**Token estimate**: ~60k tokens
**Agent strategy**: Parallel analysis, sequential integration. One agent writes `destroy.go` + the callback + the graph builder. After that compiles, a second writes `destroy_test.go`. Integrate and run `go test ./internal/parser/...` sequentially.

### Phase 1.3: Make Config.Destroy real

**File changes**
- `config.go:136-142` — implement `Destroy()`:
  - `saved := c.currentState`;
  - if `c.stateStore != nil`: if `!c.stateStore.Exists()`, return nil, and never Save. Otherwise `loaded, err := c.stateStore.Load()`. Return `fmt.Errorf("failed to load state: %w", err)` on error, including `UnknownTypesError`. Then `saved = loaded`. If `saved` is nil or `ResourceCount() == 0`, return nil;
  - build the parser exactly like Apply (`config.go:109-114`, without Variables);
  - `remaining, err := p.Destroy(saved)`, then `c.currentState = remaining`, then return err;
  - update the doc comment: needs no configuration; saves after each resource; failures are kept as `destroy_failed` and named in the error.
- `events.go:194-198` — the `EventHandler` doc says "during Apply". Extend it to "during Apply and Destroy".
- Check that `FileStateStore` is created by `NewFileStateStore` with an empty file on construction (`state/file_state_store.go:16-29`). "Creates no saved state" is therefore asserted against a `MockStateStore` whose `Exists` is false and on which `Save` is never called, and, for the file store, as "the file content is unchanged".

**Tests** (new `config_destroy_test.go` in package `xcl`)
- Add a helper `setupDestroyConfig(t) (*Config, *parser.TestPlugin, *state.FileStateStore, string)`: set a temp HOME, create `registry.NewPluginRegistry(logger.NewTestLogger(t))`, `RegisterPlugin(&parser.TestPlugin{})`, and `state.NewFileStateStore(tmp/state.json, reg)`, then `NewConfig(WithPluginRegistry, WithStateStore, WithEventHandler(recorder.handle))`. Copy the fixture directory into a temp dir, so the test can delete the configuration files.
- `TestConfigDestroyDestroysEverythingAndEmptiesState` (AC "full destroy").
- `TestConfigDestroyNeedsNoConfiguration`: apply, `os.RemoveAll(configDir)`, destroy succeeds, and every created resource was destroyed.
- `TestConfigDestroyOrdersFromSavedStateWithoutConfiguration`: configuration deleted, order third, second, first (AC "order comes from saved state").
- `TestConfigDestroyWithNothingSavedSucceeds`: use `setupConfig` (mock store) with `Exists` false. Assert no error, `Save` not called, no provider calls.
- `TestConfigDestroyReportsStartThenSuccess`, `TestConfigDestroyReportsStartThenErrorWhenDestroyFails` and `TestConfigDestroyReportsOnlySuccessForVariable`, using `eventRecorder` (`config_events_test.go:18-43`).
- `TestConfigDestroyReturnsErrorNamingFailedResource`.
- `TestConfigDestroyFailsWhenSavedStateHasUnknownType`: apply with the plugin, then build a new Config whose registry has no plugin over the same state file. Destroy returns `UnknownTypesError` and the state file is unchanged.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential execution. It depends on 1.2.

### Phase 2.1: Destroy removed resources first during Apply, and reject empty configurations

**File changes**
- `internal/parser/parser.go:205-240` (`Parser.Apply`) — after `parseAndValidate`:
  1. If `currentState.ResourceCount() == 0`, return `nil, ErrEmptyConfiguration`. A nil state means Config.Apply saves nothing (`config.go:119-121`).
  2. `removed` = the resources in `previousState.GetResources()` whose ID is not found by `currentState.FindResource`.
  3. If `len(removed) > 0`:
     - `working := state.NewState()` with every previous resource;
     - `d := &destroyer{working, p.stateStore, …}`;
     - `if err := d.destroy(removed); err != nil { ce.AppendError(err); return working, ce }`. The working state holds previous minus destroyed, with the failures marked `destroy_failed`. Config.Apply saves it (`config.go:127-131`);
     - `previousState = working`.
  4. Continue with the existing walk unchanged.
  Update the Apply doc comment (steps list and the failure contract).
- `internal/parser/errors.go` (new) or the top of `parser.go` — `var ErrEmptyConfiguration = errors.New(...)`. Note the naming clash: the package imports `github.com/jumppad-labs/xcl/errors` as `errors`, so use `stderrors "errors"` or put it in a file without that import.
- `config.go:98-102` — the Apply doc comment mentions removal first, the empty-configuration error and destroy_failed retry.
- Fixtures (new, under `internal/test_fixtures/config/lifecycle/removal/`):
  - `before/main.xcl`: `network "x"`, `network "y"`, `network "p"`, `container "q"` referencing `resource.network.p.meta.name`;
  - `without_x/main.xcl`: y, p, q;
  - `without_x_with_z/main.xcl`: y, p, q, `network "z"`;
  - `without_pq/main.xcl`: x, y.
  - `internal/test_fixtures/config/empty/empty.xcl` contains only a comment. `findXclFiles` then finds a file with no blocks, and git keeps the directory.
- Check `lifecycle.run` (`lifecycle.go:100-105`): a resource saved as `destroy_failed` that is still in the configuration is rebuilt, which is unchanged behaviour. A removed resource saved as `destroy_failed` is now retried by the removal step, because it is in previous but not in current, which is what the spec wants.

**Tests** (new `internal/parser/removal_test.go`, harness `setupLifecycle`)
- `TestApplyDestroysRemovedResource`: apply `before`, then apply `without_x`. Destroy is called for x, x is absent from the saved state, y's saved entry is unchanged, and there are no create/update calls for y.
- `TestApplyDestroysRemovedBeforeCreatingNew`: `before`, then `without_x_with_z`. `requireBefore("destroy x", "create z")`.
- `TestApplyDestroysRemovedChildBeforeRemovedParent`: `before`, then `without_pq`. q is destroyed before p.
- `TestApplyStopsWhenRemovalFails`: `SetDestroyError(x)`, then apply `without_x_with_z` fails with an error containing x's ID. There is no `create z` call, and x is saved as `destroy_failed`.
- `TestApplyRetriesFailedRemovalBeforeCreating`: continuing from the previous state, `ClearErrors`, then re-apply `without_x_with_z`. Destroy x comes before create z, and x is gone.
- `TestApplyRejectsEmptyConfiguration`: apply `before`, then apply `empty`. `errors.Is(err, ErrEmptyConfiguration)`, no provider calls after the first apply, and the saved file bytes are unchanged. Check this at Config level too, in `config_destroy_test.go` or `config_validate_test.go`: `TestApplyRejectsEmptyConfigurationAndChangesNothing`.
- `TestRemovalOrphansNothingWhenADestroyFails` (metric 1, removal) and `TestRemovalNeverDestroysParentOfFailedChild` (metric 2, removal), with `SetDestroyError(q)`: p gets no destroy call.
- `TestReapplyWithoutChangesDestroysNothingAndKeepsParents` from 1.1 now covers the no-regression metric through the removal path.
- The existing `registered_types_test.go:275-291` (`TestApplyAfterRemovingRegisteredBlockSucceedsWithoutProvider`) should still pass. Extend it to assert that the removed block is gone from the state and that no provider was called.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential execution. It depends on 1.2.

### Phase 3.1: Examples apply and then destroy

**File changes**
- `example/configonly/main.go:39-91` — change `run(out, log, dir)` to `run(out io.Writer, log logger.Logger, dir, statePath string) ([]any, error)`:
  - `store, err := state.NewFileStateStore(statePath, r)`, then add `xcl.WithStateStore(store)`;
  - after printing, capture `applied := c.GetResources()`, then call `c.Destroy()`. Print `## Destroyed` and the remaining count, which is 0, and return `applied`.
  - `main` makes the state path with `os.MkdirTemp("", "xcl-example")` plus `state.json`, and removes the directory on exit.
- `example/configonly/main_test.go` — pass `filepath.Join(t.TempDir(), "state.json")` at every `run` call site. Add:
  - `TestRunDestroysEverythingItApplied`: load the state file with a registry holding the same registered types, and assert `ResourceCount() == 0`;
  - `TestDestroyIsLoggedAsSuccessAtDebug`: `recordingLogger.events("debug", "destroy")` covers all 10 IDs with the phase success;
  - keep the existing "nothing above debug" test passing, since it now includes destroy.
- `example/plugin/main.go:55-118` — same change: add a `statePath` parameter, a file store and a destroy after the printing. The deferred plugin-host stop must run after Destroy, which it does already because it is deferred at `:59-63`. `main` uses a temp dir.
- `example/plugin/main_test.go` — update the `run` calls. Add:
  - `TestRunDestroysEverythingItApplied`: the state file loads empty through a registry with both plugins. Alternatively, read the raw file and assert `[]`, which avoids reloading the plugins;
  - `TestProvidersLogDestroyForEveryResource`: postgres (3) and app (1) `destroy` debug messages via `withMessage`;
  - `TestDestroyEventPhases`: provider types have start+success, builtins success only.
- `example/configonly/Makefile`, `example/plugin/Makefile` — no change. `go run` uses `main`'s temp dir.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2 parallel agents, one per example, since the two are independent.

### Phase 3.2: Documentation describes destroy

**File changes**
- `docs/parser-lifecycle.md:113-121` (`### Destroy`) — rewrite it:
  - the destroyer;
  - the reverse walk over `Meta.Parents`;
  - per-resource save;
  - `destroy_failed` retry;
  - disabled/builtin/registered skip.
  Update `:6-40` (the Apply steps: empty-configuration check and removal phase), `:194-215` (event sequences: "Removed resource" now fires destroy start/success-or-error; builtins fire destroy success on Destroy) and `:220-250`.
- `docs/state.md` — the statuses table (`destroyed` is never saved; `destroy_failed` is retried by Destroy or by the next Apply). Also cover `meta.parents` in the shape section, `Load` failing with `UnknownTypesError`, and a new "State during a destroy" section.
- `docs/overview.md:14,83-89,114-118` — remove the stub note and describe Destroy and removal.
- `docs/plugin-developer-guide.md:98-131,212-224` — Destroy is called on Config.Destroy, on removal and on rebuild, always with the saved copy. `force` is false.
- `docs/README.md:17-20` — the Destroy wording.
- `README.md` around `:110-170` — show `NewFileStateStore` + `WithStateStore`, then `Apply`, then `Destroy`, and mention that destroy events go through `WithEventHandler`.
- `CHANGELOG.md` — the implement workflow adds the entry for `20260919152915-destroy-cycle`, in the same style as the existing entries. Call out the breaking changes (Load errors on unknown types; an empty configuration errors on Apply).

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

- **Phase 1.1**: unit/scenario tests for recorded parents (chain, module membership, module-wide reference) on real saved state. The unchanged re-apply regression test. A Load test for unknown types (hand-written state file allowed, because it is about the format).
- **Phase 1.2**: parser-level destroy scenarios on the `dependent` fixture through `setupLifecycle`: order, failure isolation, retry, per-save snapshots, resume from a real mid-way snapshot, provider-less blocks via `setupRegisteredTypes` with a mock resolver that expects no calls, plus metric tests 1 and 2 for destroy.
- **Phase 1.3**: Config-level tests with a real file store and a copied fixture directory: full teardown, destroy after the configuration is deleted (including order), nothing saved (mock store: `Save` never called), the three event sequences via `eventRecorder`, the error naming the failed resource, and unknown types.
- **Phase 2.1**: removal scenarios on the new `lifecycle/removal/*` fixtures: X removed, X before Z, Q before P, a failed removal stops the apply, retry, the empty configuration rejected (parser and Config level), plus metric tests 1 and 2 for removal. The existing registered-removal test is extended.
- **Phase 3.1**: both examples' tests assert an empty saved state after `run`, destroy events at debug, and nothing above debug. The plugin example also asserts that each provider logged `destroy` for its resources.
- **Phase 3.2**: no tests. A doc grep for "stub" and "not destroyed" wording.
- Across all phases: testify `require`, no table tests, separate positive and negative functions, state produced by real applies, and the full `go test ./...` plus `go vet ./...` must pass at the end of each phase.

Success metrics mapping:
- *Nothing is orphaned*: behavioural tests `TestDestroyOrphansNothingWhenAResourceFails` (1.2) and `TestRemovalOrphansNothingWhenADestroyFails` (2.1).
- *A parent is never destroyed after a child fails*: behavioural tests `TestDestroyNeverDestroysParentOfFailedChild` (1.2) and `TestRemovalNeverDestroysParentOfFailedChild` (2.1).
- *No regression*: behavioural. The full suite and vet, plus `TestReapplyWithoutChangesDestroysNothingAndKeepsParents` (1.1, still passing after 2.1).

## Project References

- Spec: `20260919152915-destroy-cycle` (read with `spektacular spec file read 20260919152915-destroy-cycle.md`).
- Prior plans: `20260918165700-provider-lifecycle-read` (rebuild, `callProvider`, partial state on error) and `20260919120639-config-only-types-and-examples` (`TypeRegistry`, `handledWithoutProvider`, the examples).
- Knowledge conventions: `conventions/test-state-from-real-apply.md`, `conventions/testing-and-mocking.md`, `conventions/never-modify-dependencies.md` (repo tier, store `xclconfig`).
- Docs to update: `docs/state.md`, `docs/parser-lifecycle.md`, `docs/overview.md`, `docs/plugin-developer-guide.md`, `docs/README.md`, `README.md`.
- Key code: `config.go`, `internal/parser/{parser,dag,callbacks,lifecycle,progress,util}.go`, `internal/dag/walk.go`, `state/{state,file_state_store,errors}.go`, `types/resource.go`, `internal/parser/test_plugin.go`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Phase 1.2 is the only High phase. Phases 1.1, 1.2, 1.3 and 2.1 must run in order. Phases 3.1 and 3.2 can run in parallel after 2.1.

## Migration Notes

- **Saved state format**: `meta.parents` is added (omitempty). State written before this change loads fine, but its resources have no parents until they are re-applied. Destroying such state ignores ordering (accepted in the spec). Re-applying once records parents.
- **Breaking**: `FileStateStore.Load` now returns `state.UnknownTypesError` instead of silently dropping entries of unregistered types. This affects both Apply and Destroy. Applications must register every type/plugin their saved state uses before loading it.
- **Breaking**: applying a configuration with no blocks returns `parser.ErrEmptyConfiguration`. Previously it silently dropped everything from the saved state.
- **Behaviour change**: removed blocks are now destroyed through their provider on the next apply.
- `destroyed` status is no longer set in saved state, because destroyed resources are removed.

## Performance Considerations

- A destroy saves the whole state once per resource, so there are N file rewrites for N resources. This is fine at config scale, where state is one configuration's worth of resources, and it is required for resumability. Saves are serialised by the destroyer's mutex, so parallel branches wait on each other only for the save itself, not for provider calls.
- `State.FindResource`/`RemoveResource` are linear scans, so a removal is O(N²) at worst. That is negligible at this scale.
- An apply that removes nothing does no extra work beyond one set difference over the previous state.
