---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Research: 20260918165700-provider-lifecycle-read

## Alternatives considered and rejected

- **`Read(ctx, new)` only (parser copies identity from old)** — rejected by the user in the spec: config alone can't locate resources whose identity is assigned at create. The spec fixes `Read(ctx, old, new)`.
- **Read refreshes `old` instead of `new`** — rejected in spec discussion: observed fields have no desired value in config, so a flat diff of refreshed-old vs raw-new is meaningless.
- **Keep `Refresh` alongside `Read` / compatibility shim** — rejected by spec constraint ("No, break it", v2 branch).
- **`computed` as an hcl tag option (`hcl:"id,optional,computed"`)** — rejected: gohcl panics on unknown hcl field kinds (`gohcl/schema.go:143-180`, hcl v2.21.0; kind is everything after the first comma → `panic("invalid hcl field tag kind ...")` at `:178`). `gohcl.DecodeBody` runs on every resource (`internal/parser/callbacks.go:106`).
- **Carry `ErrNotFound` across gRPC with `status.Error(codes.NotFound)`** — rejected: every server method returns a nil gRPC error and an in-band `string error` field (`plugins/grpc_server.go:140-143, 207-212`); transport errors are the only thing on the gRPC error path (`plugins/grpc_plugin_host.go:151-153`). Mixing a domain signal into it breaks the convention.
- **Rely on string-matching the error message on the host** — rejected: fragile; `fmt.Errorf(resp.Error)` also mangles `%` (vet flags `grpc_plugin_host.go:105..191`).
- **Get per-vertex results from the DAG walker after the walk** — not possible: `diagsMap`/`upstreamFailed` are unexported in `github.com/jumppad-labs/dag@v0.0.0-20220518035006-a7e85ada93c5/walk.go` (no accessors). Must record progress in the callback.
- **Keep `previousParsed *parsed` as the lookup type** — rejected: previous state is a `*state.State` (typed pointers from `FileStateStore.Load`), and `state.FindResource(id)` already works (`state/state.go:81`, matches Module/Type/Name). Converting to `*parsed` adds nothing; the dead `plugins.go:callPluginLifecycle` already used `previousState.FindResource` (`internal/parser/plugins.go:50`).
- **Reuse `internal/parser/plugins.go:callPluginLifecycle`** — rejected: dead code (no callers), spec says remove it.
- **Detect "set by reference" via `processExpr`** — weaker: `processExpr` (`internal/parser/exp.go:22-143`) misses `ParenthesesExpr`, `IndexExpr`, `RelativeTraversalExpr`, `ForExpr`. `hclsyntax.Expression.Variables()` returns every traversal in any expression shape; filter `RootName() == "resource"` (or module outputs).

## Chosen approach — evidence

