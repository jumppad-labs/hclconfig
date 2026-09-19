# Working context: destroy-cycle

## Problem and motivation

User asked "has the destroy cycle been implemented?" after the
config-only-types work. Findings from the code (2026-09-19):

- `Config.Destroy` (`config.go:139`) is a stub: `// TODO: Implement destroy
  logic`, returns nil. A caller gets success and believes everything was torn
  down.
- Apply drops blocks removed from configuration: they are left out of the new
  state and the provider's Destroy is never called, so real resources are
  orphaned and xcl loses track of them.
- `destroyWalkCallback` (`internal/parser/callbacks.go:183`) exists: calls the
  provider's Destroy, fires `destroy` start/success/error events, sets
  `destroyed`/`destroy_failed`, skips builtin and registered types (no
  provider). Nothing calls it: no `Parser.Destroy`, no reverse DAG walk. Only
  caller is a registered-types unit test.
- The only destroy that runs today is in `resourceLifecycle.rebuild`
  (`lifecycle.go:233`): a resource saved as failed/destroy_failed is destroyed
  from its saved copy, then created again.

## What the user agreed should exist (my summary, user said "let's create a spec")

- `Config.Destroy`: reverse dependency walk over the current state
  (dependents before what they depend on), save state after each resource so
  an interrupted destroy resumes, failures left as `destroy_failed`.
- `Apply`: destroy resources in the previous state that are no longer in the
  configuration, in reverse dependency order.
- Same `destroy` events and logging as everything else.

## Interview answers (user)

- Apply destroys removed resources first, before creates/updates.
- Ordering, user's words: "destroy should destroy things that depend on it,
  not things it depends on. We should never touch things it depends on".
- Destroy failure: resource kept as destroy_failed, its dependencies untouched,
  unrelated resources still destroyed.
- Config.Destroy works from saved state only, no paths.
- After destroy only destroy_failed resources stay in state.
- Apply: a failed destroy of a removed resource stops the apply before any
  create/update.
- Spec name: 20260919152915-destroy-cycle. Interview written to
  .spektacular/work/20260919152915-destroy-cycle/interview.md.

## Conventions / decisions from this session that apply

- Log level is importance, not who wrote it: everything except errors at
  debug (user: "Honestly other than the Error, all of this should be DEBUG").
- Events: `xcl.WithEventHandler` / `xcl.Event` exist; operations parse,
  create, read, changed, update, destroy; builtin/registered types only get
  success (no provider).
- Registered types (`PluginRegistry.RegisterType`) have no provider and must
  never be passed to a provider, including on destroy.
- Never commit binaries; tests build any binary they need.
- Test conventions: testify require, no table-driven tests, positive and
  negative in separate functions, state for re-apply tests from a real apply.
- Breaking API changes are acceptable (library unreleased).
- Requirements agreed. User: "we should destroy in order, basically the same graph but backwards" -> Technical Approach: reuse the dependency graph, walked in reverse.
- User: "we should never destroy a parent when the child fails" (parent = depended-on, child = dependent). Added to Constraints and Success Metrics.
- Non-goals agreed. User: applying an empty config to remove everything is not a goal, "Destroy does that".
- Review triage answers: "children should store their parents in the state" -> constraint; examples apply+destroy -> requirement + AC; applying with no blocks errors -> requirement + AC (non-goal removed). Terminology unified to "saved state" (Overview keeps "record").
- Spec committed to store (14 requirements, 16 acceptance criteria). Old state without parents: no guarantee, not mentioned in spec. Next: plan workflow.

## Plan workflow (started 2026-09-19)

- Plan name: 20260919152915-destroy-cycle. Section working files in
  .spektacular/work/20260919152915-destroy-cycle/.
- Overview step done: spec read (14 requirements, 16 ACs, 3 success metrics
  that must become tests in the Testing Approach step).
- Discovery done (research.md written). Key learnings: destroyWalkCallback +
  buildDestroyDAG exist unused; buildDestroyDAG exact-matches unresolved
  depends_on strings (useless); dag.Walker has Reverse option with upstream
  failure cascade; saved state has no resolved parents; state saved once per
  apply; examples have no state store; FileStateStore.Load silently drops
  unknown types.
- User decision (plan discovery): FileStateStore.Load must FAIL on unknown
  types (names them), so Destroy and Apply stop before touching anything.
