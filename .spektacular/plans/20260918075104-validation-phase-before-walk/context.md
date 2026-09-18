---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Context: 20260918075104-validation-phase-before-walk

**Repo:** `xclconfig` — the only registered repo
(`spektacular repo list` → root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).
All paths below are relative to that root. Line numbers are as of branch `v2`,
commit `858013a`; **re-locate by symbol name** rather than trusting a number once
edits begin.

## Current State Analysis

### The pipeline today

`Parse` (`internal/parser/parser.go:182`) is the single funnel. Both public
entry points reach it and differ only by one boolean:

| entry point | call | effect |
|---|---|---|
| `Config.Validate` (`config.go:67`) | `Parse(false, …)` | skips providers, **still walks and decodes** |
| `Config.Apply` (`config.go:94`) | `Parse(true, …)` | walks, decodes, calls providers |

Inside `Parse`:

| lines | what happens |
|---|---|
| 187-198 | load previous state (note: `previousState` is then **never used** — `walk` hardcodes `nil` at 902-904) |
| 249-254 | parse loop over every discovered file, accumulating into `ce` |
| 256-258 | hard gate — `if len(ce.Errors) > 0 { return nil, ce }` |
| 260-265 | move resources into `currentState` (map iteration, so order is non-deterministic) |
| 268 | `functions := p.getFunctions()` |
| 272 | `p.walk(...)` — **always**, per the comment at 269-271 |

**The validation gate belongs between 258 and 260.** At that point every
resource from every file is in `p.parsedResources`, every `Meta.Links` is
populated, and no body has been decoded.

### The defect, precisely

`parseResourcesInFile` (`parser.go:284`) classifies parse diagnostics:

```go
level := errors.ParserErrorLevelWarning          // :290 — defaults to warning
for _, e := range diag.Errs() {
    err, ok := e.(*hcl.Diagnostic)
    if !ok { continue }
    if err.Summary == "Error in function call" {  // :298 — only this promotes
        level = errors.ParserErrorLevelError
        break
    }
}
```

Two independent faults:

1. **The promotion branch is dead by construction.** `ParseXCLFile`
   (`hclparse/parser.go:68` → `hclsyntax.ParseConfig`) builds an AST and never
   evaluates, while every function-call diagnostic originates in
   `FunctionCallExpr.Value(ctx)` — an evaluation method. So a syntax error is
   **always** a warning. This is why
   `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed` fails at
   `parse_test.go:917`.
2. **Only `diag[0]` is ever reported** (`:305-315`), then `return []error{de}`.
   Probed: an unterminated string yields **3** diagnostics, an unclosed `${`
   yields **3** — two are discarded each time.

The same classifier exists a second time, byte-for-byte, as
`checkIfErrorInFunction` (`parser.go:923-938`), called once from
`callbacks.go:110`. That call site is **not** dead — `gohcl.DecodeBody`
evaluates, so the promotion can genuinely fire there.

**Crucially, severity is already advisory-only:** both sites `return`/`Append`
regardless of level, and `Parse:256` aborts on any accumulated error. The level
changes only what `ContainsErrors()` reports (`errors/config_error.go:37-53`),
where a non-`ParserError` defaults to *true*.

### What already exists in our favour

- **Reference extraction, evaluation-free.** `processExpr` (`exp.go:22`) walks
  ten expression forms structurally and yields dot-joined reference strings
  rooted at `resource`/`module`/`variable`/`output` (`exp.go:152` filters the
  rest), with bracket indices. It already runs at parse time via
  `getUniqueResourceLinks` (`parser.go:697`) to feed the DAG.
- **Module children are already parsed.** `parseModule` (`parser.go:565-692`)
  resolves local sources (`:667`) and recurses (`:674-689`), keying children
  `module.<instance>.resource.<type>.<name>`. So module contents cost nothing
  extra at validation time.
- **Error plumbing fits.** `ConfigError` accumulates
  (`errors/config_error.go:11-23`); `ParserError` carries `Filename`, `Line`,
  `Column` (`errors/parser_error.go:17-24`) — the "what and where" the spec wants.
- **Typed resources exist at parse time.** `parser.go:404` instantiates the
  concrete resource, so a `reflect.Type` is available per resource.

### The silent fall-through — stage 2's whole job

`parser.go:778-817` looks up each dependency in `parsedResources.resources`:

```go
if depResource, ok := p.parsedResources.resources[dep]; ok {   // :788 — NO else
```

Four stacked silent-miss paths: `:787` (nil map), `:788` (key miss), `:790-792`
(meta failure), `:796-799` (unparseable FQRN). The key miss fires routinely,
for two structural reasons:

1. **Ordering** — resources are parsed in file order, so any forward reference
   misses.
2. **Key shape** — `FQRNFromResource` (`internal/resources/fqrn.go:167-177`)
   **never sets `Attribute`**, while `processExpr` references often carry one.
   `StringWithoutAttribute()` (`fqrn.go:205`) exists but is applied at `:800`,
   *after* the lookup at `:788`.

Only the direct two-cycle A→B→A is actually caught (`:803-813`). Longer cycles
reach `d.Validate()` (`parser.go:894`) and report with **no file or line**.

### Measured baseline — the suite is NOT green before any change

`go build ./...` is clean. `go vet` reports pre-existing findings in `plugins/`
and `example/`.

**3 test packages do not compile:** `internal/functions`
(`functions_test.go:192`, undefined `setupParser`), `plugins/registry`
(`plugin_discovery_test.go:13`, undefined `newTestPluginSetup`), `example`.

**10 tests fail:**

| group | tests |
|---|---|
| **In scope — this work fixes** | `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed` (`:903`, the motivating test); `TestParseFileReturnsConfigErrorWhenInvalidFileFails` (`:963` — wrong error *type*: `*fmt.wrapError`, not `*errors.ConfigError`) |
| **Must be decided, not assumed** | `TestParserEventCallback` (`:1001`), `TestParserEventErrorCallback` (`:1071`), `TestParserEventForVariablesOutputsLocals` (`:1128`) — on the error-production path this work rewrites; `TestParserErrorHighlightsLine`, `TestParserErrorNonErrorLineGrey` (`errors/`) — on the rendering path |
| **Out of scope** | `TestDestroyLifecycle` (`:1189`, **panics** — mock `StateStore.Exists` has no expectation, `parser.go:189`); `TestReadResourceFromFileAtLocation`, `TestLineFromFileAtLocation` (root, `.hcl`/`.xcl` fixture drift) |