- Previous state already loaded: `internal/parser/parser.go:232-243`; passed to `walk` (`:200`, `:927`) but discarded (`:946-950`, `previousParsed = nil` TODO).
- Lifecycle to rewrite: `internal/parser/callbacks.go:256-391` (`callProviderLifecycle`). Refresh result unmarshalled into `r` (`:340`) but `Changed` gets pre-refresh `resourceJSON` (`:287, :350`); refresh error swallowed (`:334-336`).
- DAG walker semantics (jumppad-labs/dag fork, `walk.go`): dependents of a failed vertex are skipped with "upstream dependencies failed" (`waitDeps` `:402-439`, `walkVertex` `:376-384`); independent vertices still run in parallel (one goroutine per vertex, `:300`); `Wait` returns only real failures (`:112-129`). So "failure skips only dependents" is already the walker's behaviour — callback must record outcomes itself, with a mutex (parallel).
- Current/parsed resources share pointers (`parser.go:323-327`; `dag.go:53-55`), so mutating `r` in the callback updates `currentState`.
- Failed apply loses state: `Parser.Apply` returns `nil, ce` on walk errors (`parser.go:201-206`); `Config.Apply` only saves on success (`config.go:106-119`).
- Plugin types are rebuilt via `reflect.StructOf` from a schema that carries the raw tag string (`internal/schema/serialize.go:53`, `deserialize.go:63-130`), so a new `xcl:"computed"` tag key reaches the host with no schema/proto change. Mapped types (`ResourceBase`, `Meta`, `cty.Value`) use host definitions (`plugins/registry/plugin_registry.go:48-52`).
- Existing precedent for extra tag keys: `default:"..."` (creasty/defaults, `callbacks.go:103`), `mapstructure`.
- Validation slot: `internal/parser/validate.go:25-47` stages; `validateStructure` is empty (`:145-147`); errors via `errors.NewParserError(file,line,col,msg)` (`errors/parser_error.go:72`); bodies at `p.parsedResources.bodies[id]` give attribute `SrcRange`/`NameRange`. hcl name → field mapping `propertyNames`/`hclTagName` (`internal/parser/types.go:34-90`) flattens embedded structs.
- gRPC error convention: in-band `string error` in each response (`plugins/plugin.proto:49..104`); host rebuilds with `fmt.Errorf(resp.Error)`. Adding `bool not_found` to `ReadResponse` and mapping it to `plugins.ErrNotFound` on the host is a one-server/one-client change. In-process `TypedProviderAdapter` passes errors through unchanged (`plugins/adapter.go:153`), so `errors.Is` works directly.
- Remote `Changed` runs inside the plugin process (`grpc_server.go:182` → `plugin.go:203` → `adapter.go:205`), so an embedded `DefaultChanged[T]` promoted method works in- and out-of-process.
- Logger reachable in the walk: `options.Logger` (`parser.go:65`, passed as `&p.options` to `walkCallback`), `Warn(msg, kv...)` (`logger/logger.go:4-9`).

## Files examined

