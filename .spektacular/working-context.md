# Working Context

Implement workflow: `20260918075104-validation-phase-before-walk`
Repo: `xclconfig` (root `/home/nicj/code/github.com/jumppad-labs/xclconfig`), branch `v2`.

## Validation gate (read_plan) — outcome

Structural validation **passed**: all 10 required `## ` sections present, 12
phases (3 per milestone × 4), every `*Technical detail:*` link anchor resolves to
a matching `### Phase N.M:` heading in `context.md`.

Changelog mode: **first-phase invocation** — no `## Changelog` section exists in
`plan.md` yet, so `update_changelog` creates it on first use. Pick up at Phase 1.1.

Spec coverage: **complete**. All 14 requirements and 21 acceptance criteria map to
phases; no gaps, nothing descoped.

## Drift check — two stale path references, user chose to proceed

Every symbol, type, function and line anchor named in the plan was verified present
and **exact** — `Parse:182`, `parseResourcesInFile:284`, `parseModule:565`,
`getUniqueResourceLinks:697`, `getDependentResources:741`, `walk:883`,
`checkIfErrorInFunction:923`, `parsed:39`, the silent fall-through at `parser.go:788`,
`fqrn.go:53/167/205`, `exp.go:22/145`, the `errors` and `config.go`/`diff.go` symbols,
and every cited `parse_test.go` line. The `hcl:"rm,remain"` landmine at
`internal/test_fixtures/plugin/structs/container.go:14` is present as described, as
are the nine `hcl:",remain"` embeds and the untagged `Meta.Properties/Links/Status`.

Two **path** references are stale (content is fine, location differs):

1. **`hclparse/parser.go:68`** — cited for `ParseXCLFile`. No such repo-local file.
   `ParseXCLFile` comes from the **external** patched `github.com/hashicorp/hcl/v2/hclparse`
   (imported at `internal/parser/parser.go:13`, used at `:287`). The plan's *reasoning*
   is unaffected — it builds an AST and never evaluates, which is why the
   `"Error in function call"` promotion branch is dead by construction.
2. **`internal/test_fixtures/config/functions/default.xcl`** — actual path is
   **`internal/test_fixtures/functions/default.xcl`** (no `config/` segment). Note
   `internal/functions/functions_test.go:182` still points at `default.hcl`, part of the
   known `.hcl`/`.xcl` fixture drift.

## Baseline — reproduced exactly, matches the plan

`go build ./...` clean. `go test ./...`:

- **3 packages fail to build**: `internal/functions`, `plugins/registry`, `example`.
- **10 failing tests**, exactly the plan's table:
  - *In scope (this work fixes)*: `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed`,
    `TestParseFileReturnsConfigErrorWhenInvalidFileFails`
  - *Must be decided, not assumed*: `TestParserEventCallback`,
    `TestParserEventErrorCallback`, `TestParserEventForVariablesOutputsLocals`,
    `TestParserErrorHighlightsLine`, `TestParserErrorNonErrorLineGrey`
  - *Out of scope*: `TestDestroyLifecycle` (panics — mock `StateStore.Exists` has no
    expectation), `TestReadResourceFromFileAtLocation`, `TestLineFromFileAtLocation`

**"All tests pass" is not a usable exit criterion** — Phase 4.3 measures against this table.

## Phase 1.1 analysis — probed facts (measured, not assumed)

Probed HCL diagnostic counts directly (throwaway test, since removed):

| fixture | diagnostics | note |
|---|---|---|
| `process_error/bad_format.xcl` | **1** | `Invalid block definition` @ 7,30 |
| `process_error/bad_interpolation.xcl` | **0** | no parse diags — it is a *stage 3* case |
| `process_error/function_error.xcl` | **0** | no parse diags |
| `invalid/no_name.xcl`, `no_resource.xcl`, `no_type.xcl` | **0** each | errors come from the **block loop**, not the diag path |
| unterminated string (synthetic) | **3** | 2 discarded today — confirms the plan's measurement |
| unclosed `${` (synthetic) | **2** | |

