# Working context: provider-lifecycle-read

## How we got here

Started from `TestParserEventCallback` failing: it called `p.Parse(false, ...)`,
and `executePlugins=false` skipped the provider lifecycle, so no events fired.
`executePlugins=false` existed only to back the old `Config.Validate` (which
called `Parse(false)`); `Config.Validate` now calls `Parser.Validate`, so the
flag was dead.

Already done in the working tree (not part of this spec, but context):
- Removed `executePlugins`; renamed `Parser.Parse` -> `Parser.Apply`; updated
  70 test call sites, `config.go`, docs.
- Added `requireEvent` test helper; split the event error test into
  `TestParserCreateEventErrorCallback` (passes) and
  `TestParserRefreshEventErrorCallback` (fails: previous state never reaches
  the walk).
- Rewrote the `ParserEvent` section of `docs/parser-lifecycle.md`; fixed
  `ParserEvent` field comments.
- Wrote `docs/plugin-developer-guide.md` (rough, "just for us") describing
  the agreed contract, with a "gaps vs code today" list.

## Problems found in current code

- `walk` hard-codes `previousParsed = nil` (TODO), so every resource takes the
  Create path on every Apply; Refresh/Changed/Update are unreachable.
- `Refresh` receives the *config* copy; its result is unmarshalled into `r`,
  but `Changed` is passed `resourceJSON` serialized *before* Refresh, so the
  refresh result never reaches `Changed`.
- A Refresh error is swallowed ("Continue even if refresh fails"). Introduced
  in commit ebd2672 "Refactor" (2026-02-04) with no stated reason; before it,
  refresh errors set status `failed` and aborted. Treated as an accident.
- Status values are inconsistent: `types/resource.go` documents
  pending/created/failed; code sets created/updated/failed/destroyed/
  destroy_failed/destroyed_failed; nothing sets `pending`; an unchanged
  `failed` resource stays failed forever and is never recreated (contradicts
  `plugins/provider.go` "Create ... recreates a failed resource").
- `destroyWalkCallback` is never called: no destroy walk runs.
- `internal/parser/plugins.go` `callPluginLifecycle` is dead duplicate code.
- Example provider's Refresh rewrites a config field (`Description`); its
  Changed comment calls it "drift detection".

## Purpose of Refresh: how the user got there