**Consequence: "all tests pass" is not a usable exit criterion.** See Phase 4.3.

### Two existing tests assert the spec's opposite

- `parse_test.go:943-961` — `TestParseFileReturnsConfigErrorWhenResourceInterpolationError`
  asserts `require.False(t, ce.ContainsErrors())` and `Level == Warning` for
  `resource.network.test.nam` (`bad_interpolation.xcl:11`) — a property typo.
  The spec requires this to **fail**. Invert it.
- `parse_test.go:873-885` — asserts `Len(ce.Errors, 1)` over
  `config/process_error/`, a directory of **three** broken files. That count
  encodes the defect.

Also note `parse_test.go:919,939,959` do unchecked
`ce.Errors[0].(*errors.ParserError)` — these **panic** if a non-`ParserError`
enters the collection.

### Schema feasibility — probed, not inferred

Throwaway tests against `structs.ContainerBase` (since removed) established:

| Go field | `Type` string | `Properties` |
|---|---|---|
| `Volumes` | `[]structs.Volume` | Source, Destination, … |
| `CreatedNetworksMap` | `map[string]structs.Network` | (ResourceBase), Subnet |
| `Env` | `map[string]string` | *(none — scalar value type)* |

`serialize.go:60-70` collapses Slice/Map/Ptr to `Elem()` while `Type` still
records the collection — exactly the unknown-member / known-type distinction the
spec needs.

**But `GenerateSchemaFromInstance` must not be reused as the walk mechanism:**

- returns **JSON bytes**; `serializeAttribute` (`:23`) is unexported
- **truncates silently**: depth 1 → 8 of 86 nodes; the cap returns `nil`
  (`:27-29`) with no marker, so a checker cannot tell "absent" from "not looked
  at" → **false rejections**
- **no cycle guard**: a self-referential struct produced **137 KB** at depth 100

Plugin schemas arrive pre-generated at depth **10** (`plugins/plugin.go:107`),
so truncation is live, not hypothetical. Treat a truncation boundary as
unknowable and **accept**.

### The naming rule lives in a forked dependency

`go.mod:98` replaces `zclconf/go-cty` with `jumppad-labs/go-cty`. In the module
cache, `cty/gocty/helpers.go:34-54`:

```go
attrName := field.Tag.Get("hcl")        // upstream reads "cty"
attrs := strings.Split(attrName, ",")
if len(attrs) > 0 && attrs[0] != "" { ret[attrs[0]] = i }
```

Rules to mirror exactly:

1. HCL name = `strings.Split(tag.Get("hcl"), ",")[0]`
2. `optional` / `block` strip identically
3. empty first segment → contributes no name
4. **no `hcl` tag → invisible to HCL.** `Meta.Properties`, `Meta.Links`,
   `Meta.Status` (`types/resource.go:36,40,45`) must be reported as **not
   existing** despite being real Go fields
5. anonymous embeds flatten **recursively** (`type_implied.go:107-121`, a
   fork-only addition)

**The `rm,remain` landmine:** `structs/container.go:14` uses `hcl:"rm,remain"` —
a non-empty first segment, so it registers an attribute named `rm` **and**
flattens. It is the lone outlier among ten embeds; the other nine use
`hcl:",remain"`.

**No first-party file in this repo parses HCL tags today.**

## Per-Phase Technical Notes

All phases are carried out in the single registered repo **`xclconfig`**
(`spektacular repo list` → root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).
Paths below are relative to that root. Line numbers are as of branch `v2` at
commit `858013a` and will drift as edits land — re-locate by symbol name rather
than trusting a number after the first edit in a file.

**Requirement → repo/files resolution.** Every spec requirement resolves to
`xclconfig`, concentrated in three areas: the parse and validation pipeline
(`internal/parser/`), the error types (`errors/`), and the public entry points
(`config.go`, `diff.go`). No requirement touches another repo.

---

### Phase 1.1: Report every problem in a file, and treat malformed configuration as a failure

**File changes**

- `internal/parser/parser.go:284-318` — `parseResourcesInFile`. Rewrite the
  diagnostic block. Two separate defects to fix here:
  - **:290-302** — delete the level-classification loop entirely. It defaults to
    `ParserErrorLevelWarning` and only promotes on `err.Summary == "Error in
    function call"`, which **cannot occur on this path**: `ParseXCLFile`
    (`hclparse/parser.go:68` → `hclsyntax.ParseConfig`) builds an AST and never
    evaluates, while all function-call diagnostics originate in
    `FunctionCallExpr.Value(ctx)`, an evaluation method. The branch is dead by
    construction. Replace with an unconditional error.
  - **:303-317** — currently reads only `diag[0]` for position and detail, then
    `return []error{de}`. Change to iterate **all** of `diag`, emitting one
    `ParserError` per diagnostic with that diagnostic's own
    `Subject.Start.Line/Column`. Measured: an unterminated string yields 3
    diagnostics, an unclosed `${` yields 3 — today 2 of 3 are discarded in each
    case.
- `internal/parser/parser.go:284` — the signature already returns `[]error`; no
  change needed, but **every** `return []error{...}` in the function body
  (:317, :338, :365) short-circuits the block loop at :326-367. For this phase
  only the diagnostic path must accumulate; the block loop should also continue
  past a bad block rather than returning, so a file with two unnamed blocks
  reports both.
- `internal/parser/parser.go:249-254` — the caller already accumulates per-file
  errors into `ce` and does not break, so no change; verify it still holds.
- `errors/parser_error.go:162-177` — `NewParserErrorFromHCLDiag` already exists
  and hard-codes `Level: ParserErrorLevelError` (:174). **Prefer reusing it** for
  the per-diagnostic construction rather than hand-building; it already takes a
  `*hcl.Diagnostic` and a filename. Note it does not currently set Line/Column
  from the diagnostic — check and extend if needed.

**Tests**