**Decided test counts** (derived from the above, *not* read back from a run):

- `parse_test.go:915` `bad_format.xcl` → stays **1**. The file yields exactly one
  diagnostic, so per-diagnostic iteration cannot change it. Deliberate, not unexamined.
- `parse_test.go:871` `config/invalid/` → stays **3**. Three files, one bad block each,
  all from the block loop; one error per file.
- `parse_test.go:885` `config/process_error/` → currently asserts **1** over 3 files.
  `bad_format` gives 1 parse error; `bad_interpolation` and `function_error` produce no
  *parse* diagnostics, so at Phase 1.1 this legitimately remains 1. Rewrite to assert on
  **content** (that `bad_format.xcl` is represented) rather than a bare count, per the
  plan's assertion discipline. It rises once stages 2/3 land.

**New fixture design constraint:** HCL halts on a malformed *block header* — a synthetic
file with two bad headers still yielded only 1 diagnostic. So the "two malformations in one
file" fixture must use two **recoverable** faults (e.g. two unterminated strings inside
otherwise well-formed blocks) to actually prove per-diagnostic accumulation.

**`NewParserErrorFromHCLDiag` needs no extension** — contrary to the context.md "check and
extend if needed" note, it already sets Line/Column from `diag.Subject` and hardcodes
`ParserErrorLevelError` (`errors/parser_error.go:162-177`). Reuse it as-is. Its only other
call site is `internal/parser/util.go:131`, which also reads only `diag[0]`.

**Caller loop needs no change** — `parser.go:249-254` already accumulates every file's
errors into `ce` without breaking. Verified.

**Block loop must also accumulate**: the three `return []error{de}` sites at
`parser.go:338` (no name), `:365` (unknown stanza) and the `parseResource`/`parseModule`
returns short-circuit the loop at `:326-367`, so a file with two unnamed blocks reports
only the first.

## Phase 1.1 implementation — done, decisions made

`internal/parser/parseResourcesInFile` changed in two places:

1. **Diagnostic path** — deleted the dead level-classification loop wholesale and now
   emit **one `ParserError` per diagnostic** via the existing
   `errors.NewParserErrorFromHCLDiag(d, file)`, each carrying its own
   `Subject.Start.Line/Column`. Previously only `diag[0]` was reported at
   `ParserErrorLevelWarning`.
2. **Block loop** — replaced all three `return []error{...}` short-circuits with
   accumulation into a `blockErrors` slice (`continue` after the no-name error), returning
   the whole set at the end. A file with two unnamed blocks now reports both.

**Judgement call — promoted the unknown-stanza error to `ParserErrorLevelError`
(`parser.go`, `default:` branch).** It was `ParserErrorLevelWarning` yet still aborted the
file. The plan assigns severity removal to Phase 4.1, but leaving it as a warning *while*
making the loop accumulate would let a malformed stanza slip past the Phase 1.2 gate —
the level is read by `ContainsErrors()` until 4.1 deletes it. Phase 4.1 removes the
constant anyway, so this is directionally identical, just earlier. **Flag at walkthrough.**

**Result:** the motivating test `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed`
**now passes**. Full suite went from 10 failures to **9** with nothing new appearing —
no regressions. `go build ./...` clean, `gofmt` clean, `go vet ./internal/parser/` adds
no findings (the `plugins/` ones are pre-existing).

**`TestParseFileReturnsConfigErrorWhenInvalidFileFails` still fails, and is NOT Phase 1.1's
to fix.** Its fixture is `invalid/notexist.xcl` — a **nonexistent path**, so `findXclFiles`
fails at `parser.go:243-246` and `Parse` returns a `fmt.Errorf` wrap, never reaching
`parseResourcesInFile`. Making `Parse` return a `ConfigError` consistently belongs with the
**Phase 1.2** gate work. Recorded so it is not mistaken for a Phase 1.1 miss.

