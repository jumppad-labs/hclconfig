# Plan: 20260714080036-restore-module-support

<!-- Metadata -->
<!-- Created: 2026-07-15T07:48:01Z -->
<!-- Commit: a908757 -->
<!-- Branch: v2 -->
<!-- Repository: git@github.com:jumppad-labs/hclconfig.git -->

## Overview

This plan restores module support to xclconfig's v2 parser: the ability to package configuration as a named, reusable unit and instantiate it multiple times with different variables. Module blocks are currently parsed and then silently skipped, so their resources never appear in state. This work fits module parsing into the existing two-phase parse-then-decode architecture — completing several already-drafted but commented-out hooks rather than introducing new mechanisms — so that users can once again share and reuse common infrastructure patterns across a configuration without duplicating them.

## Conventions

- **Prefer small, focused interfaces** (`conventions/code-style.md`) — the new `parseModule` function is added as a narrowly-scoped addition alongside `parseResource`, not a broadened do-everything parser method.
- **Prefer composition over inheritance** (`conventions/patterns-and-architecture.md`) — module parsing reuses and composes with the existing shell-creation, link-extraction, and DAG-walk machinery (`parseResource`, `findXclFiles`, `buildContextForResource`) rather than introducing a parallel module-specific mechanism, directly matching the spec's own technical-approach steer to reuse existing DAG/FQRN/Link infrastructure.
- **Testing & Mocking rules** (`conventions/testing-and-mocking.md`) — the six target module tests and the new/edited fixture test must follow testify `require`, no table-driven tests, and no mixing of positive/negative assertions in one test function; this governs how the Testing Approach section and any fixture-test edits (e.g. the `TestParseModuleDoesNotCacheLocalFiles` narrowing) must be written.
- **Pin dependency versions / prefer standard library** (`conventions/dependencies.md`) — directly satisfied already: the chosen approach introduces no new go.mod dependencies, using only `hashicorp/hcl/v2` and stdlib `path/filepath`, both already present and pinned.

Dropped as not applicable to this feature: Database & External Services (no DB/connection-pooling surface in parser work), Development Standards' structured-logging rule (no new logging surface introduced), Project Structure (no new top-level directories or packages are being added).

## Architecture & Design Decisions

The chosen design restores module support entirely within the existing two-phase parse-then-decode architecture, adding one new Phase-1 code path and completing three already-drafted (but commented-out) Phase-2 hooks — no new subsystems, graph structures, or resource-addressing mechanisms are introduced. Research (`research.md#chosen-approach--evidence`) found that the DAG, FQRN nested-module scoping, and dependency resolution are already fully module-aware; the gap is narrow and localized to three files in the parser package.

**Module discovery and shell creation.** A dedicated `parseModule` function (replacing the currently-stubbed module case) creates the module's `*resources.Module` shell through the same non-eager path `parseResource` already uses for ordinary resources — link-extracting its `variables`/`source`/`disabled` attributes and storing its raw HCL body, never calling `gohcl.DecodeBody` at parse time. It then resolves `Source` as a local relative path against the including file's directory, discovers that directory's `.xcl` files, and recurses into each child file via the already-plumbed-but-unused module-name parameter that the existing file-parsing function already accepts. This was chosen over folding module handling directly into the block-dispatch loop because it keeps the recursive-directory-walk concern (module) cleanly separated from the flat per-block dispatch concern (ordinary resources/variables/outputs), mirroring the shape of the pre-refactor v1 module parser while deferring all decoding to Phase 2 as the constraints require. This also directly satisfies the "prefer small, focused interfaces" and "prefer composition over inheritance" conventions above — module parsing composes with the existing resource-parsing path rather than duplicating or subclassing it. Per-instance isolation (no state leakage between repeated instantiations of the same source) falls out for free: each recursion creates fresh shell objects and does not cache or reuse parsed shells across instances.

**Module-instance variable passing.** When the module vertex itself is decoded during the DAG walk, the newly-added logic builds a merged variable set — module's own variable defaults overridden by any instance-supplied values — and stores it on the module resource's previously-unused `SubContext` field, whose doc comment already describes exactly this purpose. The interpolation context builder (currently an empty stub for module-scoped resources) is completed to look up the owning module resource for any module-scoped child and read `SubContext` when building that child's variable context. This was chosen over recomputing the merge fresh for every child because DAG ordering already guarantees the module vertex decodes before its children, so the merge is naturally computed once per module instance and read many times — cheaper, and it exercises a field that was already added and documented for this exact purpose rather than leaving it permanently dead. The precedence is a two-layer merge (module's own variable defaults, then instance-supplied overrides) — narrower than the root-level defaults-then-files-then-env-then-direct chain, since `.vars`-file and environment-variable layers are explicitly root-only, confirmed by an existing passing test and the existing gating logic that already excludes module-scoped resources from that chain.