User had lost the original intent ("This conversation is sadly gone from my
memory"). Reasoned through examples:
- Computed values (postgres `connection_string`): user pointed out these are
  already written to state after Create/Update, so Refresh need not compute
  them. Copying them from state into the new resource needs a computed-field
  marker — user: "This is needed but is not our issue." (out of scope)
- File hash: name unchanged, contents changed -> must trigger Update.
- Container: "in the state the status is Running, but is it? Refresh would
  check things like that". User settled on: Refresh fills in the non-config
  fields of `new` so a flat diff works — "I am actually thinking Refresh
  should add computed fields to the new config so that a diff can be done".

Rejected: refreshing `old` instead of `new` — observed fields have no desired
value in config, so a flat diff of refreshed-old vs raw-new is meaningless
(always "changed" when running, never when stopped).

## Decisions

- Rename `Refresh` -> `Read`. Signature `Read(ctx, old, new) (T, error)`:
  `old` is used to locate the real resource (identity fields like IDs only
  exist on old); returns `new` with identity/observed/derived fields filled in.
  User: "I was thinking Refresh should be Read(new), but this assumes that you
  can infer the resource from the config alone ... I don't think you can."
  Options considered: Read(new) with parser copying identity from old (needs
  computed marker), Read(old) + parser merge (needs marker). User chose
  option 1: "I think option 1 is correct".
- Read must never write config fields, nor self-changing values (uptime,
  timestamps). Must not mutate the real resource.
- Read returns `ErrNotFound` when the resource is gone -> parser calls Create.
- Any other Read error is fatal; resource status `failed`.
- Read is only called when the resource exists in previous state (old never nil).
- `Changed(old, new)` receives `new` after Read. Default diff helper, overridable.
  User: "we can have a default diff helper, if you want to override that then
  you can. But in most cases as long as you put the work in Refresh you should
  not have to". Proposed shape: embeddable `plugins.DefaultChanged[T]` doing a
  flat diff ignoring `Meta` (Go has no optional interface methods).
- `Update(new)` returns resource with observed fields set to new reality.
- Parser must pass loaded previous state into `walk`.
- Statuses: created, updated, failed, destroyed, destroy_failed.
- Event operation `refresh` becomes `read`.
- Document the concepts in a plugin developers guide — user: "keep it rough
  for now just for us".

## Interview answers

- Failed resources: "If Create or Update fails and it is marked as failed, it
  should be put into destroy before create" -> next Apply does Destroy(old)
  then Create(new).
- Breaking the provider interface / gRPC proto is fine (v2); no Refresh shim.
- Status clean-up is in this spec.
- Saving state when Apply fails is in this spec (found: today a failed Apply
  saves nothing, so created resources are forgotten).
- Destroy of a missing resource: provider decides; parser treats Destroy
  errors as fatal.

## Review-round decisions

- Computed marker is now IN scope (reverses earlier "not our issue"): user
  "we need computed as a tag". Computed fields are optional by default,
  provider may leave them unset; users setting one is a validation error.
- Parser carries computed values from saved state onto the configured
  resource before Read — user: "even if read has no custom logic we can
  correctly detect changed with default logic. Read is still necessary as
  some fields are only available with a manual lookup."
- Provider changing a non-computed value: allowed but warned (log warning),
  checked after create/read/update; fields set by reference are exempt.
  User: "mark this as a warning rather than a hard error as a compromise".
  Unset non-computed optional fields filled by a provider also warn.
- Failure: dependents skipped, independent resources complete and are saved;
  unreached new resources are not saved.
- Rebuild whose destroy fails -> `destroy_failed`, no create, retried next apply.
- Old saved state need not load.

## Out of scope (explicitly)

- Running the destroy walk.
- Exposing `OnParserEvent` through `Config`.
- Diff (plan) output — user: "we can add a Diff later".


## Plan workflow (started 2026-09-19)

- Planning against spec `20260918165700-provider-lifecycle-read` (user choice).
- Single repo: xclconfig, root `/home/nicj/code/github.com/jumppad-labs/xclconfig`.
- Discovery done. Key learnings: jumppad-labs/dag walker already skips dependents & runs independents in parallel (record outcomes in callback w/ mutex); `computed` must be separate tag key `xcl:"computed"` (gohcl panics on unknown hcl options); schema preserves raw tags so no proto change for computed; gRPC errors are in-band strings → add `not_found` bool to ReadResponse; Parser.Apply/Config.Apply drop state on error.
- Baseline test failures pre-exist (errors pkg, root utils fixtures, build failures in ./example, internal/functions, plugins/registry tests) — treated as out of scope (see assumptions.md).
- Architecture chosen: `internal/parser/lifecycle.go` resourceLifecycle + applyProgress (mutex); Parser.Apply returns partial state+error; Config.Apply saves non-nil state on error; ErrNotFound via proto `not_found`; DefaultChanged ignores meta/depends_on/disabled; `xcl:"computed"` top-level only; keep destroyWalkCallback (fix statuses), delete internal/parser/plugins.go.
- Components drafted.
- Data structures drafted (DefaultChanged value receiver struct; applyProgress/outcome; Apply returns partial state+err on walk failure).
- Implementation detail drafted.
- Dependencies drafted.
- Testing approach drafted: scenario tests use real FileStateStore in t.TempDir().
- Milestones: M1 contract, M2 repeat-apply lifecycle, M3 failed-apply progress, M4 computed + guide.
- Phases drafted (9 phases: 1.1-1.3, 2.1-2.2, 3.1, 4.1-4.3). New files planned: plugins/errors.go, plugins/changed.go, types/status.go, internal/parser/{lifecycle,computed,configured_check}.go, fixtures config/lifecycle, config/computed_set.
- Open questions: only DefaultChanged null/empty normalisation risk. Out of scope drafted.
- Assembled & staged plan/context/research to .spektacular/tmp/*_template.md.
- Verification passed (context section order fixed; removed make command from plan.md).
- plan.md written.
- context.md written.
- research.md written; work dir removed. Next: walkthrough (read committed docs via plan file read).
- Walkthrough: user asked about xcl-only tags (generate hcl/json via reflection at schema rebuild). Decision: follow-up spec; added to plan Out of Scope.
- User direction: fork HCL fully into an XCL-owned parser with custom xcl tags (accept no upstream updates; may rewrite later). Recorded in plan Out of Scope as follow-up spec.
- Knowledge written: decisions/own-hcl-fork-with-xcl-tags.md. Walkthrough now at beat 4 (drafting assumptions).
- Walkthrough change: all scenario test state comes from real prior applies (no hand-built state). Rebuild moved from Phase 2.2 to Phase 3.2 (after save-on-failure 3.1); read-failure 'saved as failed' check moved to 3.1. M3 renamed.
- Walkthrough change: computed fields at any depth (nested/pointer/list/map blocks); list elements paired by xcl:"key" fields else position; maps by key. Phase 4.1 now High. Based on jumppad container Image.ID / Networks[].AssignedAddress pattern.
- Knowledge written: conventions/test-state-from-real-apply.md. User signed off on the plan (2026-09-19).

## Implementation session (2026-09-19)

- Implement workflow started for plan `20260918165700-provider-lifecycle-read`
  (user chose it). HEAD = plan commit 2daa13f, so no drift; only line numbers
  in context.md are approximate (e.g. `adapter.go` Refresh is at ~138, not 223).
- Spec coverage check: every requirement and acceptance criterion is covered;
  nothing descoped. First-phase run (no `## Changelog` in plan.md yet).
- Tooling present: mockery v3.5.5, protoc, protoc-gen-go, protoc-gen-go-grpc.
- Phase 1.1 analysis: all referenced symbols present. gRPC wrapper keeps the old
  RefreshRequest on the wire during 1.1 (sends only new data) until 1.2 changes
  the proto. logger already imports types, so the printer can use the status
  constants.
- Phase 1.1 implement:
  - Deleted dead `internal/parser/plugins.go` in 1.1, not 1.3, because it
    called `adapter.Refresh` and the module has to build at the end of every phase.
  - Minimal renames in the example provider and test plugin (signature only);
    their real rework is still Phase 1.3.
  - Mocks were regenerated in 1.1, not 1.2. **Mockery gotcha:** the installed
    mockery v3.5.5 (built with go1.25) fails under go1.27 with "package context
    without types". `go run ...@v3.5.5` fails too (old x/tools). What works:
    `PATH=$(go env GOMODCACHE)/golang.org/toolchain@v0.0.1-go1.25.6.linux-amd64/bin:$PATH GOTOOLCHAIN=local mockery`
  - Pretty printer: created/updated → green ✅, destroyed → yellow 🟡,
    failed/destroy_failed → red ❌; `pending` removed.
- Phase 1.1 test: added `plugins/changed_test.go` and `plugins/adapter_test.go`.
  These are the first tests in `plugins`, so `go test` now runs vet there. To
  keep that working, the `fmt.Errorf(resp.Error)` → `errors.New` fix was
  pulled forward from 1.2.
- Pre-existing parser failures on the baseline: `TestDestroyLifecycle` (panics;
  replaced in 3.2), `TestParserReadEventErrorCallback` (passes in 2.1) and
  **`TestPluginResourceCreationWithFallback` (panics on a nil pluginRegistry; checked
  at HEAD 2daa13f; not in the plan's baseline list)**. Run parser tests with
  `-skip 'TestDestroyLifecycle|TestPluginResourceCreationWithFallback'`.
- **User instruction (after 1.1): "Just finish all phases"**, so loop through every
  phase without asking between them. The knowledge offer (mockery gotcha) got no
  answer; don't write it.
- Phase 2.1 implement: `internal/parser/lifecycle.go` holds `resourceLifecycle`
  (paths: create, read→changed→update; any other previous status → create
  until 3.2 adds rebuild). `callProvider` wraps every provider call with events
  and errors `"<op> failed for <id>: …"`. `walkCallback(parsed, rp, lifecycle,
  options, functions)`; `callProviderLifecycle` deleted.
- The test plugin has a mutex and embeds `DefaultChanged`, with settings for
  ReadNotFound, ReadObserved, ChangedResults, ReadCalls and Calls ("<op> <id>").
  CreateSetsID (on by default) sets the Network `ProviderID="id-<name>"`.
  `ResetCalls()` clears the recording only.
- `-race` on internal/parser already reports races at the base commit (80), all
  from test OnParserEvent closures appending to slices. Not ours.
- All phases 1.1–4.3 implemented, verified and recorded in the plan changelog.
  Test plan: none required (all three success metrics are covered by tests).
- Spec reconciled: 44 criteria checked. "Test suite passes" left unchecked: every
  touched package passes, but older failures remain in untouched packages
  (errors, root file-location tests, example/internal/functions/plugins/registry
  test builds, TestPluginResourceCreationWithFallback).
- The tracked binary `plugins/example/build/example` is rebuilt by the e2e tests;
  it was restored with git checkout after the final run.