- `internal/parser/parse_test.go:903-921` —
  `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed`. Currently fails at
  **:917** (`require.True(t, ce.ContainsErrors())`). Should pass after this
  phase. Note `ContainsErrors()` survives until Phase 4.1, so this assertion is
  still valid here; it must be revisited when that method is deleted.
- `internal/parser/parse_test.go:915` — `require.Len(t, ce.Errors, 1)` on
  `bad_format.xcl`. **Decide** the new count from the diagnostics that file
  actually produces (`bad_format.xcl:7` is a `resource` block missing its `{`;
  probe showed 1 diagnostic — `Invalid block definition`). If it stays 1, say so
  deliberately rather than leaving it unexamined.
- `internal/parser/parse_test.go:873-885` —
  `TestParseDirectoryReturnsConfigErrorWhenResourceProcessError` asserts
  `Len(ce.Errors, 1)` over `config/process_error/` which holds **3** broken
  files. This assertion encodes the defect. Rewrite to assert on the *content* —
  that each of the three files is represented — not on a bare count.
- `internal/parser/parse_test.go:858-871` —
  `TestParseDirectoryReturnsConfigErrorWhenParseDirectoryFails` asserts
  `Len(ce.Errors, 3)` over `config/invalid/` (3 files, one bad block each).
  Likely unchanged, but confirm each file still contributes exactly one.
- New fixture: a file with **two** separate malformed blocks, to prove
  accumulation within a single file. Nothing existing covers this.

**Complexity**: Medium
**Token estimate**: ~25k
**Agent strategy**: Single agent, sequential. The change is confined to one
function plus its tests, and the test updates depend on the implementation's
actual diagnostic output.

---

### Phase 1.2: Add the validation gate and make validation a standalone answer

**File changes**

- `internal/parser/parser.go:256-272` — **the insertion point.** After the parse
  gate at :256-258 and before the state transfer at :260-265. Call the new
  validator with `p.parsedResources` and `p.pluginRegistry`; on a non-empty
  result, append each problem to `ce` and `return nil, ce`.
  - Insert **before** :260-265, not after: the spec requires nothing be acted
    upon, and moving resources into `currentState` is the first step toward that.
  - Note :268 (`p.getFunctions()`) and :272 (`p.walk`) must become unreachable
    for an invalid configuration.
- **New file** `internal/parser/validate.go` — the validator entry point. Takes
  `*parsed` and the registry; returns `[]error`. Owns stage ordering and
  accumulation. For this phase it runs only the structural checks (Phase 1.3
  fills them in); stages 2 and 3 are added in later milestones.
- `config.go:61-88` — `Config.Validate`. Change signature from
  `(paths ...string) (*Diff, error)` to `(paths ...string) error`. Delete
  :86-87 (`buildDiff` call and diff return). Update the doc comment at :61-66,
  which currently claims it "compares against existing state / Returns a Diff".
  It must **not** walk — see the note below.
- `diff.go` — **delete the entire file** (67 lines: `Diff` at :11-15,
  `buildDiff` at :18-67). Verified: one production consumer (`config.go:86`),
  zero test coverage, no `diff_test.go`.
- `example/main.go:68-80` — the only other consumer. Uses
  `diff, err := config.Validate(...)` then reads `len(diff.ToCreate)`,
  `ToUpdate`, `ToDestroy`. Rewrite to the error-only contract. **Note this
  package currently fails `go vet`** (`example/main.go:16,35` redundant
  newlines) — pre-existing, do not fix as part of this.

**Important subtlety — "Validate must not walk".** `Parse` **always** calls
`p.walk` at :272 regardless of `executePlugins` (see the comment at :270-271:
the walk is needed to decode resources even when plugins are off). So
`Config.Validate` calling `Parse(false, ...)` still walks today. Satisfying
"validation acts on nothing" requires either a `Parse` variant that stops after
validation, or a parameter telling it to. **Decide this explicitly** — it is the
one place where the gate alone is not sufficient. The `executePlugins=false`
path does not call providers (`callbacks.go:156`), so nothing is created today
either; but it *does* decode, which is resolution work the spec forbids in
validation.

**Tests**

- `config_test.go` — currently only 2 live tests (:16-31); lines 33-457 are one
  commented-out block. **`Config.Validate` has zero coverage**, so these are the
  first tests at this layer. New tests: validating a valid configuration returns
  no error and creates nothing; applying an invalid one returns an error and
  creates nothing. Use `TestPlugin`'s `GetCreatedResources()` /
  `GetDestroyedResources()` (`internal/parser/test_plugin.go:34-54`) to assert
  nothing happened — note these live in package `parser`, so a root-package test
  needs an equivalent observation route.
- Check `plugins/example/e2e_test.go` for `.Validate(` hits — research found
  these are a *different* (plugin-interface) `Validate` and unaffected, but
  confirm before changing signatures.

**Complexity**: High
**Token estimate**: ~40k
**Agent strategy**: Parallel analysis, sequential integration. The `Diff`
removal and the gate insertion are independent reads but must land together —
`Validate` cannot both stop before the walk and return a diff that requires
walking.

---

### Phase 1.3: Fail on a resource whose type cannot be checked, and guard module recursion

**File changes**

- `internal/parser/parser.go:404-414` — `pluginRegistry.CreateResource` already
  produces a **fatal** `ParserError` at `ParserErrorLevelError` when a type is
  not registered. So "unverifiable type fails" may **already hold** at parse
  time. **Verify first**: write the failing case before writing any code. Note
  the message at :410 interpolates `b.Type` (the literal string `"resource"`),
  not the unknown type name — the type name appears only inside the wrapped
  error. Improve so the problem names the type, per "each problem says what".
  - Registry path: `plugins/registry/plugin_registry.go:35-43` → `:82` returns
    `"resource type %s not found in any registered plugin"`. Note `:37` silently
    swallows the builtin error and falls through, so a malformed builtin
    surfaces as "not found in any plugin" — misleading but pre-existing.