**Disabled-state propagation and override.** A previously commented-out propagation block is completed and wired with a live resource-provider parameter threaded into the decode-time walk callback, so that when a module vertex evaluates as disabled, all of its current child resources are located and marked disabled ahead of their own decode step. Because DAG order guarantees the module decodes first, and each child's existing early-exit already skips further processing once its own disabled flag is true while leaving the resource visible in state, this reuses a mechanism already proven by a passing non-module test — no new disabled-handling code path is needed, only propagation of an existing flag. A child resource's own `disabled = <expr>` attribute is evaluated after context build by pre-existing logic, so it independently reflects the child's own variables regardless of module-level state, satisfying the override requirement without any special-casing.

**Scope note carried from discovery.** The existing module test fixture currently instantiates a third module instance from a remote go-getter source, and its companion test asserts that source gets fetched and cached — both conflict with the spec's local-relative-path-only constraint. Per user decision during the architecture step, this plan's phases include editing that fixture (replacing the remote instance with a second local instance) and narrowing the test to assert only local-source no-caching behavior, so the "all currently-failing module-related tests pass" success metric remains achievable without building remote-source fetching. Full alternative analysis, including why inline eager-decode and a parallel container-vertex DAG abstraction were rejected, is in [research.md#alternatives-considered-and-rejected](./research.md#alternatives-considered-and-rejected).

## Component Breakdown

- **Module block parser** (new). Owns discovering `module` blocks during Phase-1 file parsing, creating the module's resource shell (unresolved, body stored for later decode), resolving its `source` attribute to a local directory, and recursively parsing that directory's files as module-scoped resources. Sits alongside the existing per-resource parser as a sibling entry point, and hands its discovered child resources into the same shell/body storage the rest of Phase 1 already uses — it does not decode anything itself.

- **File/resource parser** (changed). The existing per-block parsing path (resource/variable/output shell creation, link extraction, body storage) is extended only insofar as it's now invoked recursively, with a non-empty module-instance name, by the module block parser above. Its own per-block logic is unchanged; it owns creating shells and extracting dependency links regardless of whether it's operating at the root or inside a module.

- **Dependency graph builder** (unchanged, reused). Owns building the DAG from all parsed resource shells (root and module-scoped alike) and their extracted links, and walking it in dependency order. No changes required — it is already module-aware and already expands module-level dependencies and module-parent edges correctly once module shells exist to be discovered.

- **Resource decoder / walk callback** (changed). Owns decoding each shell's HCL body in DAG order, evaluating its disabled state, and invoking any provider lifecycle. Gains two new responsibilities: propagating a disabled module's state onto its child resources once the module vertex itself has been evaluated, and (working with the context builder below) making a module instance's resolved variables available before its children decode. Everything else about its role — decode order, disabled short-circuiting, provider lifecycle skip-list for builtin types — is unchanged.

- **Interpolation context builder** (changed). Owns assembling the HCL evaluation context (variables, resource references) available to a shell at decode time. Gains the ability to recognize when a resource being decoded belongs to a module, look up that module's resolved variable values, and merge them over the module's own variable defaults before building the child's `variable` context — completing an existing but currently inert scoping branch. Its root-level behavior (loading `.vars` files, environment variables, direct-supplied variables) is unchanged and remains excluded for module-scoped resources.

- **Module resource type** (changed). Owns the shape of a module instance's own declared data — its source location, version (unused/out of scope), instance-supplied variables, and (newly used) a resolved-variables context handed down to its children. No structural change to its serializable fields; only its previously-unused variable-context field becomes populated and consumed.

- **State / resource addressing** (unchanged, reused). Owns registering, querying, and serializing all resources — root and module-scoped alike — by their fully-qualified path, including per-instance and nested-module addressing and disabled-scoped module queries. No changes required; this is the component that already makes multi-instance and nested-module scoping "just work" once shells are correctly registered with the right scoping metadata.

- **Module test fixtures** (changed). Owns the on-disk `.xcl` configurations and companion Go tests that exercise module parsing end to end. The existing fixture set is adjusted to remove its one out-of-scope remote-source instance (replaced with an additional local instance) and its corresponding test narrowed to assert only local-source behavior, so the full fixture set stays within the local-relative-path-only constraint while still exercising multi-instance, nested-module, variable-passing, and disabled-propagation scenarios.

## Data Structures & Interfaces

No new exported types are introduced. One existing field is retyped (see below); the on-disk/serialized `State` JSON shape is unaffected regardless, since `resources.Module` is never itself a member of `types.Meta`/`ResourceBase` or otherwise part of `State`'s serialized shape — retyping it changes nothing about how `State`'s JSON is structured.

**`resources.Module`** (existing struct, one field retyped) — its `Source string` field is now actually read and resolved (as a local relative path) instead of being ignored. Its `Variables any` field is retyped to `map[string]cty.Value` (or an equivalent `cty.Value`-based shape): confirmed by reading `gohcl`'s decode path that an `any`/`interface{}`-kind struct field has no corresponding case in the underlying `gocty.ImpliedType`'s type switch — meaning `Variables any` as currently typed cannot be decoded by `gohcl.DecodeBody` at all, it would error every time a `variables = { ... }` attribute is present. This was verified directly against the vendored decode source rather than assumed. Its previously-unused `SubContext *hcl.EvalContext` field becomes populated during the module vertex's own decode step, holding the merged (defaults-overridden-by-supplied) variable values as an HCL evaluation context, and is read by the interpolation context builder when building context for any resource scoped to that module instance.

**Module-parsing entry point** (new internal function) — conceptually `parseModule(file string, block *hclsyntax.Block, parentModule string) []error`, called from the existing per-block dispatch switch in place of today's skip. Its contract: create and register the module's shell exactly as the resource parser does for other block types (so it participates in link extraction and Phase-2 decode), resolve `Source` to a local directory relative to the declaring file, and recurse into that directory's `.xcl` files via the existing file-parsing function's signature — passing the module instance's fully-qualified name so child shells get correctly scoped. This reuses an existing signature rather than introducing a new one.

**Resource decoder / walk callback** (existing internal function, signature change) — gains a resource-provider parameter (the same interface `state.State` already implements) so that, when a module vertex is found disabled, it can look up and flag that module's current child resources. This is an additive parameter on an already-internal function; no public interface changes.

**Interpolation context builder** (existing internal function, behavior change only) — its existing signature is retained; the change is purely internal logic that, for a module-scoped resource, looks up the owning module shell and reads its `SubContext` (once populated) to merge module-instance variable values into the resource's `variable` evaluation context. No new parameters are required since the owning module shell is already reachable through the same working-resource map the function already consults for other dependency lookups.

**Test fixtures** (data, not code) — the module fixture is edited to replace its one remote-source module instance with an additional local-source instance; no new fixture files or directory layouts are introduced beyond what the nested-module and disabled-propagation scenarios already require.

## Implementation Detail

This work introduces no new architectural pattern — it completes an existing one. The two-phase parse-then-decode model, the DAG-based dependency walk, and the dot-joined FQRN module-scoping scheme are all established patterns already used for ordinary resources; module support is implemented by extending each of them to a block type that was previously excluded, not by introducing a parallel mechanism alongside them.

The main code-shape change is recursive self-similarity: module parsing is Phase-1 file parsing calling itself, scoped one level deeper. A developer reading the module-parsing code should recognize the same shape as the top-level parse entry point (discover files in a directory, parse each block, extract dependency links, store shells for later decode) rather than a bespoke module-specific code path — the recursion argument (an accumulating module-instance name) is the only thing that changes between a root parse and a module parse, and nested modules require no special-casing beyond calling the same recursive step again with a further-qualified name. This mirrors how the FQRN's module field already supports arbitrary nesting depth as a single dot-joined string rather than a tree structure.

The disabled-propagation and variable-passing additions follow the existing "resolve during the DAG walk, using only what's already been decoded" discipline rather than reaching back into raw parse-time state. A developer extending decode-time behavior in the future should expect to find module-aware logic living entirely inside the walk callback and context-building step (where all other per-resource decode logic already lives), not scattered into Phase-1 parsing — Phase 1 stays purely about discovering shells and their dependency links, exactly as it does today for non-module resources.

No package boundaries move, no interfaces are introduced to replace concrete types, and no existing non-module code path changes behavior — a developer working on non-module configurations should see no difference in how parsing, decoding, or state population behaves. The only externally-observable change is that `module` blocks, previously silently skipped, now produce resources in state.

## Dependencies

- **`github.com/hashicorp/hcl/v2`** — already a direct go.mod dependency; provides `hclparse`, `hclsyntax`, and `gohcl.DecodeBody`/`DecodeExpression`, used unchanged for module block parsing and decoding. No version change or new usage pattern required.
- **`github.com/zclconf/go-cty`** (via the project's `jumppad-labs/go-cty` replace) — already a direct dependency; provides `cty.Value`/`cty.ObjectVal`, used to build the merged variable context stored on `Module.SubContext`. No version change required.
- **`github.com/silas/dag`** (via the project's `jumppad-labs/dag` replace) — already a direct dependency; the DAG walker is used unchanged, since module-scoped resources are added as ordinary vertices through the existing dependency-resolution machinery.
- **stdlib `path/filepath`** — used unchanged for resolving a module's `source` attribute to a local directory relative to the including file, the same way existing file-discovery code already resolves paths.
- **`internal/resources` package (FQRN, `Module`, `Variable`, `Output` types)** — already supports nested module scoping and is consumed as-is; only `resources.Module`'s previously-unused fields become active.
- **`state` package (`State`, `ConfigProvider`/`ResourceProvider` interfaces)** — already implements `FindModuleResources`/`FindRelativeResource`; consumed as-is by the completed disabled-propagation logic. No changes to `state` package required.
- **No new external dependencies.** Everything required is already present in go.mod; this satisfies the spec's constraint directly.
- **No prior specs or plans must land first.** This is the only spec/plan in the store.
- **Explicitly not depended on**: `github.com/hashicorp/go-getter` and the existing `internal/getter` package — fully implemented but intentionally left unused, since remote/URL module sources are out of scope per the spec's constraints.

## Testing Approach

Testing is entirely unit-level, following the project's existing parser test conventions (testify `require`, no table-driven tests, no mixing positive/negative assertions in one test function): the six existing module tests in the parser test suite are the primary coverage surface and are treated as regression tests to fix, not tests to rewrite around new behavior — they already encode the acceptance criteria from the spec (multi-instance scoping, variable passing with default fallback, disabled propagation and override, module outputs, and — via the disabled-fixture's nested `sub` module — nested-module scoping). The one exception is `TestParseModuleDoesNotCacheLocalFiles`, which is narrowed to drop its remote-source assertion while keeping its local-source no-caching assertion, since remote sources are out of scope.

The resource decoder / walk callback and interpolation context builder get the heaviest indirect coverage, since nearly every module test exercises decode-time behavior (variable merging, disabled evaluation) rather than parse-time shell creation alone — parse-time correctness (shells created, links extracted, no panics) is a precondition the decode-time tests already fail loudly on today, so no dedicated parse-only test is added beyond what's needed to keep the existing suite passing.

No new integration or end-to-end tests are added: the six module tests already exercise the full parse-through-state pipeline (file discovery → shell creation → DAG walk → decode → state query) against realistic fixtures, which constitutes end-to-end coverage for this feature within the existing test style. No new fixture directories are needed beyond the one edit already scoped; the disabled/nested-module fixtures already exist and already assert the nested-scoping behavior.

Deliberate gap: no dedicated unit tests are added purely for the internals of `parseModule`, the `SubContext` merge logic, or the disabled-propagation code in isolation — these are covered exclusively through the existing black-box module tests asserting on final decoded state, consistent with how the rest of the parser package is tested.

**Success metrics mapping** (from the spec):
- *"All currently-failing module-related parser tests pass"* — **Behavioural test**. The six named tests (`TestParseModuleCreatesResources`, `TestParseModuleDoesNotCacheLocalFiles` as narrowed, `TestParseModuleCreatesOutputs`, `TestDoesNotLoadsVariablesFilesFromInsideModules`, `TestModuleDisabledCanBeOverriden`, `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled`) must all pass; each asserts a specific piece of module behavior directly against decoded state.
- *"The full parser test suite passes with no regressions introduced"* — **Behavioural test**. The entire existing parser package test suite (all currently-passing non-module tests) must continue to pass unmodified, verifying no shared code path regresses for non-module configurations.

## Milestones & Phases

### Milestone 1: Modules can be defined, instantiated, and their resources show up in state

**What changes**: Users can write a `module` block pointing at a local directory of configuration and have that module's resources actually appear in the parsed configuration — addressable by a path that includes the module instance's name. Instantiating the same module source multiple times under different names produces independent, non-colliding sets of resources for each instance, and resources inside a module can reference each other and be referenced from the module's own dependency graph regardless of the order they're declared in. This is the foundational milestone: it turns `module` blocks from silently-skipped into real, working configuration, which every other module capability depends on.

#### - [x] Phase 1.1: Parse module blocks into shells instead of skipping them

Replace the current behavior of silently ignoring `module` blocks with real parsing: a module block becomes a proper resource shell (like any other resource, variable, or output), with its own dependency links extracted from its `source`, `variables`, and `disabled` attributes. Nothing is decoded yet at this point — this phase only makes modules visible to the parser as a discoverable, trackable unit of configuration, matching the existing behavior for every other resource type.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-parse-module-blocks-into-shells-instead-of-skipping-them)

**Acceptance criteria**:
- [x] A configuration containing a `module` block parses without error instead of being silently skipped.
- [x] The module's own attributes (source, variables, disabled) are tracked as dependencies the same way an ordinary resource's attributes are.

#### - [x] Phase 1.2: Recurse into a module's local source directory and scope its child resources

When a module block is parsed, resolve its `source` attribute as a directory relative to the file that declared it, discover the `.xcl` files inside that directory, and parse each one exactly as the top-level configuration is parsed — except every resource created this way is scoped under the module instance's name. This is what makes a module's resources actually show up, addressable by a path that includes which module instance they came from, and what makes instantiating the same module source multiple times produce independent resource sets.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-recurse-into-a-modules-local-source-directory-and-scope-its-child-resources)

