# Context: 20260714080036-restore-module-support

## Current State Analysis

Module blocks are already recognized as a resource type by the parser's type system, but are silently skipped during Phase-1 file parsing: `internal/parser/parser.go:332-339` has `case resources.TypeModule: continue` in place of a real parse call, meaning no `*resources.Module` shell is ever created and no child resources from a module's source directory are ever discovered. Everything downstream of that point — the DAG builder (`internal/parser/dag.go`), the dependency-link extraction (`internal/parser/exp.go`, `internal/parser/util.go:493-568`), and the resource-addressing scheme (`internal/resources/fqrn.go`, `state/state.go`) — is already fully module-aware and requires no changes; it was already written and tested against module-shaped FQRNs and dependency edges during the broader v2 parser refactor, but has had nothing to operate on because Phase 1 produces no module shells.

Three specific places carry dead, commented-out logic that was drafted for module support during the refactor and never finished: `internal/parser/callbacks.go:75-90` (module-disabled propagation onto children, references an undefined variable `c`), `internal/parser/callbacks.go:114-138` (module variable override, references non-existent helper functions `getModuleVariables`/`processModuleVariables`), and `internal/parser/context.go:103-122` (an empty `if rMeta.Module != ""` stub for merging module-passed variables into a child's context, also referencing the non-existent `getModuleVariables`). This plan replaces all three with working logic that operates on data structures that already exist (`state.State`'s `FindModuleResources`/`FindRelativeResource`, `resources.Module`'s `SubContext` field), rather than inventing new mechanisms.

The two-phase parse-then-decode architecture (`.spektacular/knowledge/decisions/parser-refactor-analysis.md`) was deliberately introduced to fix interpolation-ordering bugs that the old (pre-refactor, commit `3cf707c`) single-pass `parseModule` could not solve correctly, since that old implementation decoded module bodies eagerly and inline. This plan's module-parsing addition must create shells only during Phase 1 (never calling `gohcl.DecodeBody`) and defer all decoding — including the module's own `source`/`variables`/`disabled` attributes — to the Phase-2 DAG walk, exactly as every other resource type already does.

One concrete bug was found and must be fixed as part of this work, not treated as a nice-to-have: `resources.Module.Variables` is currently typed `any`, but `gohcl.DecodeBody`'s attribute-decode path (`gohcl/decode.go:102-131` → `DecodeExpression` → `gocty.ImpliedType`) has no `case reflect.Interface` in its type switch (verified against the project's `jumppad-labs/go-cty` fork, `cty/gocty/type_implied.go:30-73`) and would error on any module block with a `variables = { ... }` attribute. This field must be retyped to `map[string]cty.Value` (or an equivalent `cty.Value`-based shape) as part of Phase 2.1.

## Per-Phase Technical Notes

### Phase 1.1: Parse module blocks into shells instead of skipping them

- `internal/parser/parser.go:332-339` — replace the `case resources.TypeModule: continue` stub with a call to a new `parseModule(file string, b *hclsyntax.Block, parentModule string) []error` function (defined near `parseResource`, e.g. after line 548).
- `parseModule` mirrors `parseResource` (`internal/parser/parser.go:364-548`) for the module block type: create the `*resources.Module` shell via `resources.DefaultResources().CreateResource(resources.TypeModule, name)` (or equivalent — check `p.createBuiltinResource`, `parser.go:797-818`, extend if `TypeModule` isn't already routed through it), validate exactly one label (mirroring the variable/output label check at parser.go:444-455), set `rtMeta.Module = parentModule`, `rtMeta.File`, `rtMeta.Line`, `rtMeta.Column` (mirroring parser.go:494-497), compute `rtMeta.ID` via `resources.FQRNFromResource` (mirroring parser.go:500-501), call `p.getUniqueResourceLinks(rt, b)` (parser.go:505, reused unchanged — already walks `b.Body.Attributes` including `source`/`variables`/`disabled` via `exp.go`'s existing `module`-root-aware traversal), and store into `p.parsedResources.bodies[rtMeta.ID]` / `p.parsedResources.resources[rtMeta.ID]` (mirroring parser.go:544-545).
- Do **not** call `gohcl.DecodeBody` anywhere in this phase — `Source`/`Variables`/`Disabled` remain zero-valued on the shell until the Phase-2 DAG walk decodes them (this is what satisfies the "must not reintroduce eager/inline decoding" constraint).