- DoYouLikeDags name/comment must be kept (author's note in dag.go).
- Architecture done: single parser `destroyer` (internal/parser/destroy.go)
  used by Parser.Destroy and Parser.Apply removals; graph parent->child from
  new Meta.Parents, walked Reverse=true; per-resource save via StateStore;
  destroyWalkCallback reworked (not duplicated).
- Components step done.
- Data structures done: Meta.Parents, Parser.Destroy(saved)->(state,err), destroyer, state.UnknownTypesError, parser.ErrEmptyConfiguration.
- Implementation detail done.
- Dependencies done.
- Testing approach done (metrics: orphan check + parent-after-failed-child check for destroy and removal; no-regression via suite+vet+unchanged re-apply).
- Milestones done: M1 Destroy, M2 removal in Apply + empty config error, M3 examples + docs.
- Phases done: 1.1 parents+Load guard, 1.2 destroyer+Parser.Destroy, 1.3 Config.Destroy, 2.1 removal in Apply+empty config, 3.1 examples, 3.2 docs.
- Open questions done (one: tests relying on Load skipping unknown types).
- Out of scope done.
- Assembled and staged plan/context/research to .spektacular/tmp/.
- Verification passed (added Project References to context, removed shell wording from plan).
- All three plan docs committed to store; working dir removed. Now in walkthrough (read docs back via spektacular plan file read).
- Walkthrough complete: user signed off the plan with no changes (2026-09-19).

## Implement session (2026-09-19)

- Implementing plan `20260919152915-destroy-cycle` (user picked it).
- read_plan: structure OK, all spec items covered, drift only line-number
  shifts (e.g. `Config.Destroy` at config.go:139 not 136; TestPlugin has
  `GetCreatedResources()` method / `CreatedResources` field). No `## Changelog`
  in plan.md yet → first-phase run, start at Phase 1.1.
- 1.1 analysis: only non-test Load caller is parser.go:267 (parseAndValidate);
  no test relies on the unknown-type skip. registered/basic has a module
  (module.shared) but no module-wide `depends_on`, so a new fixture is needed
  for the module-wide-reference test. `network.independent` references a
  variable, so its parents include `variable.independent_subnet` (the plan's
  "independent → none" is approximate).
- 1.1 code: `Meta.Parents` added; buildCreateDAG records sorted non-nil parent
  IDs and now uses a `connected` flag (not len(deps)) for the root edge, so a
  resource whose only deps are nil hangs off root. Load returns
  `state.UnknownTypesError`. Many parser files were already not gofmt-clean
  before this work — don't reformat unrelated files.
- 1.1 tests: `internal/parser/parents_test.go`, new fixture
  `internal/test_fixtures/config/lifecycle/module_reference/` (module
  "networks" with network.one/two; network.consumer depends_on module;
  network.alone), two Load tests in `state/file_state_store_test.go`. Adding a
  Meta field requires updating the golden schema
  `internal/schema/test_fixtures/embedded.go` (EmbeddedJson) — done for Parents.
  Module-wide ref parents = the module's resources, not the module itself.
- 1.1 verify: all green (build, vet, 6 new tests, full suite, gofmt).
- 1.1 complete: plan ticked, changelog section created. Next: Phase 1.2 (High complexity).
- User said 'keep going': run remaining phases without pausing between them. Saved gotcha gotchas/meta-field-golden-schema.md.
- 1.2 analysis: walker Reverse makes edge Source wait on Target, so
  parent→child edges + Reverse = children first; root→target edges make root
  run last. `State.GetResources` returns the live slice and RemoveResource
  mutates it — always copy before iterating. Did analysis in main context.
- 1.2 code: `internal/parser/destroy.go` (destroyer), `buildDestroyDAG` rebuilt
  from Meta.Parents, `destroyWalkCallback(d *destroyer)` (disabled skip,
  failedToDestroy/destroyed + save), `Parser.Destroy(saved)`; legacy
  commented tests removed from parse_test.go; registered destroy test adapted.
- 1.2 tests: internal/parser/destroy_test.go (+ fixture lifecycle/disabled/disabled.xcl); passes -race -count=5.
- 1.2 complete (plan ticked, changelog added). Next: Phase 1.3 Config.Destroy.
- 1.3 code: Config.Destroy implemented (Exists false → nil, Load error wrapped 'failed to load state', empty → nil); event docs in events.go/options.go mention Destroy.
- 1.3 tests: config_destroy_test.go (14 tests). Criterion 5 only partly observable at Config level (no logging in destroy path); examples in 3.1 cover it.
- 1.3 complete. Next: Phase 2.1 removal during Apply + empty config.
- 2.1 code: ErrEmptyConfiguration in new internal/parser/errors.go (stdlib errors, avoids xcl/errors clash); Apply removal phase + removedResources helper; config.go Apply doc; fixtures lifecycle/removal/{before,without_x,without_x_with_z,without_pq} and config/empty/empty.xcl.
- 2.1 tests: internal/parser/removal_test.go, extended registered_types_test.go, TestApplyRejectsEmptyConfigurationAndChangesNothing in config_destroy_test.go.
- 2.1 complete. Next: 3.1 examples, then 3.2 docs.
- 3.1 code: both examples take statePath, use FileStateStore, Destroy after printing, return applied resources; main uses os.MkdirTemp. Manual runs: destroy events DEBU, 0 remaining.
- 3.1 tests written (renamed to file prefixes). 3.2 docs agent started in parallel during 3.1 test step.
- 3.1 complete. Next: 3.2 docs (agent already running).
- 3.2: docs written by agent. Re-exported xcl.ErrEmptyConfiguration (parser is internal, apps can't import it); README/docs/Config test use it.
- 3.2 complete; all phases done. Next: test_plan.
- test_plan: none required (all 3 metrics automated).
- feature changelog: project + repo records written; CHANGELOG.md entry prepended.
- reconcile_spec: all 14 requirements + 16 ACs ticked.