- `internal/parser/parser.go:674-689` — `parseModule` recursion. **Add a
  visited-set guard.** Today `findXclFiles(sourceDir)` at :674 then
  `parseResourcesInFile(childFile, moduleInstanceName)` at :684 recurses with no
  record of which source directories have been entered. A module whose source
  transitively includes itself recurses to **stack exhaustion** — a crash, not a
  diagnostic. Track resolved `sourceDir` values (absolute, symlink-resolved) on
  the `Parser` or threaded through, and emit a `ParserError` against the module
  block (`b.TypeRange.Start`) on re-entry.
  - `sourceDir` is computed at :667 as
    `filepath.Join(filepath.Dir(file), sourceVal.AsString())` — local-relative
    only. `filepath.Abs` + `filepath.EvalSymlinks` before comparing.
- `internal/parser/parser.go:645-666` — module `source` resolution. Already
  errors when `source` is missing (:645-654) or unresolvable (:656-663, evaluated
  with a **nil** EvalContext so it must be a string literal). A non-local source
  is not currently distinguished — it simply fails to be a directory. Per the
  spec, treat as "contents cannot be obtained", reported against the module.
  `Version` (`internal/resources/module.go:17`) is declared but never read — do
  not start reading it; out of scope.

**Tests**

- New fixture: a config with a resource of an unregistered type. Assert failure
  and that the message names the type.
- New fixture: two module directories whose sources point at each other, plus
  one self-referential. Assert a reported failure rather than a crash. **Add a
  timeout guard** so a regression manifests as a failure, not a hung suite.
- `internal/test_fixtures/config/modules/` — existing module fixtures must keep
  parsing. `modules.xcl` asserts 41 resources (`parse_test.go:349,398`).

**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: 2 parallel agents — the type check and the module guard are
independent, touching different functions.

---

### Phase 2.1: Resolve references against the whole configuration

**File changes**

- **New file** `internal/parser/references.go` — the reference resolver.
  Signature per § Data Structures: reference string → (target, remaining
  attribute path, found).
- `internal/resources/fqrn.go:53-141` — `ParseFQRN`. **Read carefully; do not
  change.** For the `resource` branch (:72-81),
  `attribute = strings.Join(resourceParts[2:], ".")` — a flat dotted string with
  bracket indices **not** decomposed (only the `output`/`local` branch at :95
  does that). Splats survive verbatim (`fqrn_test.go:50` → `"tasks.*.id"`).
- `internal/resources/fqrn.go:205-224` — `StringWithoutAttribute()`. **This is
  the key to the fix.** Working-set keys come from `FQRNFromResource` (:167-177)
  which **never sets `Attribute`**, while references from `processExpr` do carry
  one. Apply `StringWithoutAttribute()` **before** the map lookup.
  - Contrast `internal/parser/parser.go:800`, which blanks the attribute *after*
    the lookup at :788 — that ordering is exactly why the current lookup misses.
- `internal/parser/parser.go:741-820` — `getDependentResources`. The reference
  strings come from `processExpr` (`internal/parser/exp.go:22`) at :746 and
  :762-776. **Reuse; do not reimplement** (spec Technical Approach).
  - Known blind spots, pre-existing and shared with the DAG — do **not** widen
    here: `IndexExpr`, `RelativeTraversalExpr`, `ForExpr` yield nothing, and
    `SplatExpr` (`exp.go:122-131`) traverses `Source` but not `Each`.
  - Reference string format from `processScopeTraversal` (`exp.go:145-178`):
    dot-joined, rooted at `resource`/`module`/`variable`/`output` (:152 filters
    everything else), with indices in bracket form — `[0]`, `["key"]`.
- `internal/parser/parser.go:260-265` — module children are already in the same
  flat `parsedResources.resources` map, keyed
  `module.<instance>.resource.<type>.<name>` (`fqrn.go:181-202`), because
  `parseModule` recurses at parse time. **Cross-file and cross-module resolution
  is therefore a plain map lookup** — no fetching.

**Tests**

- Unit tests for the resolver itself: attribute-carrying reference against an
  attribute-free key; module-scoped key; `variable.`/`output.` forms (which
  `fqrn.go:190-192` renders differently).
- `internal/test_fixtures/config/simple/container.xcl` — 10 links asserted at
  `parse_test.go:149-160`; a good resolution corpus.

**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: Single agent — the correctness hinges on one subtle ordering
fix best held in one head.

---

### Phase 2.2: Report references that name nothing

**File changes**

- `internal/parser/validate.go` — add stage 2. For every resource in
  `parsedResources.resources`, take its `Meta.Links` and resolve each; collect a
  `ParserError` per miss. Position comes from the *referring* resource's
  `Meta.File/Line/Column` (set at `parser.go:502-505`), since the link string
  carries no position of its own.
  - **Accuracy limit worth recording:** `Meta` positions point at the resource
    block, not the exact expression. So "the file and position it appears at" is
    satisfied at block granularity. Finer positions would need
    `hclsyntax.Expression.Range()` captured during `processExpr` — a larger
    change. **Decide and state** which is delivered; block granularity is the
    proposed default.
- `internal/parser/parser.go:778-817` — the existing cycle check. **The silent
  fall-through at :788 (`if depResource, ok := ...; ok` with no `else`) is the
  case stage 2 now reports.** Four stacked silent-miss paths: :787 (nil map),
  :788 (key miss), :790-792 (meta failure), :796-799 (unparseable FQRN).
  - Decide whether the cycle check keeps its own lookup or defers to stage 2.
    It catches only direct A→B→A (:803-813) and only when B is already parsed;
    real cycle detection is `d.Validate()` at `parser.go:894`, which reports
    with **no file/line**. Bringing cycles into stage 2 would satisfy "each
    problem says what and where" for them too — **in scope only if cheap**;
    otherwise record as a known gap.
- `internal/parser/parser.go:697-705` — `getUniqueResourceLinks` **discards**
  the `AppendUniqueDependency` error at :704. Fix per the explicit-error-handling
  convention.

**Tests**

- Four new fixtures: undefined resource, variable, output, module. Each its own
  test, asserting the message names the missing item.
- One fixture with several bad references across **two** files, asserting all are
  reported (spec criterion "multiple faults ... spread across more than one file").
- `internal/test_fixtures/config/cyclical/pass/` — must still pass; it proves
  cross-module references are not cycles.

**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: 2 parallel agents — stage-2 logic and fixtures/tests.

---

### Phase 2.3: Check references into modules, and attribute unobtainable modules correctly

**File changes**