## Phase 1.1 tests — written and independently verified

New fixtures in a **new** directory `internal/test_fixtures/config/multiple_errors/`
(deliberately new: `findXclFiles` scans non-recursively by `.xcl`, so existing tests that
assert exact error counts over `process_error/` and `invalid/` are unaffected):

- `two_blocks_missing_names.xcl` — two nameless `resource` blocks (lines 3, 7); proves
  **block-loop** accumulation.
- `unterminated_string.xcl` — one unterminated string (line 4) → **3** HCL diagnostics;
  proves **diagnostic-loop** accumulation.

Five new tests plus two strengthened, all in `internal/parser/parse_test.go`, all passing.
Counts stayed as decided (1 for `bad_format.xcl`, 3 for `config/invalid/`);
`TestParseDirectoryReturnsConfigErrorWhenResourceProcessError` rewritten to assert
**content** (file, line 7, col 30, "unable to parse file") rather than leaning on a bare count.

**Verified the tests are not vacuous:** stashed `parser.go` and re-ran — the three core new
tests **FAIL against the old code** and pass against the new. They genuinely exercise the change.

**Full suite: 9 failures, exactly the baseline's 10 minus the fixed motivating test, with no
new entries** (diffed the sorted failure list). `gofmt` clean.

Left alone deliberately: `TestParseFileReturnsConfigErrorWhenResourceInterpolationError`
(its warning/`ContainsErrors()==false` assertion inverts in **Phase 3.2**, not here).

## Phase 1.1 — CLOSED

Verified green, all 5 acceptance criteria checked in `plan.md`, changelog entry written
(`## Changelog` section created — this was the first `update_changelog` invocation).
11 phases remain; **next is Phase 1.2: Add the validation gate and make validation a
standalone answer** (High complexity, ~40k).

Note `errors/parser_error.go` is flagged by `gofmt -l` — **pre-existing**, `errors/` is
untouched by this work (`git diff HEAD -- errors/` is empty). Not ours to fix.

## Phase 1.2 analysis — the plan's STOP-and-ask HAS FIRED

Research (verified independently) found the plan's Open Question #1 is **live**, and the
picture is worse than the plan anticipated. Skipping the walk for `Validate` loses more
than resolution:

1. **Indirect cycle detection lives INSIDE the walk.** `d.Validate()` at `parser.go:882`
   is the only thing catching cycles of 3+ nodes. Direct 2-node cycles are caught earlier
   at parse time (`getDependentResources`, `parser.go:766-805`), so
   `TestParserCyclicalReferenceReturnsError` (2-node) would still pass — **the existing
   suite would not catch this regression.** Verified empirically with a 3-node fixture.
2. **Schema/decode errors live inside the walk** (`gohcl.DecodeBody`, `callbacks.go:107`).
   Fixtures `bad_format.xcl`, `function_error.xcl`, `bad_interpolation.xcl` all exercise it.
   Without the walk, `Validate` becomes **strictly weaker than today** until stages 2 and 3
   (Milestones 2-3) exist to replace those checks.
3. **`executePlugins=false` gates ONE thing only** — `callProviderLifecycle` at
   `callbacks.go:156`. `gohcl.DecodeBody` runs regardless. So today `Validate` and `Apply`
   do identical resolution work, differing only in whether providers are invoked. The doc
   comment at `parser.go:179` claiming `false` "tolerates missing interpolated values" is
   **not backed by any code**.
4. **The bool is overloaded in tests.** 38 of 41 `p.Parse(` test call sites pass `false`
   and then read **decoded** fields (`cont.Command[0]`, `cont.Resources.CPU`,
   `cont.Disabled`, `out.Value`). Redefining `false` to skip the walk breaks the parser
   suite wholesale. Any mechanism MUST keep "decode without plugins" reachable.