**Acceptance criteria**:
- [x] Resources declared inside a module's source configuration are present and retrievable from state, addressed by a path that includes the module instance's name.
- [x] Two module instances built from the same source, given different instance names, produce fully independent resources — inspecting or changing one instance's resources never affects the other's.
- [x] A resource inside a module that references another resource or variable declared in the same module resolves to the correct value.
- [x] Parsing succeeds and produces correctly evaluated values regardless of whether a resource, variable, or module instance is declared before or after what it depends on.

### Milestone 2: Module variables flow in with correct default fallback, and nested modules work

**What changes**: Users can pass variable values into a module instance, and those values are used inside that instance in place of the module's own defaults — while instances that don't supply a value still get the module's default. This also covers modules that themselves contain other module instances, so a module can be built by composing smaller modules, matching prior supported behavior. This builds directly on Milestone 1's resource scoping, since variable values must be resolved before the module's decoded resources reflect them.

#### - [x] Phase 2.1: Pass module-instance variables to child resources, with default fallback

A module instance can supply values for the module's own variables, and those values should be used inside that instance instead of the module's own defaults — while an instance that supplies nothing still gets the module's default. This phase makes that override actually happen by merging the module's supplied variables over its defaults once the module itself is evaluated, and making that merged set available to every resource inside that instance.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-pass-module-instance-variables-to-child-resources-with-default-fallback)