- `internal/parser/validate.go` — module handling in stage 2. Module outputs are
  ordinary `output` blocks already parsed into the working set under
  `module.<instance>.output.<name>`, so `module.foo.some_output` is a map lookup.
  - **Open assumption to verify first (research.md #6):** the spec-phase note
    that `module.SubContext` is unpopulated until the walk
    (`callbacks.go:120-125`) is believed irrelevant to a *static existence*
    check. Confirm a module output resolves without `SubContext`. If it does
    not, **STOP and ask** — it would change the approach.
- `internal/parser/validate.go` — **suppression rule.** When a module is itself
  reported (unobtainable contents, or the Phase 1.3 recursion guard), suppress
  reference errors for references whose FQRN `Module` matches. The spec is
  explicit that such references "must not additionally be reported as naming
  things that do not exist". Implement as a filter after stage 2 collection.
- `internal/resources/module.go:17` — `Version` declared, never read. Remains
  unread; a declared version has no bearing on what is retrieved.

**Tests**

- `internal/test_fixtures/config/modules/modules.xcl:60-90` — references to
  `module.consul_1.output.container_resources_cpu` etc. Must keep passing.
- New fixture: a module output that is **not** declared → fails naming it.
- New fixture: a module whose source does not exist, with a reference into it →
  one problem against the module, and **no** problem for the reference. This is
  the suppression test and the easiest to get wrong.

**Complexity**: Medium
**Token estimate**: ~25k
**Agent strategy**: Single agent — the suppression rule couples the two halves.

---

### Phase 3.1: Establish what properties a type has

**File changes**

- **New file** `internal/parser/types.go` (or similar) — the type name resolver.
  `reflect.Type` → map of HCL name to type.
- **The authoritative rule is external.** `go.mod:98` replaces `zclconf/go-cty`
  with `jumppad-labs/go-cty`. In the module cache,
  `cty/gocty/helpers.go:34-54`:
  ```go
  attrName := field.Tag.Get("hcl")
  attrs := strings.Split(attrName, ",")
  if len(attrs) > 0 && attrs[0] != "" { ret[attrs[0]] = i }
  ```
  Upstream reads `cty` (`zclconf/go-cty@v1.15.0/cty/gocty/helpers.go:36`).
  **Mirror the fork, not upstream, and not the `json` tag.**
- Anonymous embeds flatten **recursively** — `cty/gocty/type_implied.go:107-121`
  in the fork (absent upstream at v1.15.0 `:83-107`).
- **Both embed spellings must work.** Ten occurrences in tree:
  - `hcl:"rm,remain"` — `internal/test_fixtures/plugin/structs/container.go:14`.
    Non-empty first segment → registers an attribute named `rm` **and** flattens.
    The lone outlier.
  - `hcl:",remain"` — `structs/container.go:96,102`, `structs/network.go:10`,
    `structs/template.go:13`, `internal/resources/{output,module,variable,root}.go`.
- **Fields with no `hcl` tag are invisible** — `types/resource.go:36,40,45`
  (`Meta.Properties`, `Meta.Links`, `Meta.Status`) carry only `json`. Must be
  **absent** from the result.
- **Two sources of type information:**
  - Builtins — real Go types via `types/register.go:23-39`
    (`reflect.TypeOf(t).Elem()` at :26). `internal/resources/default.go:6-13`
    registers `variable`, `output`, `module`, `root`.
  - Plugin types — **anonymous** `reflect.StructOf` structs
    (`internal/schema/deserialize.go:130-134`), rebuilt from JSON schema
    (`plugins/plugin.go:39`, generated at depth **10** at `plugins/plugin.go:107`).
    Tags are reconstructed verbatim (`deserialize.go:66,95,113,123`), so the same
    tag-driven walk works — but the depth-10 truncation is real (see Phase 3.3).
- `internal/parser/parser.go:404` already instantiates the concrete typed
  resource **at parse time**, so a `reflect.Type` is available for every parsed
  resource without extra work.

**Tests**

- Direct unit tests, not only through the parser. Cases: `hcl:"rm,remain"`
  yielding both `rm` and the flattened names; `hcl:",remain"` yielding only
  flattened names; a no-`hcl`-tag field absent; multi-level embedding
  (`Container` → `ContainerBase` → `ResourceBase`, `structs/container.go:95-99`);
  `optional` and `block` both stripping to the bare name.

**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: Single agent — small, subtle, and must not be split.

---

### Phase 3.2: Check property paths, continuing through member selection

**File changes**

- **New file** `internal/parser/properties.go` — the path checker.
  `(targetType, attributePath) → problem or nil`.
- **Walk `reflect.Type` segment by segment.** Do **not** route through
  `internal/schema.GenerateSchemaFromInstance` (`serialize.go:13`) — measured
  defects: returns JSON bytes (`serializeAttribute` at :23 is unexported);
  silent depth truncation (`:27-29` returns `nil`, parent omits child — depth=1
  gave 8 of 86 nodes for `ContainerBase`); no cycle guard (137 KB at depth 100
  for a self-referential struct). Segment-at-a-time needs no depth cap and is
  cycle-safe because path length bounds the walk.
- **Splitting the path.** `FQRN.Attribute` is flat and dotted
  (`fqrn.go:80`). Split on `.`, and extract `[N]` / `["key"]` — for `resource`
  references these are **not** pre-decomposed. Reference strings from
  `processScopeTraversal` (`exp.go:160-170`) use bracket form; dotted numeric
  indices also occur in fixtures (`interpolation.xcl:36`
  `volume.0.source`). **Handle both.**
- **The member-selection rule.** On `reflect.Slice`, `reflect.Map`,
  `reflect.Array`, `reflect.Ptr`: step to `Elem()` and continue against the
  element type. `*` and `[N]` and a map key all behave the same — they identify
  *which*, never *what type*. Mirrors `serialize.go:60-70`, which already
  collapses these to `Elem()`.
  - Verified by probe: `Volumes` is `[]structs.Volume` with `Destination`
    beneath; `CreatedNetworksMap` is `map[string]structs.Network` with `Subnet`
    beneath; `Networks` is `[]structs.NetworkAttachment` with `ID` beneath.
- **The canonical negative case is `volume.*.destnation`** — nothing existing
  covers a typo *after* a splat, and an implementation that stops at the splat
  passes every other test while failing this one.

**Tests** — positive and negative strictly separated (convention):

| case | fixture | expect |
|---|---|---|
| `volume.*.destination` | `config/interpolation/interpolation.xcl:44` | accept |
| `volume.*.destnation` | **new** | reject, names `destnation` |
| `network[0].id` | `config/simple/container.xcl:85` | accept |
| `volume.0.source` | `config/interpolation/interpolation.xcl:36` | accept |
| `created_network_map.one.subnet` | `config/cyclical/fail/cyclical.xcl:7` | accept |
| `resource.network.test.nam` | `config/process_error/bad_interpolation.xcl:11` | **reject** |

- `parse_test.go:943-961` —
  `TestParseFileReturnsConfigErrorWhenResourceInterpolationError` **currently
  asserts the opposite**: `require.False(t, ce.ContainsErrors())` (:957) and
  `Level == ParserErrorLevelWarning` (:960) for that `.nam` typo. **Invert it.**
  This is deliberate, not a regression.

**Complexity**: High
**Token estimate**: ~45k
**Agent strategy**: Parallel analysis, sequential integration. The riskiest
phase: one subtle rule, many cases, and an over-eager implementation passes all
the negative tests while breaking real configuration.

---

### Phase 3.3: Stop checking where the type is no longer knowable

**File changes**

- `internal/parser/properties.go` — the stopping rules. **Accept and stop** when:
  - the element type of a collection is itself unknowable — e.g.
    `Env map[string]string` (`structs/container.go:24`) has a **scalar** value
    type, so `env.MY_VAR` ends there;
  - a `cty.Value`-typed field is reached (becomes `cty.DynamicPseudoType`,
    fork `type_implied.go:77-80`) — e.g. `Template.Vars`
    (`internal/test_fixtures/plugin/structs/template.go:19`);
  - `reflect.Interface` / `any` is reached — e.g.
    `Meta.Properties map[string]any` (`types/resource.go:36`), though that field
    is HCL-invisible so it should not be reachable by name anyway;
  - a plugin schema was **truncated** at generation depth 10
    (`plugins/plugin.go:107`). Cannot distinguish "absent" from "not looked at",
    so **must accept**. This is the false-rejection risk.
- **Never reject at an unknowable boundary.** A rejection here is a false
  negative that breaks working configuration — strictly worse than missing a
  typo, which is merely the status quo.
- Provider-populated fields are **not** specially marked: `// output` at
  `structs/container.go:39` is a bare comment above a group, with no
  machine-readable structure. `CreatedNetworks` / `CreatedNetworksMap` are
  ordinary tagged fields and resolve normally — they need no special case. The
  spec's "property populated during apply is accepted" holds because the field
  exists on the type, not because it is flagged.

**Tests**

- `config/functions/default.xcl:48` —
  `resource.container.with_networks.created_network_map != null ? values(...).*.meta.name : []`.
  Conditional + function + splat + `meta` traversal. Must be **accepted**. The
  best single regression case in the repo.
- `config/simple/container.xcl:52-54` — `resource.network.onprem.meta.id`
  through the flattened `ResourceBase`. Must be accepted.
- New: a reference through a `cty.Value` field (`template.vars.anything.deep`) —
  accepted.
- New: a property beneath `env.SOME_KEY` — accepted (scalar map value).

**Complexity**: Medium
**Token estimate**: ~30k
**Agent strategy**: Single agent, continuing directly from Phase 3.2 — the same
walk, and the stopping rules are meaningless apart from it.

---

### Phase 4.1: Remove the advisory/fatal classification

**File changes**

- `errors/parser_error.go:23` — delete the `Level` field. Note `Details` (:21)
  is **also dead** (never read or written) — remove it too, or leave it
  deliberately; decide rather than overlook.
- `errors/parser_error.go:77-85` — `NewParserError`: drop the `level` parameter.
- `errors/parser_error.go:88-104` — `NewParserErrorFromResource`: same.
- `errors/parser_error.go:174` — `NewParserErrorFromHCLDiag` hard-codes
  `Level: ParserErrorLevelError`; drop.
- `errors/parser_error.go:13-14` — delete both constants.
- `errors/config_error.go:26-34` — delete `ContainsWarnings()`.
- `errors/config_error.go:37-45` — delete `ContainsErrors()`.
- `errors/config_error.go:48-53` — delete `isParserError`.
  - **Note this is a public API break beyond what the spec names** (see
    assumption DS1). `ContainsErrors()` would become a synonym for
    `len(Errors) > 0`; keeping it would preserve the illusion that some problems
    are advisory. Flagged at walkthrough.
- `internal/parser/parser.go:923-938` — delete `checkIfErrorInFunction`.
- `internal/parser/callbacks.go:107-118` — its only call site; construct the
  error without a level.
- `errors/parser_error.go:27-74` — `Error()` **never reads `Level`**, so
  **no rendered output changes**. Do not "fix" the rendering.
- **~50 write sites** to update as the `level` argument disappears:
  `internal/parser/parser.go` (:299, :334, :385, :398, :410, :423, :435, :447,
  :459, :471, :483, :497, :519, :572, :584, :596, :609, :633, :651, :662, :680,
  :752, :810, :862, :871), `internal/parser/dag.go` (:70, :81),
  `internal/parser/callbacks.go` (:69, :139, :160, :219, :236, :254),
  `internal/parser/util.go` (:221, :231, :242, :255, :289, :302, :511, :558).
  Also the commented-out `parser.go:545`.
- `internal/parser/parser.go:357-365` — the unknown-stanza error is currently
  **warning**-level yet still returns and aborts. Becomes an ordinary failure.

**Tests**

- `errors/config_error_test.go:16-22` — `TestContainsWarningsReturnsTrue`:
  delete.
- `errors/config_error_test.go:24-30` — `TestContainsErrorsReturnsTrue`: delete
  or rewrite against `len(Errors)`.
- `errors/config_error_test.go:32-38` — asserts the exact rendered string.
  **Must still pass unchanged** — a good guard that rendering did not shift.
- `parse_test.go:917,920,937,940,957,960` — level assertions; remove or invert.
- `parse_test.go:919,939,959` — unchecked `ce.Errors[0].(*errors.ParserError)`
  will **panic** if any non-`ParserError` reaches the collection. Ensure
  validation emits only `*ParserError`, and prefer selecting by content over
  index now that several problems may be present.
- `errors/parser_error_test.go` — `TestParserErrorHighlightsLine` and
  `TestParserErrorNonErrorLineGrey` **fail at baseline** (`.hcl`/`.xcl` drift).
  Determine whether they are collateral or pre-existing; do not silently adopt.

**Complexity**: Medium (wide but mechanical)
**Token estimate**: ~35k
**Agent strategy**: 2-3 parallel agents by package (`errors/`,
`internal/parser/`, tests), sequential final compile — the signature change
touches every call site at once.

---

### Phase 4.2: Bring documentation and project knowledge in line

**File changes**

- **Knowledge entry** `architecture/ux-flow.md` (repo tier, store `xclconfig`,
  at `.spektacular/knowledge/architecture/ux-flow.md`). Section
  "4. Validate Without Applying" shows `diff, err := config.Validate(...)` and
  iterating `diff.Creates`; the component map lists
  `Diff - Represents changes between states`. **Both must go.** Replace with the
  error-only contract and a description of the three stages.
  - **Write via `spektacular knowledge write`**, not the `Write` tool. Per the
    knowledge workflow, **propose the exact content to the user and wait for
    explicit confirmation** before writing.
  - The user confirmed during planning that this entry is updated within this
    work.
- `docs/overview.md:40,63` — references `Diff`. Update.
- `docs/README.md:27` — file-map table lists `diff.go`. Remove the row.
- `TODO.md:11,146,147` — historical notes mentioning Diff. Leave as history, or
  annotate; do **not** rewrite the record.
- `config.go:61-66` — the `Validate` doc comment (already updated in Phase 1.2;
  verify it survived).

**Tests**

None — documentation only. Verify by reading back the knowledge entry with
`spektacular knowledge read` after writing.

**Complexity**: Low
**Token estimate**: ~12k
**Agent strategy**: Single agent, sequential. Must run **after** 4.1 so the
documented behaviour is the final one.

---

### Phase 4.3: Confirm nothing that worked before has broken

**File changes**

None expected. This phase verifies; any change it prompts belongs to the phase
that introduced the problem.

**The baseline this is measured against** (branch `v2`, commit `858013a`, before
any change — see `.spektacular/work/.../baseline.md`):

- `go build ./...` — **clean**.
- **3 test packages do not compile**: `internal/functions`
  (`functions_test.go:192`, undefined `setupParser`), `plugins/registry`
  (`plugin_discovery_test.go:13`, undefined `newTestPluginSetup`), `example`.
- **10 failing tests**:
  - *In scope — this work fixes them:*
    `TestParseFileReturnsConfigErrorWhenResourceBadlyFormed` (:903, the
    motivating test), `TestParseFileReturnsConfigErrorWhenInvalidFileFails`
    (:963, wrong error **type**: `*fmt.wrapError` not `*errors.ConfigError`).
  - *Must be decided, not assumed:* `TestParserEventCallback` (:1001),
    `TestParserEventErrorCallback` (:1071),
    `TestParserEventForVariablesOutputsLocals` (:1128) — all on the error
    production path this work rewrites; `TestParserErrorHighlightsLine`,
    `TestParserErrorNonErrorLineGrey` (`errors/`) — on the rendering path.
  - *Out of scope:* `TestDestroyLifecycle` (:1189, **panics** — mock
    `StateStore.Exists` has no expectation, `parser.go:189`),
    `TestReadResourceFromFileAtLocation`, `TestLineFromFileAtLocation` (root,
    `.hcl`/`.xcl` drift).

**Exit criterion — "all tests pass" is NOT usable here.** Instead:

1. The two in-scope failures now pass.
2. Every new test passes.
3. Each of the five "must be decided" tests has a recorded decision: fixed,
   updated to a new contract, or left failing with a stated reason.
4. **No test that passed at baseline now fails.**
5. Every deliberately-changed assertion has a justified expected value — derived
   from what the stage should find, never read back from a test run (T3).

**Verification**

- Re-run the baseline commands and diff against the table above.
- Apply `config/simple/container.xcl` and `config/modules/modules.xcl` before and
  after; confirm the same resources are produced (41 resources asserted at
  `parse_test.go:349,398`; 10 at `:291` for `interpolation.xcl`).
- `go vet ./...` — pre-existing findings in `plugins/` and `example/` must not
  grow.

**Complexity**: Low
**Token estimate**: ~15k
**Agent strategy**: Single agent, sequential — a judgement exercise requiring
the whole change in view.


## Testing Strategy

Per-phase detail. Conventions are binding: testify `require`, **never**
table-driven, **never** mix positive and negative in one function, verbosity
over abstraction. Match `parse_test.go` style — `filepath.Abs` + `setupParser(t)`
+ `p.Parse(false, path)`, naming `Test<Subject><Behaviour>` and
`Test...ReturnsConfigErrorWhen<Cause>`.

**Spec success metrics:** the spec states success is measured by its 21
acceptance criteria, not by separate metrics. All 21 are **behavioural**; none
is classified *Manual — captured in the implementation test plan*. Its second
metric (user reports of confusing errors) is forward-looking and has nothing to
assert today.

| phase | tests |
|---|---|
| 1.1 | `parse_test.go:903` should pass; decide new counts at `:915`, `:870`, `:885`; new fixture with two malformations in one file |
| 1.2 | **First ever tests for `Config.Validate`** (`config_test.go` has 2 live tests; `:33-457` commented out). Valid config → no error, nothing created. Invalid → error, nothing created. Observe via `TestPlugin.GetCreatedResources()` |
| 1.3 | Unregistered type names the type; mutually-including modules report rather than crash (**add a timeout guard**) |
| 2.1 | Unit tests for the resolver: attribute-carrying reference vs attribute-free key; module-scoped keys; `variable.`/`output.` forms |
| 2.2 | One test per undefined kind (resource, variable, output, module); one with several bad refs across **two** files asserting all are reported |
| 2.3 | Module output resolves; undeclared output fails naming it; **suppression** — unobtainable module reports once and its inbound refs are not also reported |
| 3.1 | Direct unit tests: `rm,remain` yields both `rm` and flattened names; `,remain` yields only flattened; no-`hcl`-tag field absent; multi-level embedding; `optional`/`block` both strip |
| 3.2 | Positive/negative **pairs in separate functions**: `volume.*.destination` accept vs `volume.*.destnation` reject; invert `parse_test.go:943` |
| 3.3 | `functions/default.xcl:48` accepted (best single regression case); `cty.Value` traversal accepted; `env.KEY` accepted |
| 4.1 | Delete `TestContainsWarningsReturnsTrue`; `errors/config_error_test.go:32-38` (exact rendered string) **must still pass unchanged** — the guard that rendering did not shift |
| 4.3 | Diff against the baseline table above |

**Assertion discipline:** assert on problem **content**, not bare counts — a
count matching for the wrong reason tells us nothing, and count-only assertions
are exactly what let the current defect hide. Every changed expected count must
be **decided** from what the stage should find, never read back from a run.

## Project References

| what | where |
|---|---|
| Insertion point for the gate | `internal/parser/parser.go:256-272` |
| Per-file parsing / severity block | `internal/parser/parser.go:284-318` |
| Duplicate classifier | `internal/parser/parser.go:923-938` → `callbacks.go:110` |
| Silent dependency fall-through | `internal/parser/parser.go:778-817` (esp. `:788`) |
| Module parsing + recursion | `internal/parser/parser.go:565-692` (`:674-689`) |
| Reference extraction | `internal/parser/exp.go:22`, `:145-178` |
| FQRN parse / render | `internal/resources/fqrn.go:53`, `:167-177`, `:205` |
| Error types | `errors/parser_error.go:13-24`, `errors/config_error.go:11-53` |
| Public entry points | `config.go:67` (`Validate`), `config.go:94` (`Apply`) |
| To be deleted | `diff.go` (whole file) |
| Schema generation | `internal/schema/serialize.go:13`, `:60-70`; `types.go:3-9` |
| Plugin type rebuild | `internal/schema/deserialize.go:130-134` |
| Type registry | `types/register.go:23-39`; `internal/resources/default.go:6-13` |
| Forked naming rule | `go.mod:98`; cache `cty/gocty/helpers.go:34-54` |
| Test style template | `internal/parser/parse_test.go:43-83` (`setupParser`) |
| Test plugin / observation | `internal/parser/test_plugin.go:34-54`, `:123-137` |
| Knowledge entry to update | `.spektacular/knowledge/architecture/ux-flow.md` |

### Fixtures already covering spec cases

| case | fixture |
|---|---|
| syntax error | `config/process_error/bad_format.xcl:7` (missing `{`) |
| property typo on a known type | `config/process_error/bad_interpolation.xcl:11` (`.nam`) |
| splat into element type | `config/interpolation/interpolation.xcl:44` |
| dotted numeric index | `config/interpolation/interpolation.xcl:36` |
| collection index | `config/simple/container.xcl:85` (`network[0].id`) |
| map key then property | `config/cyclical/fail/cyclical.xcl:7` |
| conditional + splat + apply-populated | `config/functions/default.xcl:48` |
| cross-file / module outputs | `config/modules/modules.xcl` + `config/single/container.xcl` |
| module refs are not cycles | `config/cyclical/pass/` |

**New fixtures needed:** a bad property *after* a splat (`volume.*.destnation`
— the canonical negative case, nothing covers it); an unregistered resource
type; two mutually-including module sources; a file with two separate
malformations.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Per-phase estimates: 1.1 ~25k · 1.2 ~40k · 1.3 ~30k · 2.1 ~30k · 2.2 ~30k ·
2.3 ~25k · 3.1 ~30k · 3.2 ~45k · 3.3 ~30k · 4.1 ~35k · 4.2 ~12k · 4.3 ~15k.

Phases 1.2 and 3.2 are the High-tier work. 3.2 is the riskiest in the plan: one
subtle rule, many cases, and an over-eager implementation passes every negative
test while breaking real configuration — so its positive cases matter more than
its negative ones.

## Migration Notes

**Breaking public API changes.** Acceptable because the user confirmed the
library has no consumers yet, but they are real:

1. `Config.Validate` returns `error` instead of `(*Diff, error)`.
2. `Diff` and `buildDiff` are **deleted** (`diff.go`).
3. `ConfigError.ContainsErrors()` and `ContainsWarnings()` are **deleted** —
   callers test `len(Errors) > 0`. This goes one step beyond what the spec names
   explicitly (assumption DS1): once severity is gone, `ContainsErrors()` would
   always agree with `len(Errors) > 0`, and a method that always agrees with a
   simpler expression is a trap.
4. `ParserError.Level` is deleted, and `level` leaves the
   `NewParserError` / `NewParserErrorFromResource` signatures (~50 call sites).

**No data migration.** Nothing persisted changes shape; validation runs before
anything reaches state.

**No rendered output changes.** `ParserError.Error()` never read `Level`
(`parser_error.go:27-74`) — an advisory problem already printed identically to a
fatal one. Only the *decision* about usability changes.

**Behavioural change users will notice:** configurations that previously applied
with warnings will now be rejected. That is the intent of the spec, not a
regression — but it is the migration story for anyone with a configuration
containing a property typo or a dangling reference.

## Performance Considerations

Not a performance-motivated change, and no spec target exists. Three notes:

- **The property walk is bounded by the reference, not the type.** Resolving
  segment-by-segment does work proportional to the *path length*, not to the
  size of the resource schema — and needs no depth cap. This is why it
  terminates on self-referential types where schema generation produced 137 KB
  at depth 100.
- **Validation adds one pass over already-parsed data.** No file is re-read and
  no module re-fetched: module children are already in `parsedResources` from
  parse time. Cost is roughly linear in the number of references.
- **`Validate` should get materially cheaper**, since it stops before the DAG
  walk and decode rather than performing them to produce a diff.
- One pre-existing inefficiency, untouched: `parseResourcesInFile` creates a
  fresh `hclparse.NewParser()` per file (`parser.go:285`), so a file reachable
  through two module paths is re-read and re-parsed.