**Pre-walk state is a correct but empty skeleton**: all resources present in the returned
state with correct `Meta.ID`/`Meta.Links`/`DependsOn`, but every payload field zero.
`buildDiff` only reads `Meta.ID`, so it would still work — and `example/main.go:70-79`,
the sole `Config.Validate` consumer, reads only `len()` of the three slices.

**Mechanism options** (call sites: 2 production, 40 test):
- **A** second bool param → touches all 42
- **B** separate method (e.g. `ParseOnly`) → touches **1**; factor the pre-walk body into a
  shared helper. Smallest blast radius.
- **C** `ParserOptions.SkipWalk` field → touches 0 existing (zero value preserves behaviour)
- **D** mode enum → touches all 42, clearest contract, removes the overloading

**Corrections to earlier notes in this file:** the failing
`TestParseFileReturnsConfigErrorWhenInvalidFileFails` errors in **`findVarsFiles`**
(`parser.go:213`, via `util.go:59-103`), NOT `findXclFiles` (`parser.go:243`) — vars files
are discovered 30 lines earlier, so execution never reaches the xcl lookup. `ce` is already
declared at `parser.go:209`, above both, so either site can append to it without restructuring.

**Also found:** `TODO.md:146-147` is an **unchecked roadmap item** asking for *enhanced*
Diff — a direct contradiction with deleting it. Worth flagging to the user.
`docs/parser-lifecycle.md:28` documents `executePlugins=false` as the mode `Config.Validate`
uses for "schema/DAG checking" — becomes wrong under this plan; add to Phase 4.2 scope.

## USER DECISIONS on the Phase 1.2 STOP-and-ask (binding)

Put to the user with the findings above; they chose:

1. **Keep the walk for now.** The validation gate lands before the walk as planned, but
   `Config.Validate` **still walks**. This keeps every milestone in a working state and
   loses no existing checks (indirect cycles, schema/decode errors). The spec's
   "validation performs no resolution work" constraint is **deliberately deferred**, to be
   satisfied at the **end of Milestone 3**, once stages 2 and 3 replace what the walk
   provides. **Carry this forward — it is an outstanding task, not a dropped one.**
2. **Mechanism: an unexported `parseAndValidate()`** that parses and validates only.
   (Option B, explicitly private rather than an exported `ParseOnly`.)
3. **Renaming the public `Parse` → `Apply` is deferred** — the user said "we can do that
   later". Do **not** do it in this phase.

## Phase 1.2 implementation — done

- **New `internal/parser/validate.go`** — `(*Parser).validate()` owns stage ordering and
  accumulation, returning `[]error`. Stage 1 (`validateStructure`) is currently a no-op
  anchoring the ordering; stages 2 and 3 arrive in Milestones 2-3.
- **`Parse` split.** New unexported **`parseAndValidate(paths...) (*state.State, *state.State, error)`**
  holds everything up to and including the gate — state load, file discovery, the parse
  loop, the parse gate, the **validation gate**, and the move into `currentState`. `Parse`
  now calls it, then walks. Per the user's decision the walk still runs for `Validate`;
  **removing it is deferred to the end of Milestone 3.**
- **Gate placement**: after the parse gate, before resources move into `currentState` —
  so nothing reaches state when validation fails.
- **`Config.Validate` now returns `error` alone**; `diff.go` **deleted**; `example/main.go`
  rewritten to the error-only contract. No `Diff`/`buildDiff`/`ToCreate`/`ToUpdate`/
  `ToDestroy` references remain in any Go file. The two in `docs/` are **Phase 4.2** scope.
- **Path-discovery errors now return `ConfigError`.** Both `findVarsFiles` (`parser.go:213`)
  and `findXclFiles` sites previously returned `fmt.Errorf` wraps; they now append a
  `ParserError` to the existing `ce` and return it. This fixes the second in-scope baseline
  failure.

**Result: baseline 10 failures → 8.** Both in-scope failures now pass
(`...WhenResourceBadlyFormed` from 1.1, `...WhenInvalidFileFails` from 1.2).