**Complexity**: Medium (new function closely mirrors an existing one, but requires careful reuse of `parseResource`'s shell-creation subroutines without duplicating validation logic).
**Token estimate**: ~20k tokens.
**Agent strategy**: Single agent, sequential execution — this phase is a self-contained addition to `parser.go` with no cross-file coordination needed yet.

### Phase 1.2: Recurse into a module's local source directory and scope its child resources

- Inside the new `parseModule` (added in Phase 1.1), after storing the module's own shell: resolve `Source` (read directly off the block's HCL body via `b.Body.Attributes["source"]` and `.Expr.Value(nil)` — mirroring how `parser.go:518-522` reads the output `description` attribute at parse time without a full context, since `Source` must be resolvable before decode) as a path relative to `filepath.Dir(file)` using stdlib `path/filepath`.
- Call the existing `findXclFiles` (`internal/parser/util.go:20-53`) against the resolved directory — reused unchanged, already handles directory-vs-file and `.xcl` filtering.
- For each discovered child file, call `p.parseResourcesInFile(childFile, moduleInstanceName)` (`parser.go:273`, reused unchanged) where `moduleInstanceName` is the module's own fully-qualified module path (`parentModule` + this module's own name, dot-joined — matching the existing `FQRN.AppendParentModule` convention at `internal/resources/fqrn.go:150-164` so nested modules compose correctly without new logic).
- Do **not** call `findVarsFiles` (`internal/parser/util.go:59-104`) for the module's resolved directory — confirmed required by `TestDoesNotLoadsVariablesFilesFromInsideModules` (`internal/parser/parse_test.go:444-458`); `.vars` auto-discovery stays scoped to the paths passed to the top-level `Parse()` call only (`parser.go:202`).
- No changes needed to `internal/parser/dag.go`, `internal/parser/util.go:493-568` (`getResourceDependencies`), or `internal/parser/exp.go` — confirmed in research.md that dependency resolution, module-vertex edges, and cross-module reference resolution already work correctly once shells with correct `Meta.Module`/`Meta.Links` exist.

**Complexity**: Medium (directory resolution + recursive parsing call, but reuses existing recursive-parse-capable functions rather than inventing new ones).
**Token estimate**: ~20k tokens.
**Agent strategy**: Single agent, sequential execution — depends directly on Phase 1.1's `parseModule` function existing; not parallelizable with it.

### Phase 2.1: Pass module-instance variables to child resources, with default fallback

- `internal/resources/module.go:19` — retype `Variables any` to `Variables map[string]cty.Value \`hcl:"variables,optional" json:"variables,omitempty"\`` (or equivalent `cty.Value`-based shape). Confirmed necessary by reading the vendored `gohcl`/`gocty` decode path directly: `gohcl.decodeBodyToValue` (`gohcl/decode.go:102-131`) decodes a plain attribute field via `DecodeExpression(attr.Expr, ctx, fieldV.Addr().Interface())`, which calls `gocty.ImpliedType(val)` (`gohcl/decode.go:294`); `gocty.ImpliedType`'s type switch (`github.com/jumppad-labs/go-cty`'s `cty/gocty/type_implied.go:30-73`) has no `case reflect.Interface` and falls through to its `default: return cty.NilType, path.NewErrorf(...)` branch for any `any`/`interface{}`-kind field — meaning `Variables any` as currently declared cannot be decoded by `gohcl.DecodeBody` at all and would error on every module block with a `variables = { ... }` attribute. This is a required fix, not an assumption to verify later.
- `internal/resources/module.go:22-23` — `Module.SubContext *hcl.EvalContext` field: populate it during the module vertex's own decode step.
- `internal/parser/callbacks.go:22-163` (`walkCallback`) — after the module vertex's own `gohcl.DecodeBody` call (line 101) succeeds and `rMeta.Type == resources.TypeModule`, build a merged `cty.ObjectVal` from the module's own `Variables map[string]cty.Value` field (now decodable) — this replaces the dead branch commented out at callbacks.go:128-138 (`processModuleVariables` reference) with real logic that resolves the module's own variable defaults (by looking up its child `Variable` shells the same way `buildContextForResource` already does at `context.go:52-57`) overridden by whatever the decoded `Variables` map supplies, then stores the result as an `*hcl.EvalContext` (or the raw merged `cty.ObjectVal`, whichever `SubContext`'s declared type most naturally holds) onto `Module.SubContext`.
- `internal/parser/context.go:103-122` — replace the empty `if rMeta.Module != "" { }` stub with logic that looks up the owning module resource via `res.resources["module."+rMeta.Module]` (or the appropriate FQRN string for the resource's own module scope — reuse `resources.ParseFQRN`/`FQRN.String()` patterns already used elsewhere in this same function at context.go:36 and 42), reads its `SubContext`, and merges those values into `variableVars` before line 101's `ctx.Variables["variable"] = cty.ObjectVal(variableVars)` (this replaces the dead `getModuleVariables` reference at context.go:105-121 with real logic operating on the now-populated `SubContext` instead of a nonexistent helper).
- Precedence: module's own variable defaults (already in `variableVars` from context.go:52-57 for that module's own `Variable` shells) overridden by `SubContext`'s supplied values when present — a simple two-key-set merge, no `.vars`/env layer involved (that stays gated to the `else if options != nil` branch at context.go:123-138, unchanged).

