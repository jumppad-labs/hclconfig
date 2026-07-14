# Context: Restore Module Support in v2 Parser

## Problem identified and why it needs solving

While debugging why the Go test window showed "Buffered Logs (test failed)" with no
visible error content, we traced a chain of investigation:

1. Root cause of the *display* issue: `logger/test_logger.go`'s `flushIfFailed()`
   printed the `=== Buffered Logs ===` / `=== End Buffered Logs ===` markers even when
   the log buffer was empty, producing an empty-looking section in the IDE's collapsed
   test output view. This was fixed separately (guard added: `l.t.Failed() && len(l.buffer) > 0`).
2. Investigating the underlying failing tests (not a display bug) revealed that ALL
   module-related tests in `internal/parser/parse_test.go` fail with
   `resource not found: module.X.resource...` errors:
   - TestParseModuleCreatesResources
   - TestParseModuleDoesNotCacheLocalFiles
   - TestParseModuleCreatesOutputs
   - TestDoesNotLoadsVariablesFilesFromInsideModules
   - TestModuleDisabledCanBeOverriden
   - TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled
3. Root cause: `internal/parser/parser.go` has a stub for module blocks:
   ```go
   case resources.TypeModule:
       // Module support is not yet implemented in v2
       continue
       //err := p.parseModule(file, b)
   ```
   Module blocks are silently skipped during parsing; their child resources never get
   registered into state at all.

## History / why this happened (not a regression, a mid-refactor gap)