**Flaky test identified — NOT a regression.** `TestParserErrorsOnPluginCreateError` failed
once in a `go test ./...` run but passed 10 consecutive isolated/package runs and never
recurred across 3 further full-suite runs. It spawns plugin processes, so it is sensitive to
parallel-package resource contention. Do not chase it; do not add it to the baseline list.

`example` still fails its **test** build on the pre-existing vet findings at
`example/main.go:16,35` (redundant newlines) — lines untouched by this work and explicitly
out of scope.

## Phase 1.2 tests — written and verified

New `config_validate_test.go` in the **root package** (14 tests) plus 2 appended to
`internal/parser/parse_test.go`. `Config.Validate` previously had **zero** coverage.

**CORRECTION to a plan assumption (and to what I relayed):** the plan's claim that a
root-package test cannot reach `TestPlugin` is **wrong**. The module path is
`github.com/jumppad-labs/xcl` and the root package is `xcl`, so `internal/...` *is*
importable from the repo root — Go's internal rule only blocks importers outside the
subtree rooted at `internal`'s parent. `TestPlugin` also lives in `test_plugin.go`, a
**non-test** file, so it is part of the normal package surface. This gave the strong
direct assertion (`GetCreatedResources()` etc.) instead of the weaker state-store
fallback; the mock `StateStore` `AssertNotCalled(t, "Save", ...)` is used as a *second*
independent check, not a substitute. **Useful for every later phase.**

**Non-vacuity guarded**: `TestApplyCreatesResourcesFromValidConfiguration` proves the
plugin genuinely records creates, so the `require.Empty` assertions elsewhere are real
evidence rather than a dead-registry artifact. `TestValidateJudgesDirectoryAsOneUnit` was
checked against the single file in isolation (which *does* error alone), so it is not a
tautology.

All 5 acceptance criteria covered. Failure count **stable at 8 across repeated runs**,
identical to the post-1.2 baseline; gofmt clean.

## Phase 1.2 — CLOSED

Verified green by an independent pass (gate ordering confirmed at
`internal/parser/parser.go:292-310`: parse gate → validation gate → `AppendResource`).
All 5 criteria checked, changelog written. **10 phases remain; next is Phase 1.3: Fail on
a resource whose type cannot be checked, and guard module recursion** (Medium, ~30k).

**OUTSTANDING, do not lose:** remove the walk from `Config.Validate` at the **end of
Milestone 3**. Deferred by user decision, not dropped.

## BASELINE CORRECTION — the plan's "10 failing tests" undercounts

A **9th** pre-existing parser failure exists that the plan's baseline table misses:
**`TestPluginResourceCreationWithFallback`** (`internal/parser/parser_plugin_test.go:51`)
panics with a nil-pointer dereference in `PluginRegistry.CreateResource`
(`plugins/registry/plugin_registry.go:37`) because the registry is nil.

**Verified pre-existing** by stashing all work and running it alone on the unmodified
tree — it panics identically. It is masked in normal runs because `TestDestroyLifecycle`
panics earlier in the same package and aborts the run before it executes. Enumerating
parser failures fully requires an explicit `-run` list excluding the panickers.

**Out of scope** (same class as `TestDestroyLifecycle` — incomplete mock setup), but
**Phase 4.3 must count it** or it will look like a regression introduced by this work.

## Phase 1.3 implementation — done

**Half A (type naming).** Confirmed the plan's "verify first" instruction was right: an
unregistered type **already failed** fatally at `parseResource`. The only gap was the
message interpolating `b.Type` (the literal `"resource"`). Now reads
`unable to create resource '<name>' of type '<type>': <err>`.

**Half B (module recursion guard).** `parsed` gained `moduleSources map[string]bool`,
initialised in `parseAndValidate`. `parseModule` resolves the source through a new
`canonicalPath()` helper in `util.go` (`filepath.Abs` → `filepath.EvalSymlinks`) and
reports `module '<name>' source '<dir>' includes itself` on re-entry, against the module
block. **`defer delete(...)` after the children are parsed** is load-bearing: it keeps the
guard to genuine re-entrancy so a module legitimately used twice as siblings still works.
An unresolvable source now reports `unable to obtain contents for module ...`.
`parseModule`'s child-file loop also now accumulates instead of returning at the first file.