**Complexity**: High (touches decode-order-sensitive logic across two files, plus a field retype whose ripple effects on any code that reads `Module.Variables` elsewhere must be checked — a repo-wide grep for `.Variables` usage on `*resources.Module` confirms no other read sites exist today, so the retype is self-contained to this phase).
**Token estimate**: ~35k tokens.
**Agent strategy**: Single agent, sequential execution — the retype, `SubContext` population, and context-builder read side are one coherent data-contract change best implemented together rather than split across parallel agents.

### Phase 2.2: Confirm nested modules work end-to-end

- No new production code expected — this phase is verification-and-fix, exercised via `internal/test_fixtures/config/disabled/modules/resources.xcl` (its own nested `module "sub" { ... source = "./sub-modules" }` block) and `internal/test_fixtures/config/disabled/modules/sub-modules/resources.xcl`.
- Confirm `parseModule` (Phase 1.1/1.2) correctly dot-joins module names across nesting levels when recursing (i.e. a nested module's `moduleInstanceName` argument to `parseResourcesInFile` must be `outerModuleName + "." + innerModuleName`, matching `FQRN.AppendParentModule`'s existing convention at `internal/resources/fqrn.go:150-164`).
- Confirm `buildContextForResource`'s module-lookup logic (Phase 2.1, `context.go:103-122`) correctly resolves a nested module's own parent-module-qualified FQRN when looking up its `SubContext`.
- If any gap is found, the fix belongs in `parseModule` (Phase 1.1/1.2) or `buildContextForResource` (Phase 2.1) — this phase does not introduce a third code path for nesting, per the "no special-casing for nesting" design decision in plan.md's Implementation Detail section.

**Complexity**: Low (verification-focused; fixes if needed are localized to already-touched functions).
**Token estimate**: ~10k tokens.
**Agent strategy**: Single agent, sequential execution — depends on Phases 1.1, 1.2, and 2.1 all being complete.

### Phase 3.1: Reflect a disabled module instance onto its resources