- `internal/parser/callbacks.go:31-170` — walkCallback: decode, module vars, callProviderLifecycle, output conversion.
- `internal/parser/callbacks.go:174-254` — destroyWalkCallback: never called; uses both `destroyed_failed` and `destroy_failed`.
- `internal/parser/callbacks.go:256-391` — callProviderLifecycle: create/refresh/changed/update; statuses; refresh error swallowed.
- `internal/parser/parser.go:187-209` — Apply returns nil state on walk error.
- `internal/parser/parser.go:227-330` — parseAndValidate loads previous state, validates, moves parsed into currentState.
- `internal/parser/parser.go:927-965` — walk; previousParsed nil TODO; errors extracted via errwrap.
- `internal/parser/plugins.go:15-133` — dead `callPluginLifecycle` (remove).
- `internal/parser/events.go:6-30` — ParserEvent; Operation comment lists "refresh".
- `internal/parser/validate.go:25-147` — validation stages; empty structure stage.
- `internal/parser/types.go:34-99` — propertyNames/hclTagName/dereference.
- `internal/parser/exp.go:22-178` — processExpr; limited expression coverage.
- `internal/parser/test_plugin.go:14-321` — TestPlugin records calls by ID; per-ID error maps; Changed always false; no not-found or changed-result config; Init resets maps.
- `internal/parser/parse_test.go:43-83` — setupParser (bare MockStateStore with Exists/Load/Save stubs).
- `internal/parser/parse_test.go:989-1131` — event tests; allowed ops list at `:1032` includes "refresh"; TestParserRefreshEventErrorCallback fails today.
- `internal/parser/parse_test.go:1201-1248` — TestDestroyLifecycle: panics (no Exists stub) and expects a destroy walk that is a non-goal.
- `internal/parser/parse_test.go:1252-1284` — requireEvent / requireBefore helpers.
- `internal/parser/mocks/mock_provider_resolver.go` — mockery mock.
- `plugins/provider.go:16-85` — ResourceProvider[T] with Refresh(ctx, T).
- `plugins/adapter.go:107-291` — ProviderAdapter + TypedProviderAdapter (Refresh nil handling at `:228`).
- `plugins/plugin.go:50,106-121,177-204` — PluginEntityProvider/PluginBase Refresh/Changed; RegisterType generates schema.
- `plugins/plugin_host.go:21-28`, `plugins/direct_plugin_host.go:51,61` — host Refresh/Changed (test-only callers).
- `plugins/grpc_plugin_host.go:145-195,262-282` — gRPC wrapper; `fmt.Errorf(resp.Error)`.
- `plugins/grpc_resource_adapter.go:40-50` — host-side adapter; drops ctx for Changed.
- `plugins/grpc_server.go:121-187,207-212` — server Refresh/Changed; errorToString.
- `plugins/plugin.proto:11-13,73-105` — Refresh rpc/messages; ChangedRequest already has old/new.
- `plugins/proto/*.pb.go` — generated (protoc v3.21.12, protoc-gen-go v1.36.10, protoc-gen-go-grpc v1.5.1).
- `plugins/datasource.go:10-33` — DataSourceProvider.Refresh(ctx); unused, separate; untouched.
- `plugins/mocks/mock_provider_adapter.go` — regenerate after ProviderAdapter change.
- `plugins/testing/helpers.go:125-185` — TestChanged/TestCRUDOperations call host.Changed(data,data).
- `plugins/example/pkg/person/provider.go` — Create/Refresh/Update all rewrite `Description` (config field); custom Changed.
- `plugins/example/pkg/person/resource.go` — Person fields.
- `plugins/example/e2e_test.go:88-187` — in-process & external tests; TestExternalPluginRefresh.
- `plugins/registry/plugin_registry.go:35-83,210-231` — CreateResource from schema; GetProviderForResource.
- `internal/schema/serialize.go:27-53`, `deserialize.go:52-130,244-268` — raw tags preserved.
- `state/state.go:15-231` — flat slice; AppendResource/RemoveResource/FindResource; no replace helper.
- `state/file_state_store.go:32-144` — Load needs registry; silently skips unknown types; Save non-atomic.
- `config.go:69-130` — Apply saves only on success; Destroy is a TODO.
- `config_validate_test.go:24-297` — setupConfig; Apply save/no-save assertions.
- `types/resource.go:5-58` — Meta.Status comment lists pending/created/failed.
- `logger/logger.go:4-9`, `logger/pretty_printer.go:164-168,865-872` — status colours (pending/created/failed).
- `docs/plugin-developer-guide.md` — rough draft of agreed contract with "gaps" list.
- `docs/parser-lifecycle.md`, `docs/state.md:15-17`, `docs/overview.md:96`, `docs/plugins.md:35,57,89`, `plugins/example/README.md:116-122`, `docs/README.md` — reference Refresh / old statuses.
- `Makefile:1-19`, `.mockery.yml` — `make protos`, `make mocks` (mockery v3.5.5 installed; Makefile installs v2 — use installed v3).

## External references

- hcl v2.21.0 `gohcl/schema.go:143-180` — why `computed` can't be an hcl option.
- `github.com/jumppad-labs/dag` walk.go — failure propagation semantics (dependents skipped, independents continue, no per-vertex result API).
- hashicorp/go-plugin gRPC — status errors would pass through, but project convention is in-band errors.

## Prior plans / specs consulted

- Spec `20260918165700-provider-lifecycle-read` (the source of truth for this plan).
- Knowledge `decisions/parser-refactor-analysis.md` and `architecture/ux-flow.md` — parser/Config/State relationships; Apply runs validation first, nothing is acted on if invalid. No conflicting guidance.
- Prior plan `20260918075104-validation-phase-before-walk` — introduced the validation stages the computed check plugs into (not re-read; structure confirmed from code).

## Open assumptions