Probed before and after: a self-including module previously recursed to stack exhaustion
and now reports cleanly with file/position; `config/modules/modules.xcl` still yields
exactly 41 resources.

## Phase 1.3 tests

Six tests in `parse_test.go`, six new fixture dirs (all NEW subdirectories, so no existing
count assertion is disturbed). **Recursion tests carry a 30s timeout guard** so a
regression fails rather than hangs. **The positive control
`TestParseModuleParsesSameSourceUsedTwiceAsSiblings` passes** — the most valuable test in
the set, since a guard that wrongly blocked legitimate module reuse would otherwise go
undetected.

## Phase 2.1 analysis — MODULE-SCOPED REFERENCES (plan did not anticipate this)

Probed real fixtures. The plan's core insight is **confirmed**: applying
`StringWithoutAttribute()` **before** the map lookup resolves references cleanly and yields
the leftover attribute path for stage 3. On `config/simple/container.xcl` all 14 links
resolve with zero misses.

**But `config/modules/modules.xcl` has 6 apparent "misses" that are NOT broken references:**

```
MISS resource.network.onprem.meta.name      base=resource.network.onprem
MISS variable.cpu_resources                 base=variable.cpu_resources
MISS resource.container.consul.resources.cpu base=resource.container.consul
```

These are references made **from inside a module**, written unqualified, while the working-set
key is module-scoped (`module.consul_1.variable.cpu_resources`). Confirmed the mechanism: the
referring resource's **`Meta.Module`** carries the scope (e.g. `"consul_1"`), and the resource
declaring these links is `module.consul_1.resource.container.consul`.

**Therefore stage 2 resolution MUST be two-step:**
1. If the referring resource has a non-empty `Meta.Module`, try `module.<Meta.Module>.<base>` first.
2. Fall back to the bare `<base>` (global scope).

A naive single-lookup implementation would report all 6 as undefined and **break every module
configuration** — exactly the "over-eager implementation passes the negative tests while
breaking real config" failure the plan warns about for 3.2. `config/modules/modules.xcl` (41
resources) is the regression guard.

## Phase 2.2 — position granularity STOP-and-ask RESOLVED (no escalation needed)

The plan's Open Question #2 asked whether block-granular positions are precise enough,
to be judged "once problems are actually being reported". They now are, so:

Probed a two-file config with three bad references. All three are reported together, each
naming the missing item and its file/line. Critically, `ParserError.Error()` renders a
**source excerpt** around the reported line, so the offending reference appears directly
beneath the highlighted block header:

```
resource 'resource.container.second' refers to 'variable.missing_var', which is
not defined anywhere in the configuration
  /tmp/.../b.xcl:1,1
      1 | resource "container" "second" {     <- highlighted
      2 |   command = [variable.missing_var]  <- the reference is right there
```

**Decision: block granularity is sufficient; do NOT escalate.** Capturing exact expression
positions would mean changing the shared `processExpr` path that also feeds the DAG — a
materially larger and riskier change the plan explicitly sizes out of scope. Recorded rather
than raised with the user because the evidence answers the question the plan posed.

**Stage 2 ordering note:** `validate()` returns after stage 2 finds anything, so stage 3
never runs on a configuration with broken references — deliberate, per the spec.

**Determinism:** resource IDs are sorted before iteration (`sortedResourceIDs`). The working
set is a map, so without this the same faults would be reported in a different order each run.

## Phase 2.3 — already satisfied by earlier phases; NO new production code

Probed all four requirements rather than assuming a gap (the same discipline Phase 1.3's
"verify first" instruction called for):