- The pre-refactor (v1) architecture had a real, working `parseModule` at commit
  `3cf707c` ("Working end to end with both built in plugins and grpc plugins, state
  broken, needs tidy"), in the (then root-level) `parser.go`. It decoded the module
  block **inline** during a single-pass parse (calling `decodeBody` immediately), then
  recursed into the module's child files right away, tagging child resources with
  `meta.Module = moduleName` for scoping.
- Between `3cf707c` and `ebd2672` ("Refactor"), the parser was restructured into a
  **two-phase model**, documented in
  `.spektacular/knowledge/decisions/parser-refactor-analysis.md`:
  - **Phase 1 (parse)**: build resource "shells" (zero-value structs) and store their
    raw HCL bodies in an internal `parsed` struct (`resources map[string]any`,
    `bodies map[string]*hclsyntax.Body`), keyed by FQRN. No decoding happens yet.
  - **Phase 2 (DAG walk)**: build a dependency DAG from extracted `Links`, walk it in
    topological order, and decode each shell's stored HCL body via
    `gohcl.DecodeBody(body, ctx, resource)` using a context built from
    already-decoded dependencies (`internal/parser/callbacks.go`,
    `internal/parser/context.go`).
  - Rationale: file order doesn't guarantee dependency order; a resource that
    references another resource/variable via interpolation must be decoded only
    after that dependency is already decoded. The DAG guarantees this.
- The old single-pass `parseModule` doesn't fit this: it decoded eagerly and recursed
  immediately, rather than deferring to the DAG walk. Rather than port it, whoever did
  the refactor left the `continue` stub and commented-out call as a placeholder.
- The refactor-analysis doc explicitly lists module support under
  **"Future (Module Support)"** with an unchecked checklist:
  - [ ] Implement module parsing
  - [ ] Implement module variable passing
  - [ ] Implement module resource scoping
  - [ ] Add module dependency resolution
  This confirms it's tracked, deliberate, deferred work — not an oversight or
  regression to "fix"; it needs to be designed fresh against the new two-phase model.
- The `resources.Module` type itself (`internal/resources/module.go`) survived the
  refactor untouched: `Source string`, `Version string` (optional), `Variables any`
  (optional), and a `SubContext *hcl.EvalContext` field explicitly intended "to store
  the variables as a context that can be passed to child resources" — this field
  exists but nothing populates or consumes it yet.
- FQRN handling for the `module` type prefix already exists and works
  (`internal/resources/fqrn.go` references `TypeModule` for parsing/building
  `module.<name>.resource.<type>.<name>` style paths) — only the parse-time wiring
  that populates a module's child resources into state is missing.

## What "done" looks like — acceptance criteria source of truth

The six currently-failing tests in `internal/parser/parse_test.go`, exercised against
existing fixtures, ARE the acceptance criteria:
- `internal/test_fixtures/config/modules/*.xcl` (fixtures reference `consul_1`,
  `consul_2`, `consul_3` module instances, plus a disabled/override scenario)
- `internal/test_fixtures/config/single/container.xcl` (the module source content:
  variables `cpu_resources` default 2048, `enabled` default false; resources
  `network.onprem`, `container.consul`, `container.sidecar`; several outputs)

Key behaviors the spec must define, driven by these fixtures:
1. **Discovery/parsing** of `module` blocks alongside (not instead of) the existing
   shell/DAG-decode model — i.e., modules need to fit into Phase 1 (shell creation)
   and Phase 2 (DAG decode) without reintroducing eager/inline decoding.
2. **FQRN scoping** — child resources of a module must be registered under
   `module.<name>.resource.<type>.<name>` (confirmed via test lookups like
   `module.consul_1.resource.container.consul`).
3. **Source resolution** — at minimum local relative paths (`source = "../single"`
   in the fixture). Whether remote/git sources (as `getter.go` / `internal/getter`
   might have supported pre-refactor) are in scope or explicitly deferred needs to be
   decided in the spec — lean toward deferring remote sources unless the user wants
   them in scope, since none of the current fixtures/tests exercise them.
4. **Variable passing** — module blocks can set `variables = { ... }` (see
   `resources.Module.Variables any`) which must flow into the child module's
   variable evaluation context (precedence: child defaults < parent-supplied
   variables < any other layers already established elsewhere in the precedence
   chain — confirm exact ordering against `context.go`'s existing precedence notes:
   "defaults < files < env < direct" per the refactor-analysis doc).
5. **Multiple/repeated module instances** — same source used multiple times with
   different instance names (`consul_1`, `consul_2`, `consul_3`), each producing
   independently-scoped resources; this must not cause caching/aliasing bugs between
   instances (see `TestParseModuleDoesNotCacheLocalFiles` — the "does not cache" test
   name suggests there was a known bug class here previously worth guarding against).
6. **Disabled propagation/override** — `TestModuleDisabledCanBeOverriden` and
   `TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled` show that when a
   module is disabled, its child resources should not be processed, but a child
   resource's own disabled state can still be independently overridden — the exact
   interaction rules need to be spelled out precisely in the spec (look at fixture
   `modules.xcl` and `single/container.xcl`'s `sidecar` resource which has
   `disabled = !variable.enabled` for the override mechanism example).
7. **Module outputs** — `TestParseModuleCreatesOutputs` implies module-scoped
   `output` blocks must also be discoverable/queryable, likely also FQRN-scoped
   under the module.

## Constraints / non-negotiables carried into the spec

- Must NOT reintroduce inline/eager decoding — everything must go through the
  existing two-phase shell-then-DAG-decode pipeline, since that model was
  deliberately chosen to fix interpolation-ordering bugs that inline decoding could
  not solve correctly.
- Must not regress the currently-passing non-module tests in `parse_test.go`.
- Should reuse existing DAG/Link/FQRN infrastructure rather than inventing parallel
  mechanisms, per the project's general "prefer composition, avoid new abstractions
  beyond what's needed" Go guidelines in CLAUDE.md.

## Overview step — resolved

Overview draft was presented to the user and accepted as-is (no changes):
"xclconfig configurations can currently only be authored as a single flat set of
resources. This feature restores the ability to package and reuse configuration as
modules — self-contained, named units of configuration that can be instantiated
multiple times with different variables — so users can share and reuse common
infrastructure patterns (for example, a standard container setup) across multiple
parts of a configuration without duplicating it."
Saved to `.spektacular/work/20260714080036-restore-module-support/overview.md`.

## Requirements step — resolved

Two scope decisions confirmed with user (both recommended defaults accepted):
1. **Source scope**: local relative-path sources only. Remote/git sources are
   explicitly out of scope for this spec — could be a future follow-up spec.
2. **Disabled semantics**: a disabled module's child resources still exist in state
   (visible but inert/flagged disabled), not fully suppressed/unparsed. This allows
   a child resource to still independently override its own disabled state via its
   own variables, consistent with the `single/container.xcl` fixture pattern
   (`sidecar` resource: `disabled = !variable.enabled`).

Requirements captured (11 items) saved to
`.spektacular/work/20260714080036-restore-module-support/requirements.md`, covering:
module definition, multi-instantiation, per-instance resource scoping, variable
passing with default fallback, local source resolution, disable semantics (module +
child override), cross-boundary interpolation, module outputs, and dependency-order
correctness. These map directly to the 6 failing tests identified earlier
(TestParseModuleCreatesResources, TestParseModuleDoesNotCacheLocalFiles,
TestParseModuleCreatesOutputs, TestDoesNotLoadsVariablesFilesFromInsideModules,
TestModuleDisabledCanBeOverriden, TestParseDoesNotProcessDisabledResourcesWhenModuleDisabled).

## Acceptance Criteria step — resolved

11 black-box acceptance criteria drafted (one per requirement), derived from the
existing failing tests/fixtures behavior, and confirmed as-is by the user without
edits. Saved to
`.spektacular/work/20260714080036-restore-module-support/acceptance_criteria.md`.

## Constraints step — resolved

User confirmed 5 hard constraints (all recommended options accepted):
1. Must decode module resources via the existing DAG-ordered walk phase, not
   eagerly/inline during parsing (core reason old parseModule can't be reused as-is).
2. Must not regress currently-passing non-module parser tests.
3. Must not change the on-disk/serialized State JSON format.
4. No new external dependencies (must use what's already in go.mod).
5. Local relative-path sources only (remote/git out of scope — restated from
   requirements step as a hard boundary too).
Saved to `.spektacular/work/20260714080036-restore-module-support/constraints.md`.

## Technical Approach step — resolved

User confirmed one non-binding steer: reuse existing DAG/FQRN/Link infrastructure
rather than build parallel mechanisms for modules. Also noted this is a restoration
of prior behavior redesigned for the current architecture, not a net-new feature.
No further technical direction — left open for the plan workflow. Saved to
`.spektacular/work/20260714080036-restore-module-support/technical_approach.md`.

## Success Metrics step — resolved

User confirmed test-suite passage is sufficient as the success metric (no additional
performance/usage metrics wanted). Saved to
`.spektacular/work/20260714080036-restore-module-support/success_metrics.md`.

## Non-Goals step — resolved (IMPORTANT correction made mid-step)

Initially proposed "nested modules out of scope" as a candidate non-goal — user
CORRECTED this: nested modules (module containing another module) MUST be supported,
matching prior behavior. This was NOT a non-goal; it was a missed requirement.
Corrective action taken: added a new requirement ("Support nested modules") to
`requirements.md` and a new acceptance criterion ("Resources inside a nested module
are correctly scoped and retrievable") to `acceptance_criteria.md`, both appended
after their respective original lists.

Confirmed actual non-goals (2, both recommended accepted):
1. Module version/update management is out of scope.
2. Provider/plugin lifecycle execution changes are out of scope (parsing/state
   population only — this work does not touch Create/Update execution behavior).
Saved to `.spektacular/work/20260714080036-restore-module-support/non_goals.md`.

**Carry-forward note for plan workflow**: nested modules are IN scope and must be
designed for — do not treat module support as flat/single-level only.

## Verification step — resolved

Assembled spec staged to `.spektacular/tmp/spec_template.md`. Fresh-context reviewer
subagent flagged 2 findings:
1. Requirements/Constraints overlap on local-path scope — triaged as NOT duplication
   (capability vs. boundary framing is expected), left as-is per user confirmation.
2. Technical Approach restating the parse-then-decode hard rule already in
   Constraints — confirmed genuine duplication, trimmed from Technical Approach in
   both the working file and staged spec (kept the non-binding "restoration vs.
   net-new" framing, removed the "design must fit... not reproduce" clause).
Ready to commit to spec store via `spektacular spec file write`.

## User's exact framing

User said: "ok, we should probably make a specification to restore this if is going
to be substantial" — confirming: (a) this is being treated as a *restoration* of
previously-working functionality re-architected for the new model, not a brand new
feature; (b) the user wants a spec-first approach specifically because of the
estimated size/complexity (touches DAG construction, resource scoping, variable
context passing); (c) no timeline/deadline was stated.