- Baseline `go test ./...` has pre-existing failures unrelated to this spec: `errors` (TestParserErrorHighlightsLine/NonErrorLineGrey), root (`TestReadResourceFromFileAtLocation`, `TestLineFromFileAtLocation` — missing `.hcl` fixtures), build failures in `./example` (vet), `internal/functions` (undefined setupParser in tests), `plugins/registry` (undefined newTestPluginSetup). Assumed out of scope for "Test suite passes" except where this work touches them; the implementer must report them, not silently fix or ignore.
- `TestDestroyLifecycle` tests a destroy walk that the spec lists as a non-goal; assumed it is removed/rewritten to the failed-rebuild destroy, not the removed-resource destroy.
- Previous-state resources whose type isn't registered are silently dropped by `FileStateStore.Load` — assumed acceptable (unchanged).
- A resource present in previous state but disabled/removed in config is carried over unchanged into saved state? — spec only requires carrying over *unreached previously existing* resources on failure. Assumed: on success, resources no longer in config are dropped from saved state as today (no destroy — non-goal).
- `module.x` output references count as "set by reference" for the warning exemption (anything whose expression contains a traversal rooted at `resource` or `module`).
- Builtin types (variable/output/module) get no provider calls and no status changes beyond what exists; they're excluded from failed-rebuild logic.

## Drafting assumptions

### Chosen direction: dedicated resourceLifecycle component + progress recorder (architecture)
- **Decision**: Option B — new `internal/parser/lifecycle.go` with `resourceLifecycle` (previous `*state.State`, ProviderResolver, options, mutex-protected `applyProgress`); walkCallback delegates; Parser.Apply returns partial state + error; Config.Apply saves non-nil state even on error. Provider contract: Read(old,new), ErrNotFound via proto `not_found` bool, embeddable DefaultChanged[T], typed status constants, `xcl:"computed"` tag.
- **Rationale**: one testable home for the five lifecycle paths; works with the dag fork's existing skip-dependents semantics; minimal wire change.
- **Rejected**: (A) grow callProviderLifecycle in place — >300-line function mixing paths, reflection and state assembly; (C) fork/extend the dag library to expose per-vertex results — heavier and still needs per-resource status; gRPC status codes for not-found — breaks in-band error convention.

### Only provider-call failures mark a resource `failed` (architecture)
- **Decision**: decode/context errors before any provider call leave the resource "unreached" (previous entry kept, or omitted if new); only Create/Read/Update/Changed errors mark `failed`, Destroy errors in a rebuild mark `destroy_failed`.
- **Rationale**: a resource that was never touched by a provider has nothing to destroy; marking it failed would trigger a pointless destroy next apply.
- **Rejected**: marking any walk error as failed.

### What copy is saved for failed / destroy_failed (architecture)
- **Decision**: `failed` saves the configured resource as it stood at the failing call (it already carries computed values copied from old + Read results); `destroy_failed` saves the old copy (it holds the identity needed to retry destroy).
- **Rationale**: the next apply's rebuild destroys using the saved copy, which must hold identity.
- **Rejected**: always saving the configured copy for destroy_failed — may lack identity if the resource was previously `failed` on create.

### DefaultChanged ignores all of ResourceBase (architecture)
- **Decision**: flat compare drops `meta`, `depends_on`, `disabled` JSON keys.
- **Rationale**: all three are xcl-owned metadata, not provider state; a depends_on edit should not force a provider Update.
- **Rejected**: ignoring only `meta` (spec wording) — would make a depends_on edit look like a resource change.

### Computed fields at any depth, list elements paired by `xcl:"key"` (walkthrough, user decision + default)
- **Decision**: `xcl:"computed"` is honoured at any depth: resource fields, embedded structs, nested blocks, pointer blocks, lists and maps of blocks. The user required it; jumppad's container carries `image.id` and `network[].assigned_address`/`name` over by hand today (`jumppad/pkg/config/resources/container/resource_container.go:121-146`, with fields at `:57-70`). To pair list elements between the saved and configured copies, an element type may tag identity fields `xcl:"key"` (matches jumppad pairing networks by `id`). Without a key, elements are paired by position. Maps are paired by map key. Unpaired elements are left alone.
- **Rationale**: position-only pairing moves computed values onto the wrong block when a user reorders blocks. Key-only would force every block type to declare a key. Key-with-positional-fallback covers both.
- **Rejected**: top-level only (the original draft; blocks the jumppad container/network pattern). Pairing always by position. Requiring a key on every block type.