1. **Module outputs resolve statically — the plan's open assumption #6 is CONFIRMED.**
   `module.consul_1.output.container_resources_cpu` and `.combined_map` both resolve with
   `found=true`; `.nosuch_output` correctly returns `found=false`. **`SubContext` is not
   needed** for a static existence check, exactly as the plan's closed question predicted.
   No need to STOP and ask.
2. **Undeclared module output fails naming it** — falls out of Phase 2.2's stage 2.
3. **Unobtainable module reported once, against the module** — delivered by Phase 1.3's
   `unable to obtain contents for module ...` error.
4. **Suppression works structurally, no filter required.** The plan anticipated needing a
   post-collection filter to stop references into a broken module being reported too.
   Not needed: `parseModule` reports an unobtainable module during **parsing**, so the
   **parse gate returns before stage 2 ever runs**. Probed a config with an unobtainable
   module plus **three** inbound references (`module.missing.output.{cmd,first,second}`):
   **exactly 1 problem reported**, against the module. Adding a filter would be dead code.

So Phase 2.3 is a verification phase — tests only.

## Phase 3.1 — type name resolver, cross-checked against the runtime

New `internal/parser/types.go`: `propertyNames(reflect.Type) map[string]reflect.Type`.
Read the pinned fork directly
(`/home/nicj/go/pkg/mod/github.com/jumppad-labs/go-cty@v0.0.0-20230804061424-9e985cb751f6`,
`cty/gocty/helpers.go` + `type_implied.go`) and mirrored its rule: name is
`strings.Cut(tag.Get("hcl"), ",")` first segment; empty first segment contributes nothing;
anonymous embeds flatten **recursively**; **an embed can do both at once**; no `hcl` tag =
invisible.

**The strongest verification available — exact agreement with the runtime.** Cross-checked
`propertyNames` against `gocty.ImpliedType(v).AttributeTypes()` (the actual fork the runtime
uses):

| type | ours | go-cty | only-ours | only-gocty |
|---|---|---|---|---|
| Container | 21 | 21 | — | — |
| Network | 4 | 4 | — | — |
| Template | 8 | 8 | — | — |

**Zero divergence in either direction.** This is a far better test than hand-listing names,
and it must be in the committed suite — it is the phase's central criterion ("names match
exactly what the system accepts") turned into an executable check that cannot drift.

Landmines verified handled: `Container` yields **both** `rm` *and* the flattened
`meta`/`depends_on`/`disabled` (the `hcl:"rm,remain"` outlier); `Meta` yields 7 properties
with `properties`/`links`/`status` **absent** despite being real Go fields.

## Phases 3.2 + 3.3 — walker implemented together (stopping rules are meaningless apart)

New `internal/parser/properties.go`: `checkPropertyPath(reflect.Type, path) string` returns
the first non-existent segment, or "" to accept. Stage 3 wired into `validate()` via
`validateProperties()`.

**THREE false-rejection causes found by running against real fixtures — none anticipated by
the plan.** Each broke working configuration before being fixed:

1. **A map key is only recognisable by the type it stands against.** `created_network_map.one.subnet`
   — `one` is an arbitrary key, indistinguishable in writing from a property name. Fixed by
   treating *any* segment standing against a Slice/Array/Map as a member selection
   (`isCollection`), not just `*` and digits. Before the fix `...one.subnett` wrongly ACCEPTED.
2. **A reference to an output or variable names the value it holds, not the declaration.**
   `module.consul_1.output.combined_map.name` reads `name` from the output's *value*, but the
   walk was checking it against the `Output` struct (which has only value/description/meta) and
   rejecting. Fixed by `holdsDynamicValue()` — `resources.Output` / `resources.Variable` stop the
   walk immediately. **This broke 9 tests including the 41-resource modules fixture.**
3. **`cty.Value` fields are dynamic.** `Output.CtyValue` and `Template.Vars` are `cty.Value`;
   `knowable()` stops there (also on `reflect.Interface`), applied both to plain fields and to
   collection element types.