- `internal/parser/parser.go:762` (`walk` function, the `w.Callback = walkCallback(...)` call) — pass `currentState` (already a `state.State`, which already implements `ResourceProvider`/`ConfigProvider` per `internal/parser/dag.go:12-23`) as an additional argument to `walkCallback`.
- `internal/parser/callbacks.go:22` — `walkCallback`'s signature gains a `rp ResourceProvider` (or similarly-named) parameter, threaded through the closure.
- `internal/parser/callbacks.go:75-90` — replace the commented-out block (which references an undefined `c`) with real logic: after `processDisabled` returns `isDisabled` (line 70) and `rMeta.Type == resources.TypeModule`, if `isDisabled`, call `rp.FindModuleResources(rMeta.ID, true)` (already implemented, `state/state.go:132-175`) and `types.SetDisabled(d, true)` (already implemented, `types/resource_helpers.go:150`) on each returned resource.
- No changes needed to the existing early-exit at `callbacks.go:39-46` (`if disabled { return nil }`) — this is the mechanism that already causes a propagated-disabled child to skip further decode while staying in state, confirmed by the passing `TestParseDoesNotProcessDisabledResources` (`internal/parser/parse_test.go:528-555`).
- Because of the implicit module-parent dependency edge already added by `getResourceDependencies` (`internal/parser/util.go:550-565`), the DAG guarantees a module vertex is walked before its children, so this propagation reliably runs before any child's own `walkCallback` invocation — this was traced directly through the `jumppad-labs/dag` fork's `walk.go` (its channel-based `walkVertex` blocks on each dependency's `DoneCh` before invoking its own callback) and confirmed true by construction, not merely assumed.

**Complexity**: Medium (signature change threads through one call site and one closure, but the propagation logic itself is a direct port of already-drafted commented-out code using already-implemented `state.State` methods).
**Token estimate**: ~20k tokens.
**Agent strategy**: Single agent, sequential execution — small, well-scoped change; depends on Phases 1.1/1.2 (module resources must exist in state) but not on Phase 2.x.

### Phase 3.2: Let a resource inside a module override its own disabled state

- No changes expected to `internal/parser/parser.go:705-734` (`processDisabled`) — this function already evaluates a resource's own `disabled = <expr>` attribute against that resource's freshly-built context (called at `callbacks.go:70`, after `buildContextForResource` at line 56), so a child's own disabled expression (e.g. `disabled = !variable.enabled` in `internal/test_fixtures/config/single/container.xcl`) already reflects that resource's own resolved `variable.enabled` value once Phase 2.1's variable-passing merge is in place.
- Verify only: confirm the propagation added in Phase 3.1 (`callbacks.go:75-90`) only sets `Disabled=true` as a pre-set (never clearing an already-`false`/independently-evaluated child), and that a child's own `processDisabled` call still runs and can independently determine `false` even if the module itself is not disabled — this should already hold since Phase 3.1's propagation only fires when the module itself is disabled, and a not-disabled module never touches its children's `Disabled` field at all.
- Target fixtures: `internal/test_fixtures/config/single/container.xcl`'s `sidecar` resource (`disabled = !variable.enabled`), exercised via `TestModuleDisabledCanBeOverriden` (`internal/parser/parse_test.go:460-490`).

**Complexity**: Low (expected to be a verification-only phase with no new code, since the override behavior already falls out of Phases 2.1 and 3.1 combined).
**Token estimate**: ~10k tokens.
**Agent strategy**: Single agent, sequential execution — depends on both Phase 2.1 (variable passing) and Phase 3.1 (disabled propagation) being complete.

### Phase 4.1: Align the module test fixture and test suite with the local-only scope

- `internal/test_fixtures/config/modules/modules.xcl:41-45` — replace the `consul_3` module block (currently `source = "github.com/jumppad-labs/xcl/test_fixtures//single"` with `depends_on = ["module.consul_1"]`) with a local-source instance, e.g. `source = "../single"` (matching `consul_1`/`consul_2`'s pattern), keeping the `depends_on` attribute if still useful for exercising module-to-module `depends_on` resolution (`internal/parser/util.go:518-532`'s `TypeModule` dependency-expansion branch), or removing it if no longer meaningful once the source is local.
- `internal/parser/parse_test.go:335-365` (`TestParseModuleCreatesResources`) — update the `consul_3` lookup/assertion (currently checks `cont.Networks[0].Name == "onprem"` for `module.consul_3.resource.container.consul`) to remain valid against the new local-source instance (should require no change if the fixture edit preserves the same resource shape, since `single/container.xcl` is the shared source for all three instances).
- `internal/parser/parse_test.go:367-384` (`TestParseModuleDoesNotCacheLocalFiles`) — remove the remote-cache assertion (`require.DirExists(t, filepath.Join(p.options.ModuleCache, "github.com_jumppad-labs_xcl_test_fixtures_single"))`, line ~380), keep the local-no-cache assertion (`require.NoDirExists(t, filepath.Join(p.options.ModuleCache, "single"))`, line ~383) — per user confirmation in the architecture step, this test is narrowed rather than deleted, since the local-source-does-not-cache guarantee is still real, in-scope behavior worth asserting.
- `internal/parser/parse_test.go:386-442` (`TestParseModuleCreatesOutputs`) — verify `output.module3_container_resources_cpu` (currently expects `2048`, the module's own internal default, since `consul_3` passes no variables) still holds true against the new local-source `consul_3` instance; should require no change since the source content and no-variables-passed behavior are unaffected by switching from remote to local fetch.
- Run the full parser package test suite (`internal/parser/...`) after this edit to confirm both spec success metrics: all six module tests pass, and the full suite has no regressions.

**Complexity**: Low (fixture/test data edits only, no production code changes).
**Token estimate**: ~15k tokens.
**Agent strategy**: Single agent, sequential execution — final phase, depends on all of Phases 1.1 through 3.2 being complete and passing.

## Testing Strategy

Per-phase testing is entirely regression-driven against the six pre-existing module tests in `internal/parser/parse_test.go`, run via `go test ./internal/parser/... -run 'TestParseModule|TestDoesNotLoadsVariablesFilesFromInsideModules|TestModuleDisabledCanBeOverriden|TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled' -v` after each phase to track incremental progress, and the full `go test ./...` after Phase 4.1 to confirm no regressions. No new test files are created. Phase-to-test mapping:

- Phase 1.1/1.2 → `TestParseModuleCreatesResources` (`parse_test.go:335-365`) should begin passing once shells exist and are correctly scoped/decoded.
- Phase 2.1 → `TestParseModuleCreatesOutputs` (`parse_test.go:386-442`) and `TestDoesNotLoadsVariablesFilesFromInsideModules` (`parse_test.go:444-458`).
- Phase 2.2 → no dedicated test beyond confirming the nested-module assertions inside `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` (`parse_test.go:557-585`) pass, since no fixture exercises nesting outside the disabled scenario.
- Phase 3.1/3.2 → `TestModuleDisabledCanBeOverriden` (`parse_test.go:460-490`) and `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` (`parse_test.go:557-585`).
- Phase 4.1 → `TestParseModuleDoesNotCacheLocalFiles` (`parse_test.go:367-384`, narrowed) plus a full-suite run as the final gate.

## Project References

- `.spektacular/knowledge/decisions/parser-refactor-analysis.md` — the two-phase parse-then-decode refactor rationale, dated 2025-12-27; explicitly lists module support as deferred future work. Read this first when rehydrating cold.
- `.spektacular/specs/20260714080036-restore-module-support.md` — the spec this plan implements; source of truth for requirements, constraints, and acceptance criteria.
- `research.md` (this plan's own research document) — the full decision log: alternatives rejected, chosen-approach evidence, files examined, and resolved assumptions (including the `Module.Variables any` decode bug and DAG-ordering verification).
- `internal/parser/parse_test.go:335-490, 557-585` — the six module tests that are this plan's primary acceptance mechanism; read these directly before starting any phase, since they are the executable specification of expected behavior.
- `internal/test_fixtures/config/modules/`, `internal/test_fixtures/config/single/`, `internal/test_fixtures/config/disabled/` — the fixture trees the module tests parse against; read alongside the tests that reference them.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

This plan's phases are mostly Low/Medium, with one High-complexity phase (2.1) at ~35k tokens due to its cross-file data-contract change (field retype + two decode-time call sites). All phases are run single-agent/sequential per their individual agent-strategy notes above, since each phase depends on the previous one's code existing (module shells before decode-time logic, variable passing before disabled-override verification) — there is no meaningful cross-phase parallelism opportunity in this plan.

## Migration Notes

None. This plan changes internal parser behavior only; no data migration, schema migration, or user-facing config migration is required. Existing non-module `.xcl` configurations are unaffected. Configurations that already contain `module` blocks were previously being silently skipped (their module resources never appeared in state) — after this plan lands, those same configurations will begin producing module resources in state, which is the intended restoration of prior behavior, not a breaking change requiring migration steps.

## Performance Considerations

None identified. Module parsing reuses the existing file-discovery and DAG-walk machinery; the added work (recursing into a module's source directory, an additional variable-merge computed once per module instance during decode) scales linearly with the number of module instances and their resource counts, consistent with how the parser already scales with the number of root-level resources. No new I/O patterns beyond what non-module `.xcl` file discovery already performs (`os.ReadDir` per resolved directory), and explicitly no network I/O since remote sources are out of scope.