### Keep destroyWalkCallback, fix its status strings (architecture)
- **Decision**: leave the unused destroy walk callback in place (the destroy walk is a non-goal) but switch it to status constants, removing `destroyed_failed`. Delete only `internal/parser/plugins.go`.
- **Rationale**: spec names the duplicate lifecycle code for removal, not the destroy walk; "one set of statuses" requires the typo gone.
- **Rejected**: deleting destroyWalkCallback — future destroy work would rebuild it.

### Conventions selected (architecture)
- **Decision**: apply Testing & Mocking, Patterns & Architecture, Go Code Style, Development Standards (structured logs), Dependencies. Drop Database & External Services and Project Structure.
- **Rationale**: no database/connection pooling here; work stays in existing packages, no new top-level dirs.
- **Rejected**: n/a.

### Pre-existing unrelated test failures (discovery)
- **Decision**: "Test suite passes" is scoped to packages this work touches (internal/parser, plugins/..., state, types, root config tests); pre-existing unrelated failures (errors pkg, missing .hcl fixtures in root utils tests, build failures in ./example, internal/functions, plugins/registry test files) are reported, not fixed.
- **Rationale**: they predate this spec and have nothing to do with the lifecycle; fixing them widens scope.
- **Rejected**: fixing them all in this plan — raise at walkthrough if the user wants them included.

### Reference exemption covers module traversals (discovery)
- **Decision**: a field is "set by reference" if its expression contains any traversal rooted at `resource` or `module`, found via hclsyntax `Expression.Variables()`.
- **Rationale**: module outputs are resource-derived values; `Variables()` covers all expression shapes, unlike processExpr.
- **Rejected**: reusing processExpr (misses Index/Parens/For/RelativeTraversal).