Verified 32/32 path shapes correct, including **both halves of the critical pairing**:
`volume.*.destination` accepts / `volume.*.destnation` rejects; `created_network_map.one.subnet`
accepts / `...subnett` rejects. Also handles all three index spellings (`volume.0.source`,
`volume[0].source`, `volume.*.x`) since `ParseFQRN` preserves each verbatim.

**THE HEADLINE BEHAVIOUR CHANGE — `TestParseFileReturnsConfigErrorWhenResourceInterpolationError`
INVERTED as the plan requires.** It asserted `ContainsErrors()==false` and
`Level == Warning` for `resource.network.test.nam` (a typo for `name`). It now asserts
`ContainsErrors()==true`, `Level == Error`, and that the message names `nam`. This is the
defect the whole plan exists to remove — **deliberate, not a regression.**

Baseline back to the clean **8** with stage 3 fully live.

## KNOWN GAP — the splat typo is unreachable end-to-end (verified)

`volume.*.destnation` is caught by `checkPropertyPath` at the **unit** level but **cannot be
reached end-to-end**. Verified directly: link collection records only
`resource.container.first.volume` for a splat — the trailing `destnation` is discarded.

**Root cause:** `processExpr`'s `*hclsyntax.SplatExpr` case (`internal/parser/exp.go:122-131`)
traverses only `ex.Source` and **never `ex.Each`**, so everything written after the `*` is
dropped before validation sees it.

**This is a pre-existing blind spot the plan explicitly put OUT OF SCOPE** under "Reference
collection is not widened": `processExpr` also feeds the DAG, so widening it would change what
the dependency graph acts on — a materially different and riskier change. So this is a
**delivered-scope limitation, not a defect introduced here**.

**Consequence for Phase 3.2's criterion** "A property that members of a collection do not have
is reported, even when named after selecting from that collection": met for the **dotted and
bracket** forms (`volume.0.destnation`, `volume[0].destnation` — both survive collection,
normalised to bracket form, and ARE reported), but **not for the splat form**. The negative
fixtures were therefore built on the dotted form. **Flag to the user at the end.**

Also noted: `internal/test_fixtures/functions/default.xcl` cannot complete a full `Parse` from
the parser package — it calls `file("./default.hcl")` (a file that does not exist in the repo)
resolved against the process working directory. Its regression test asserts via
`parseAndValidate` + `validate()` instead. Its only other consumer, `internal/functions`, is one
of the three pre-existing build failures, so that fixture is otherwise unexercised.

## ALL 12 PHASES COMPLETE — closing state

Test plan, project changelog, repo changelog and spec reconciliation all written.
**33 of 35 spec checkboxes marked satisfied**; 2 left unchecked and annotated (both the
splat-form limitation).

**Final state:** `go build ./...` clean. Suite at **8 failures**, all verified pre-existing
and unchanged by an audit that compared against a pristine `git archive` extraction of
commit `858013a` (not a stash — too risky with a deleted file and 34 untracked paths).
Parser tests grew **48 → 178**. `go vet` findings identical to baseline. `gofmt` improved
(2 flagged files → 1, and that one untouched by this work).

**The deferred walk removal WAS delivered** in Phase 4.1, as agreed — `Config.Validate` now
decodes nothing, walks no DAG and reaches no provider.

## Carry forward

- Line numbers **will drift** once edits land — re-locate by symbol name.
- Two STOP-and-ask items from the plan's Open Questions remain live:
  1. **Phase 1.2**: `Parse` always calls `p.walk` (`parser.go:272`) regardless of
     `executePlugins`; making the walk conditional is required for "validation acts on
     nothing". If something downstream turns out to need decoded values during
     validation, **stop and ask the user**.
  2. **Phase 2.2/2.3**: reference positions are **block-granular** (from the referring
     resource's `Meta`), not expression-exact. If that proves too coarse once real output
     can be judged, **raise with the user** rather than widening scope.
- Phase 4.2 must write the knowledge entry via `spektacular knowledge write` with
  propose-and-confirm — **not** the `Write` tool.