**Acceptance criteria**:
- [x] A module instance that supplies a value for one of the module's variables produces resources that reflect the supplied value, not the module's own default.
- [x] A module instance that does not supply a value for one of the module's variables produces resources that reflect the module's own default value.
- [x] A module's declared outputs are retrievable per instance, and their values correctly reflect that instance's variable values (including values derived from other resources outside the module).

#### - [x] Phase 2.2: Confirm nested modules work end-to-end

A module can itself contain another module instance. This phase verifies (and fixes anything that doesn't already fall out naturally from Phases 1.1–2.1) that resources inside a nested module instance are correctly scoped, evaluated, and addressable by a path reflecting both the outer and inner module instance names — matching how the configuration behaved before this restoration work began.

*Technical detail:* [context.md#phase-22](./context.md#phase-22-confirm-nested-modules-work-end-to-end)

**Acceptance criteria**:
- [x] Resources inside a nested module instance are retrievable from state, addressed by a path that reflects both the outer and inner module instance names.
- [x] A module's own variable-passing and dependency-link extraction work the same way whether the module is at the top level or nested inside another module.

### Milestone 3: Disabling a module instance is reflected on its resources, with per-resource override

**What changes**: Users can mark a module instance as disabled and see that reflected on every resource it produces, without those resources disappearing from the configuration's state. A resource inside a module can still independently determine its own disabled state — for example based on a variable — even when its owning module instance isn't itself disabled. This is the final module capability milestone, layered on top of variable passing (Milestone 2) since a resource's own disabled expression often depends on a module-supplied variable.

#### - [x] Phase 3.1: Reflect a disabled module instance onto its resources

Marking a module instance as disabled should be reflected on every resource that instance produces — those resources stay visible in the configuration's state (not removed), but are flagged as disabled and not processed further. This phase adds that propagation: once a module instance is evaluated as disabled, every resource currently known to belong to that instance is marked disabled before it would otherwise be processed.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-reflect-a-disabled-module-instance-onto-its-resources)

**Acceptance criteria**:
- [x] A module instance marked as disabled produces resources that are still retrievable from state, each reflecting a disabled outcome.
- [x] This disabled propagation also applies correctly through a nested module instance.

#### - [x] Phase 3.2: Let a resource inside a module override its own disabled state

A resource inside a module instance can determine its own disabled state independently — for example, based on one of the module's variables — even when the module instance itself is not disabled. This phase confirms (and adjusts if needed) that a child resource's own disabled expression is evaluated using that resource's own resolved variable values, so the child's disabled outcome reflects what its own expression dictates rather than being forced by the module.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-let-a-resource-inside-a-module-override-its-own-disabled-state)

**Acceptance criteria**:
- [x] A resource inside a module instance that is not itself disabled, but whose own disabled state depends on a variable, reflects what that variable dictates, independent of the module's disabled state.

### Milestone 4: Test suite is fully aligned with the local-only scope and the whole suite passes clean

**What changes**: This is an internal cleanup with no new user-visible capability — it removes the one remaining test dependency on a remote module source (which is explicitly out of scope for this work) so the full test suite reflects only in-scope, supported behavior. It's worth its own milestone because it's the gate for the spec's two success metrics: it can't be validated until Milestones 1-3 land, and it's what makes "all module-related tests pass" and "full suite passes with no regressions" true simultaneously and cleanly, without a lingering known-failing or scope-mismatched test in the suite.

#### - [x] Phase 4.1: Align the module test fixture and test suite with the local-only scope

This is an internal cleanup with no user-visible capability change. The existing test fixture set includes one module instance that uses a remote source, and a companion test that checks that remote source gets fetched and cached — both of which exercise functionality that is explicitly out of scope for this restoration. This phase replaces that instance with an additional local-source instance and narrows its companion test to only check local-source behavior, so the full test suite reflects only supported, in-scope functionality.

*Technical detail:* [context.md#phase-41](./context.md#phase-41-align-the-module-test-fixture-and-test-suite-with-the-local-only-scope)

**Acceptance criteria**:
- [x] The module test fixtures no longer depend on fetching configuration from a remote source.
- [x] All module-related tests pass, and the full parser test suite passes with no regressions to previously-passing, non-module behavior.

## Open Questions

No open questions remain. Every uncertainty surfaced during discovery and phase-drafting was resolved by direct code verification before this plan was finalized, rather than parked for implementation time:

- The exact decode shape of `resources.Module.Variables any` was traced through the vendored `gohcl`/`gocty` decode path and confirmed to be undecoded as declared — resolved as a concrete required fix (retype to `map[string]cty.Value`), captured in the Data Structures section and Phase 2.1.
- Whether the DAG walker strictly serializes a module vertex's callback before its children's was traced through the walker's channel-based dependency-waiting logic and confirmed true by construction.
- Whether the module block's own `variables` attribute gets its embedded resource/variable references correctly link-extracted was confirmed by reading the existing expression-walking code directly.
- The remote-source fixture/test conflict was resolved with the user during the architecture step.

## Out of Scope

- **Module version/update management** — resolving, pinning, or upgrading a module's source content by version is not addressed by this plan. From the spec's Non-Goals; the `Module.Version` field remains present on the struct but continues to go unread, exactly as it does today.
- **Provider/plugin lifecycle execution changes** — this plan covers parsing and state population only; how Create/Update lifecycle calls interact with module resources during actual execution is untouched. From the spec's Non-Goals.
- **Remote or URL/git-based module sources** — only local relative-path sources are supported by this plan. From the spec's Constraints (restated here since it directly shapes Phase 4.1): the fully-implemented `GoGetter` and `hashicorp/go-getter` dependency remain unused; wiring them up for module sources would be a separate, future piece of work if ever prioritized.
- **A remote-source module test fixture** — as a direct consequence of the above, `TestParseModuleDoesNotCacheLocalFiles`'s remote-fetch assertion is removed rather than made to pass; if remote module sources are ever built, a new test covering that behavior would need to be written fresh at that time, not resurrected from this plan's fixture edit.
- **Any change to the on-disk/serialized `State` JSON format** — not a feature exclusion so much as a hard boundary carried from the spec's Constraints: this plan achieves module support without touching `State`'s serialization shape at all, confirmed in the Data Structures section.

## Changelog

### 2026-07-15 — Phase 1.1: Parse module blocks into shells instead of skipping them

**What was done**: Replaced the `case resources.TypeModule: continue` stub in `parseResourcesInFile` (`internal/parser/parser.go`) with a call to a new `parseModule(file string, b *hclsyntax.Block, parentModule string) []error` function. `parseModule` mirrors `parseResource`'s shell-creation path: validates the module's single label, creates the shell via `p.createBuiltinResource(resources.TypeModule, name)`, sets `rtMeta.Module/File/Line/Column`, computes `rtMeta.ID` via `resources.FQRNFromResource`, extracts dependency links via `p.getUniqueResourceLinks(rt, b)`, and stores the shell/body into `p.parsedResources`. Deliberately does not call `gohcl.DecodeBody` — decode stays deferred to the Phase-2 DAG walk, per the plan's core constraint.

**Deviations**: None. Implementation matches the plan's Phase 1.1 technical detail exactly; no substitutions or scope changes were needed. `resources.TypeModule` was already registered in `resources.DefaultResources()` and `getUniqueResourceLinks` was already fully generic, so no extension was needed to either, as the plan anticipated.

**Files changed**:
- `internal/parser/parser.go`

**Discoveries**: `TestParserProcessesResourcesInCorrectOrder` (not one of the plan's 6 named module tests) also currently fails against the `modules/modules.xcl` fixture, for the same underlying reason (module decode/recursion not yet implemented) — confirmed via git-stash comparison that it was already failing/panicking before this phase, so it's pre-existing breakage in the same category, not a regression. It should be tracked alongside the 6 named tests as later phases land, and re-checked at Phase 4.1's full-suite gate.

### 2026-07-15 — Phase 1.2: Recurse into a module's local source directory and scope its child resources

**What was done**: Extended `parseModule` (`internal/parser/parser.go`) to read the module's `source` attribute as a raw literal value (`b.Body.Attributes["source"].Expr.Value(nil)`, mirroring the output `description` parse-time read), resolve it as a directory relative to `filepath.Dir(file)`, discover `.xcl` files in that directory via the existing `findXclFiles`, and recursively call `p.parseResourcesInFile(childFile, moduleInstanceName)` for each — where `moduleInstanceName` is `parentModule + "." + name` (guarded for an empty parent). This makes a module's child resources discoverable and correctly scoped under the module's own instance name, including through multiple levels of nesting.

**Deviations**: None. `FQRN.AppendParentModule` was initially suspected to be a general-purpose instance-name joiner but turned out to rewrite an existing FQRN's `.Module` field for nested-output resolution — unrelated to this phase's needs. Used a plain guarded dot-concat instead, matching that method's own internal joining convention without calling it directly. This was resolved during analysis, before any code was written, so it didn't require a mid-implementation correction.

**Files changed**:
- `internal/parser/parser.go`

**Discoveries**: Acceptance criteria 3 and 4 (cross-reference resolution within a module; order-independent evaluation) are only structurally verifiable at this point — actual decode-time interpolation across module-scoped resources depends on Phase 2.1's variable-passing/context-building work, not yet implemented. Verified instead that discovery and dependency-link scaffolding is correctly in place (resources found, FQRNs correct, DAG-ready) so Phase 2.1 has what it needs. `TestParseModuleCreatesResources` now fails only on `consul_3`'s remote source (expected, Phase 4.1 territory); `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` now fails on a decode error for `variable.disable_resources` inside the nested `module.sub`'s source files — confirming multi-level recursion works, decode is next.

### 2026-07-15 — Phase 2.1: Pass module-instance variables to child resources, with default fallback

**What was done**: Implemented module-instance variable passing across three files. `internal/resources/module.go`: retyped `Variables` from `any` to `hcl.Expression` (captures the raw `variables = {...}` expression via gohcl's documented special-case for `hcl.Expression`-typed fields, without attempting a fixed-type decode). `internal/parser/callbacks.go`: after the module vertex's own decode succeeds, evaluate `mod.Variables` against the now-available context and store the resulting `cty.Value` on `mod.SubContext`. `internal/parser/context.go`: completed the module-scoping branch to look up the owning module's `SubContext` and merge its values over `variableVars`; also added a `moduleVars` accumulator and `ctx.Variables["module"]` namespace so root-level (and other cross-module) references like `module.consul_1.output.foo` resolve correctly.

**Deviations**: Two real bugs were found and fixed beyond the plan's anticipated scope, both required for the target tests to pass, not "nice to have": (1) the plan's own research recommended retyping `Variables` to `map[string]cty.Value`, but this fails to decode any `variables` block with heterogeneous value types (e.g. a number and a bool in the same object) because `gocty.ImpliedType`'s map case forces one shared element type — switched to `hcl.Expression` instead, evaluated manually. (2) `internal/parser/exp.go`'s `processExpr` had no case for `hclsyntax.UnaryOpExpr`, so `disabled = !variable.enabled` never had `variable.enabled` extracted as a dependency link — this is a pre-existing, module-unrelated gap in the generic expression walker (confirmed via git-stash comparison to predate all module work), newly exposed because the module test fixtures are the first place this pattern is exercised in a way that surfaces the bug. Also fixed: `context.go`'s link lookup wasn't applying `AppendParentModule` before looking up a link's target resource, so any bare (module-unaware) reference from inside a module (e.g. `variable.cpu_resources`) failed to resolve — now mirrors the same `AppendParentModule` call `getResourceDependencies` already used for DAG edges.

**Files changed**:
- `internal/resources/module.go`
- `internal/parser/callbacks.go`
- `internal/parser/context.go`
- `internal/parser/exp.go`

**Discoveries**: Verified end-to-end via a temporary local-source copy of `modules.xcl` (consul_3's remote source swapped for a local one, deleted after use) — all three of `TestParseModuleCreatesOutputs`'s output assertions passed (4096 override-via-resource-reference, 512 override-via-variable-reference, 2048 default-fallback with no variables supplied). The real `modules.xcl`-based tests (`TestParseModuleCreatesResources`, `TestParseModuleCreatesOutputs`, `TestModuleDisabledCanBeOverriden`) still fail only on consul_3's remote source — confirmed this is exclusively Phase 4.1's remaining blocker for those three. `TestDoesNotLoadsVariablesFilesFromInsideModules` now passes fully. `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` now fails on a plain `require.True` assertion (disabled-state propagation), not a decode/lookup error — confirms it's now blocked purely on Phase 3.1, not on anything this phase owns.

### 2026-07-15 — Phase 2.2: Confirm nested modules work end-to-end

**What was done**: Verification-only phase, no production code changes. Confirmed via temporary scratch tests (created against the existing `disabled/module.xcl` nested fixture, deleted after use) that resources inside a nested `module "sub"` instance are discoverable and addressable by their dual-instance FQRN path (e.g. `module.disabled.sub.resource.container.enabled`), that two different outer instances' nested `sub` modules remain independently scoped, and that variable-passing into a nested module (`disabled_internal`'s `disable_resources = true` reaching its own child resource's `disabled = variable.disable_resources` expression) works identically to top-level variable passing.

**Deviations**: None. As the plan anticipated, nesting required no special-casing and no new code — the recursive `parseModule` (Phase 1.1/1.2) and the SubContext/context-merge mechanism (Phase 2.1) already generalize correctly to arbitrary nesting depth via the dot-joined module-instance-name convention.

**Files changed**: None.

**Discoveries**: `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled`'s remaining failure is confirmed to be exclusively Phase 3.1's disabled-propagation gap (a plain `require.True` assertion failure on a resource's disabled flag, not a lookup/decode error) — nesting mechanics themselves are already fully correct at this point.

### 2026-07-15 — Phase 3.1: Reflect a disabled module instance onto its resources

**What was done**: Threaded `currentState` (already a `state.State`, implementing `ResourceProvider`) through `walk`'s `walkCallback(...)` call site as a new `rp ResourceProvider` parameter. Completed the previously-commented-out propagation block in `walkCallback`: when a module vertex evaluates as disabled, call `rp.FindModuleResources(rMeta.ID, true)` to find all of its current resources (including nested sub-modules) and `types.SetDisabled(d, true)` on each.

**Deviations**: Minor - the dead code being replaced unconditionally `panic(err)`'d on any error from `FindModuleResources`; changed to ignore the error (`dr, _ := ...`) since a disabled module with zero remaining child resources legitimately returns a not-found error, which isn't a real failure - this mirrors the existing error-ignoring convention already used for the identical call in `util.go`'s `getResourceDependencies`. Also dropped the dead code's debug `fmt.Println`.

**Files changed**:
- `internal/parser/parser.go`
- `internal/parser/callbacks.go`

**Discoveries**: `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` now fully passes, confirming propagation works correctly through the nested `module.sub` fixture as well as the top-level case. While running a full-repo test sweep, confirmed (via a temporary worktree at the plan's baseline commit) that several other packages (`errors`, `example`, `internal/functions`, `plugins/registry`, repo root) have pre-existing, unrelated test failures that predate this entire plan - not something introduced by this work, and outside this plan's scope (its success metrics are specific to the parser package).

### 2026-07-15 — Phase 3.2: Let a resource inside a module override its own disabled state

**What was done**: Verification-only phase, no production code changes. Confirmed via a temporary local-source scratch copy of `modules.xcl` (consul_3's remote source swapped to local, deleted after use) that `module.consul_2.resource.container.sidecar` (consul_2 passes `enabled = true`) has `disabled=false`, while `module.consul_1.resource.container.sidecar` (no variables passed, module's own default `enabled=false`) has `disabled=true` - confirming a child resource's own `disabled = !variable.enabled` expression correctly reflects its own resolved variable value, independent of and not overridden by the module's own disabled state.

**Deviations**: None. As the plan anticipated, this override behavior already fell out of Phases 2.1 (variable passing) and 3.1 (disabled propagation only pre-setting `true`, never clearing an independently-`false` child) combined - no new code was needed.

**Files changed**: None.

**Discoveries**: `TestModuleDisabledCanBeOverriden` itself still fails only on `consul_3`'s remote source (confirmed exclusively Phase 4.1's remaining blocker) - its actual override-logic assertions (the ones this phase targets) are confirmed correct via the scratch verification above.

### 2026-07-15 — Phase 4.1: Align the module test fixture and test suite with the local-only scope

**What was done**: Replaced `modules.xcl`'s `consul_3` module instance's remote go-getter source (`github.com/jumppad-labs/xcl/test_fixtures//single`) with a local relative-path source (`../single`, matching `consul_1`/`consul_2`), keeping its `depends_on = ["module.consul_1"]` attribute to continue exercising module-to-module dependency resolution. Narrowed `TestParseModuleDoesNotCacheLocalFiles` in `parse_test.go` to drop the remote-fetch/cache-exists assertion, keeping only the local-source no-caching assertion (`require.NoDirExists(t, filepath.Join(p.options.ModuleCache, "single"))`).

**Deviations**: None from the plan's own described approach.

**Files changed**:
- `internal/test_fixtures/config/modules/modules.xcl`
- `internal/parser/parse_test.go`

**Discoveries**: All 6 target module tests now pass: `TestParseModuleCreatesResources`, `TestParseModuleDoesNotCacheLocalFiles` (narrowed), `TestParseModuleCreatesOutputs`, `TestDoesNotLoadsVariablesFilesFromInsideModules`, `TestModuleDisabledCanBeOverriden`, `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled`. Full parser package suite run: all tests pass except `TestParserProcessesResourcesInCorrectOrder`, which fails on a pre-existing mock-setup bug (`MockStateStore` missing an `.On("Exists")` expectation) unrelated to module logic - confirmed via a temporary `git worktree` comparison against this plan's baseline commit (`a908757`) that this exact failure, and the identical set of failures in several other unrelated packages (`errors`, `example`, `internal/functions`, `plugins/registry`, repo root), predate this entire plan's work. This is not a regression introduced by any phase of this plan - the parser package's module-related and previously-passing non-module tests all pass cleanly.

## Final status

All 4 milestones / 7 phases complete. Both of the plan's spec-level success metrics are met within the parser package's own scope: all 6 previously-failing module-related tests now pass, and the parser package has no regressions to previously-passing, non-module behavior. One pre-existing, out-of-scope test-infrastructure bug (`TestParserProcessesResourcesInCorrectOrder`'s mock setup) and several other pre-existing, unrelated package failures remain — confirmed via baseline-commit comparison to predate this plan entirely, not introduced by it, and outside this plan's scope to fix.