### Validation also rejects non-optional computed fields (components)
- **Decision**: a field tagged `xcl:"computed"` without `hcl:",optional"` is reported by validation as a type-definition error.
- **Rationale**: a required computed field can never be satisfied (users can't set it, gohcl demands it), so failing loudly at validate time beats a confusing decode error.
- **Rejected**: silently treating it as optional — can't, gohcl would still require it.

### Test plugin extended rather than replaced by mocks (components)
- **Decision**: extend `internal/parser/test_plugin.go` (not MockProviderAdapter) for most lifecycle scenario tests, because it runs through the real registry/adapter/JSON path; use MockProviderAdapter only where call ordering/arguments are easiest to assert with a mock.
- **Rationale**: the acceptance criteria are about real end-to-end apply behaviour, including JSON round-tripping and computed-tag reflection on schema-rebuilt types.
- **Rejected**: mocks only — would bypass TypedProviderAdapter and schema-rebuilt types, hiding the most likely bugs.

### Two-apply tests use a real FileStateStore (testing_approach)
- **Decision**: scenario tests persist between applies with `state.NewFileStateStore` in `t.TempDir()` and the real registry, not a MockStateStore returning the first result.
- **Rationale**: exercises the JSON save/load round-trip that previous state actually goes through (schema-rebuilt types, computed values, status), which is where the lifecycle reads its old copy.
- **Rejected**: mock store handing back the in-memory state — skips the serialization boundary and shares pointers between applies.

### TestDestroyLifecycle rewritten for failed rebuild (testing_approach)
- **Decision**: replace the existing removed-resource destroy test (panics today, tests a non-goal) with failed-rebuild destroy tests.
- **Rationale**: removed-resource destroy is an explicit non-goal; the spec requires "no lifecycle tests skipped" so it can't just be skipped.
- **Rejected**: keeping it and implementing the destroy walk — out of scope.

### Milestone ordering: contract first, then behaviour, then failure, then computed (milestones)
- **Decision**: M1 contract/wire/statuses/events → M2 previous-state lifecycle → M3 partial state on failure, then failed rebuild → M4 computed fields + guide.
- **Rationale**: M1 is a compile-level change every later milestone builds on; M2's lifecycle is where M3 records progress and M4 hooks carry-over/warnings; the guide is finalised last when the whole contract exists.
- **Rejected**: computed fields before the lifecycle (carry-over has nowhere to hook until M2); docs as their own milestone (docs follow the behaviour they describe).

### Example provider demonstrates ErrNotFound via a sentinel input (phases)
- **Decision**: the example person provider's `Read` returns `plugins.ErrNotFound` when `old.Email == "missing@example.com"`, so e2e tests can prove the sentinel crosses gRPC.
- **Rationale**: the example has no real backing store to lose a resource from; a documented sentinel is the simplest way to exercise the path end-to-end against the built binary.
- **Rejected**: a separate test-only plugin binary — more build plumbing for one assertion.

### Unrecognised previous status treated as failed (phases)
- **Decision**: a previous-state entry with any status other than created/updated/failed/destroy_failed (e.g. empty) takes the rebuild path.
- **Rationale**: unknown provenance → safest to destroy and recreate; old state files need not load per spec.
- **Rejected**: treating unknown as created — could skip rebuilding something half-made.

### Scenario state always comes from a real prior apply (walkthrough, user decision)
- **Decision**: no test hand-builds a state file. A first apply generates the state as the system would; later applies (with modified config or cleared/added errors) are checked against it. Rebuild tests (now Phase 3.2) start from state written by a real failing apply, so they follow save-on-failure (Phase 3.1).
- **Rationale**: user: test state can then never drift from what the system writes, and every scenario also exercises the full lifecycle.
- **Rejected**: seeding a `failed` state file directly so rebuild could land before save-on-failure (the original draft).

### Test fixture struct gains ProviderID/Observed fields (phases)
- **Decision**: add `ProviderID` (set at create) and `Observed` (set by Read for drift) to the test fixture `Network` (and container if needed); tag both `xcl:"computed"` in 4.1.
- **Rationale**: acceptance criteria need a create-time value in `old` and an observed value for drift; these are exactly computed fields.
- **Rejected**: reusing existing config fields for drift — Read writing a config field is what the contract forbids and would trigger the new warning.

### Example plugin apply test drives the parser directly (phases)
- **Decision**: the "second apply of unchanged config makes zero create/update" metric against the example plugin uses `internal/parser` with `OnParserEvent` from within the same module.
- **Rationale**: `Config` doesn't expose events (non-goal to add), and `internal` is importable within module `github.com/jumppad-labs/xcl`.
- **Rejected**: exposing OnParserEvent on Config — explicit non-goal.

### Proto regeneration with local toolchain (open_questions)
- **Decision**: regenerate with the installed toolchain (protoc 3.21.12, protoc-gen-go v1.36.11, protoc-gen-go-grpc 1.5.1); the one-patch-version header drift from v1.36.10 is accepted.
- **Rationale**: no behavioural difference between patch versions; go.mod's protobuf runtime is compatible.
- **Rejected**: pinning/installing v1.36.10 exactly — unnecessary churn.

## Rehydration cues

- `spektacular spec file read 20260918165700-provider-lifecycle-read.md`
- Re-read `internal/parser/callbacks.go:256-391`, `internal/parser/parser.go:187-330,927-965`, `plugins/adapter.go`, `plugins/provider.go`, `plugins/plugin.proto`, `internal/parser/test_plugin.go`, `docs/plugin-developer-guide.md`.
- DAG semantics: `$(go env GOMODCACHE)/github.com/jumppad-labs/dag@v0.0.0-20220518035006-a7e85ada93c5/walk.go:112-129,376-439`.
- `go test ./internal/parser/... ./plugins/... ./state/... ./types/... .` for the relevant suite; `make protos`, `make mocks`.
