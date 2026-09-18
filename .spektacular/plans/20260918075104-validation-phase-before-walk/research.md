---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Research: 20260918075104-validation-phase-before-walk

All findings verified against source in the `xclconfig` repo
(`spektacular repo list` → root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).
This is the only registered repo, and all changes land in it.

## Alternatives considered and rejected

### A1. Fix the severity classifier in place, keep the single walk

Change the default level at [internal/parser/parser.go:290](internal/parser/parser.go#L290)
from warning to error and correct `checkIfErrorInFunction`
([parser.go:923](internal/parser/parser.go#L923)) rather than introducing a
distinct stage.

**Rejected.** It cannot satisfy the spec and does not address the cause. The
level is doing double duty as a phase marker precisely because validation and
resolution share one path — `Config.Validate` → `Parse(false, …)` and
`Config.Apply` → `Parse(true, …)` differ only by `executePlugins`, checked at
[callbacks.go:156](internal/parser/callbacks.go#L156), well after
`gohcl.DecodeBody` at [callbacks.go:107](internal/parser/callbacks.go#L107). So
that shared path cannot hard-fail on unresolved interpolation. It also leaves
"nothing is acted upon unless valid" unmet: resources are still decoded and
providers still reached before the verdict exists.

### A2. Reuse `GenerateSchemaFromInstance` for the property check

Call [internal/schema/serialize.go:13](internal/schema/serialize.go#L13) per
resource type and walk the returned `Attribute` tree.

**Rejected as the primary mechanism**, on three measured grounds (probes run and
removed; see `schema_findings.md`):

1. It returns **JSON bytes**, not the tree — `serializeAttribute`
   ([serialize.go:23](internal/schema/serialize.go#L23)) is unexported. Every
   check would marshal then unmarshal to walk a path.
2. **Silent truncation causes false rejections.** `depth=1` yields 8 of 86 nodes
   for `ContainerBase`; the cap returns `nil`
   ([serialize.go:27-29](internal/schema/serialize.go#L27)) and the parent
   simply omits the child. A checker cannot distinguish "absent" from "truncated"
   and would reject valid configuration. Plugin schemas arrive pre-generated at
   depth 10 ([plugins/plugin.go:107](plugins/plugin.go#L107)), so this is live,
   not hypothetical.
3. **No cycle guard.** A self-referential struct produced 137 KB at depth 100 —
   no panic, just growth. Nothing tracks visited types.

Retained in part: the slice/map→element collapse at
[serialize.go:60-70](internal/schema/serialize.go#L60) is exactly the right
semantics, and the plugin path genuinely has nothing else (plugin types are
anonymous `reflect.StructOf` structs,
[deserialize.go:130](internal/schema/deserialize.go#L130)). The recommendation
is `reflect.Type` traversal for the walk, not to discard the schema tree where
it is the only available source.

### A3. Introduce a second reference extractor for validation

Write a dedicated traversal instead of reusing `processExpr`
([internal/parser/exp.go:22](internal/parser/exp.go#L22)).

**Rejected.** The spec's Technical Approach rules it out directly — two
extractors would risk disagreeing about what a configuration refers to.
`processExpr` already collects structurally without evaluating, handles ten
expression forms including splat, conditional and binary-op, and already runs at
parse time via `getUniqueResourceLinks`
([parser.go:697](internal/parser/parser.go#L697)) to feed the DAG. Reusing it
means validation checks exactly what the graph will act on.

Worth noting it has known blind spots — `IndexExpr`, `RelativeTraversalExpr`,
`ForExpr` produce no references, and `SplatExpr` traverses `Source` but not
`Each` ([exp.go:122-131](internal/parser/exp.go#L122)). Those are pre-existing
and shared with the DAG; widening them is not this work.

### A4. Validate each file independently

**Rejected** by the spec's constraint, and confirmed unnecessary. Files
reference one another, and the parse loop at
[parser.go:249-254](internal/parser/parser.go#L249) already processes every
discovered file into `p.parsedResources` before anything else runs — so
whole-configuration visibility is free at the insertion point.

### A5. Keep `Diff` by walking inside `Validate`

**Rejected**, and already settled in the spec. Producing a diff requires
resolution, which is what makes validation non-distinct. Blast radius confirmed
small: one production consumer ([config.go:86](config.go#L86)), one call site,
**zero test coverage**, one doc reference.

## Chosen approach — evidence

**The insertion point.** [parser.go:256-272](internal/parser/parser.go#L256) —
between the parse gate (`if len(ce.Errors) > 0 { return nil, ce }`) and
`p.walk(...)`. At that point every resource exists in `p.parsedResources`,
every `Meta.Links` is populated, and no body is decoded. Both public entry
points funnel through `Parse` ([config.go:80](config.go#L80),
[config.go:107](config.go#L107)), so one gate covers both — satisfying the
spec's "available on its own, and automatically before applying".

**Stage 1 needs restructuring, not just a level flip.**
`parseResourcesInFile` declares `[]error` but every path returns a
**one-element** slice, and reports only `diag[0]`
([parser.go:305-315](internal/parser/parser.go#L305)) while iterating the whole
set purely to scan for a promotion string. Measured: an unterminated string
yields 3 diagnostics, of which 2 are discarded today.

**Stage 2's error case already exists as a silent fall-through.** The cycle
check at [parser.go:788](internal/parser/parser.go#L788) looks up each
dependency in `parsedResources.resources` with **no `else` branch** on a miss.
Keys never carry an attribute
([fqrn.go:167-177](internal/resources/fqrn.go#L167)) while extracted references
can, so `StringWithoutAttribute()`
([fqrn.go:205](internal/resources/fqrn.go#L205)) must be applied *before* the
lookup, not after as at [parser.go:800](internal/parser/parser.go#L800).

**Stage 3 is feasible; the type information is statically reachable.** Probed
`ContainerBase`: `Volumes` reports `Type: "[]structs.Volume"` *and* carries the
element's fields as `Properties`; `CreatedNetworksMap` reports
`map[string]structs.Network` with `Subnet` beneath. That is precisely the
unknown-member / known-type distinction the spec's two constraints require.
`Env` (`map[string]string`) has no `Properties` — the natural stopping point.

**The authoritative naming rule is in a forked dependency.**
[go.mod:98](go.mod#L98) pins `jumppad-labs/go-cty`, whose
`cty/gocty/helpers.go:34-54` reads the **`hcl`** tag where upstream reads `cty`:
`strings.Split(tag.Get("hcl"), ",")[0]`, skipping empty first segments.
Anonymous embeds flatten recursively (`type_implied.go:107-121`, a fork-only
addition). **No first-party file in this repo parses HCL tags today** —
`internal/schema` stores the raw tag string and never reads it. Stage 3 must
introduce this, once.

**Error plumbing already fits the spec.** `errors.ConfigError` is a collection
(`Errors []error`, `AppendError`, [config_error.go:11-23](errors/config_error.go#L11))
and `ParserError` already carries `Filename`, `Line`, `Column`
([parser_error.go:17-24](errors/parser_error.go#L17)) — exactly the "what and
where" required. `ParserError.Error()` never reads `Level`
([parser_error.go:27-74](errors/parser_error.go#L27)), so removing severity
changes no rendered output.

**Severity removal is narrow.** 57 references across 8 files, but only **two
read sites**: [config_error.go:28](errors/config_error.go#L28) and
[config_error.go:49](errors/config_error.go#L49). Every other is a write that
collapses when `level` leaves the constructor signatures.

## Files examined

- `xclconfig:internal/parser/parser.go:182-281` — `Parse` flow; insertion point confirmed at 256-272
- `xclconfig:internal/parser/parser.go:283-318` — `parseResourcesInFile`; returns first error only, reports `diag[0]` only
- `xclconfig:internal/parser/parser.go:290-302` — severity block, inlined duplicate of the helper
- `xclconfig:internal/parser/parser.go:404` — `pluginRegistry.CreateResource`; typed instance exists at parse time
- `xclconfig:internal/parser/parser.go:565-692` — `parseModule`; local-only source resolution, **no recursion cycle guard**
- `xclconfig:internal/parser/parser.go:697-736` — `getUniqueResourceLinks`; inferred-link error discarded at 704
- `xclconfig:internal/parser/parser.go:778-817` — cycle check; four stacked silent-miss paths, catches only A→B→A
- `xclconfig:internal/parser/parser.go:883-921` — `walk`; `previousState` accepted but unused (902-904)
- `xclconfig:internal/parser/parser.go:923-938` — `checkIfErrorInFunction`; one call site
- `xclconfig:internal/parser/callbacks.go:107-118` — `DecodeBody` + classification; the live promotion site
- `xclconfig:internal/parser/exp.go:22-140` — `processExpr`; ten expression forms, no evaluation
- `xclconfig:internal/parser/exp.go:145-178` — `processScopeTraversal`; root filter, bracket-index format
- `xclconfig:internal/parser/util.go:29-43` — `findXclFiles` is **non-recursive**
- `xclconfig:internal/resources/fqrn.go:53-141` — `ParseFQRN`; `Attribute` is a flat dotted string, undecomposed for resources
- `xclconfig:internal/resources/fqrn.go:167-177` — `FQRNFromResource` never sets `Attribute` — the key/reference mismatch
- `xclconfig:internal/resources/fqrn.go:205-224` — `StringWithoutAttribute` already exists
- `xclconfig:internal/schema/serialize.go:13-79` — schema generation; JSON out, depth cap, slice/map→element collapse
- `xclconfig:internal/schema/types.go:3-9` — `Attribute`; `Type` is a string, no separate element/HCL-name fields
- `xclconfig:internal/schema/deserialize.go:130-182` — plugin types are anonymous structs; `parseType` decomposes type strings
- `xclconfig:types/register.go:23-39` — `CreateResource`; builtins give a real `reflect.Type`
- `xclconfig:types/resource.go:36,40,45` — `Meta.Properties/Links/Status` have no `hcl` tag → HCL-invisible
- `xclconfig:errors/config_error.go:37-53` — `ContainsErrors` / `isParserError`; non-`ParserError` defaults to error
- `xclconfig:errors/parser_error.go:13-24,27-74` — level constants, fields; `Error()` ignores `Level`
- `xclconfig:config.go:67-88` — `Config.Validate`; returns `(*Diff, error)`, never mutates state
- `xclconfig:config.go:94-123` — `Config.Apply`; identical parser setup, `Parse(true, …)`
- `xclconfig:diff.go:11-67` — `Diff` + `buildDiff`; one consumer, no tests
- `xclconfig:go.mod:98` — **forked go-cty** replace directive; the authoritative naming rule
- `xclconfig:internal/test_fixtures/plugin/structs/container.go:14` — `hcl:"rm,remain"`, the lone outlier among ten embeds
- `xclconfig:internal/test_fixtures/config/process_error/bad_interpolation.xcl:11` — `.nam` typo; ready-made stage-3 positive case
- `xclconfig:internal/parser/parse_test.go:43-83` — `setupParser`; the test-style template
- `xclconfig:internal/parser/parse_test.go:873-885` — asserts **1** error from 3 broken files; encodes the defect
- `xclconfig:internal/parser/parse_test.go:943-961` — asserts a property typo is a **warning**; asserts the spec's opposite
- `xclconfig:config_test.go:16-31` — only two live tests; `Validate`/`Diff` untested

## External references

- **`github.com/jumppad-labs/go-cty`** (module cache, `cty/gocty/helpers.go:34-54`,
  `cty/gocty/type_implied.go:107-121`) — the fork pinned at `go.mod:98`. Reads
  `hcl` tags where upstream reads `cty`, and adds recursive anonymous-embed
  flattening. **This is the authoritative definition of a type's valid property
  names**; matching Go field names instead would be wrong for every field.
- **`hashicorp/hcl/v2` (patched v2.21.0)** — per the spec-phase audit, all 85
  diagnostic summaries reachable from `ParseXCLFile` are `DiagError`; the sole
  `DiagWarning` in the module is in a CLI tool never called. Confirms hard-failing
  the parse path cannot regress interpolation leniency.
- **`hashicorp/go-cty` `gohcl.DecodeBody`** ([callbacks.go:107](internal/parser/callbacks.go#L107))
  — honours `optional`/`block`/`remain` when decoding; the behaviour stage 3
  must predict without invoking.

## Prior plans / specs consulted

- **`spektacular plan file list`** — empty. No prior plan exists for this repo;
  this is the first.
- **Spec `20260714080036-restore-module-support.md`** — the earlier module work.
  Relevant because this spec depends on module children already being parsed:
  `parseModule` resolves local sources and recurses at parse time, so module
  contents are available to stages 2 and 3 with no fetching. Read as historical
  intent, not current behaviour.
- **Knowledge `decisions/parser-refactor-analysis.md`** (2025-12-27) — an
  implementation guide for a refactor that has since landed (the `parsed` type it
  calls "MISSING" now exists at [parser.go:39](internal/parser/parser.go#L39)).
  Its rationale supports this work: "Can't build DAG until all dependencies are
  known" is the same reason validation belongs after the parse loop. Treated as
  a completed record, not a standing constraint.
- **Knowledge `architecture/ux-flow.md`** — **conflicts with this spec.** It
  documents `config.Validate()` returning a `diff` and lists `Diff` in the
  component map. Since knowledge is binding and outranks code, this entry must be
  updated as part of this work, not left to contradict the new behaviour.
  **Raised with the user, who confirmed it is updated within this plan** — the
  `Validate` example becomes the error-only contract and `Diff` leaves the
  component map, landing alongside the code change.

## Open assumptions

If any of these turns out wrong, the implement workflow must **STOP and ask**.

**RESOLVED during the open-questions step** — assumptions 1, 2 and 6 below were
closed by reading the code; see `open_questions.md`. Retained for the record:

- **#2 (plugin schemas present before validation): RESOLVED — yes.** Plugin hosts
  are appended to `pluginHosts` by `RegisterPlugin`
  (`plugins/registry/plugin_registry.go:117-128`) before `Parse` runs, and the
  type roster is read from those hosts (`:55-56`, `:101-102`, `:219-220`). No
  late registration during the walk.
- **#6 (module outputs need `SubContext`): RESOLVED — no, they do not.**
  `SubContext` is assigned at `callbacks.go:147-152` with **only**
  `"variable": suppliedVars`. It carries module *variables*, never outputs.
  Module outputs are ordinary `output` blocks already in `parsedResources`, so a
  `module.foo.some_output` reference resolves by lookup. The spec-phase concern
  was misplaced.
- **#1 (unverifiable type): LARGELY RESOLVED.** `parser.go:404-414` already
  fails fatally for an unregistered type; the gap is only that the message names
  `b.Type` rather than the type. Phase 1.3 verifies before coding.

Still genuinely open (see `open_questions.md`): whether `Validate` can skip the
walk without a wider change, and whether block-granular positions suffice.

1. **Every resource type reachable at validation time can be instantiated.**
   Stage 3 needs a type per resource. `parser.go:404` already instantiates at
   parse time and hard-fails when the type is unavailable, so the spec's
   "unverifiable type must fail" may already hold — but this was not proven for
   the case where a plugin loads but omits a declared type.
2. **Plugin schemas are present before validation runs.** Plugin types reach us
   only as JSON schema bytes ([plugins/plugin.go:39](plugins/plugin.go#L39)) via
   loaded hosts. If a host can be registered lazily during the walk, stage 3
   would see an incomplete roster. Not verified.
3. **Depth-10 truncation never bites a real path.** Plugin schemas are generated
   at depth 10. Assumed no real configuration references a path deeper than 10
   struct levels; if one does, the checker must accept rather than reject.
4. **The two `errors` package test failures at baseline are unrelated.** They
   look like `.hcl`/`.xcl` fixture drift, but they sit on the error-rendering
   path this work touches. Not root-caused.
5. **No consumer depends on `ContainsWarnings()`.** Only two read sites exist
   in-repo, but the method is exported from a public package. The user has
   confirmed nobody consumes the library yet.
6. **Module `SubContext` is not needed for stage 2/3.** Module outputs are
   parsed as `output` blocks into `parsedResources` at parse time, so a
   `module.foo.some_output` reference should resolve by lookup. The spec-phase
   note that `SubContext` is unpopulated until the walk is assumed irrelevant to
   a *static existence* check. Not proven end-to-end.

## Drafting assumptions

The judgement calls made while drafting this plan — each with its rationale and
the alternatives rejected — recorded for challenge at the walkthrough.

# Judgement calls made during planning

Recorded as they are made, so a resumed session does not re-decide them and the
walkthrough can surface them to the user.

## Discovery step

**D1. Treated the failing baseline as context, not as work.**
`go test ./...` fails before any change (10 tests, 3 non-building packages).
I did not fold "make the suite green" into this plan's scope — most failures are
unrelated fixture drift. Instead the plan carries a *named* exit criterion.
Alternative rejected: expanding scope to fix everything, which would bury the
spec's actual requirements in unrelated repair work.

**D2. Recommending against reusing `GenerateSchemaFromInstance` as-is.**
It returns JSON bytes, truncates silently at a depth cap (false rejections), and
has no cycle guard (137 KB for a self-referential type — measured). Resolving
the path against `reflect.Type` segment by segment avoids all three. This is a
recommendation to put to the user at the architecture step, not a settled call —
it trades reuse of existing code for correctness, and the spec's Technical
Approach explicitly prefers deriving properties "from the type itself".

**D3. Two existing tests assert the spec's opposite; inverting them is in scope.**
`TestParseFileReturnsConfigErrorWhenResourceInterpolationError` (`parse_test.go:943`)
asserts a property typo is a *warning* with `ContainsErrors()==false`. The spec
requires it to fail. Same for the `Len(ce.Errors, 1)` over three broken files.
I treat changing these as part of the work rather than as regressions, because
they encode the defect being removed.

**D4. Did not treat `.hcl`/`.xcl` fixture drift as this work's problem.**
Pre-existing, unrelated to validation, and fixing it would widen scope. Recorded
in [baseline.md](baseline.md) so it is not mistaken for a regression.

**D5. Verified the go-cty fork rather than trusting the schema package.**
The authoritative HCL naming rule lives in a *forked* go-cty
(`go.mod:98`) that reads `hcl` tags where upstream reads `cty`. Had I taken
`internal/schema` as the source of truth, stage 3 would have matched on Go field
names and been wrong for every field. Verified directly in the module cache.

**D6. Left the `Diff` removal exactly as the spec scopes it.**
Confirmed zero test coverage and only one production consumer, so it is a
clean deletion. No attempt to preserve it behind a flag — the spec is explicit
that it is withdrawn, not reworked, and the user confirmed nobody consumes the
library yet.

## Discovery step (research.md)

**D7. RESOLVED BY USER — the knowledge entry is updated as part of this work.**
Asked the user how to handle the conflict; they chose to update
`architecture/ux-flow.md` within this plan. So the plan carries a task to rewrite
its `Validate` example to the error-only contract and drop `Diff` from the
component map, landing with the code change. Original note follows.

**D7a. Flagged, did not resolve unilaterally, the knowledge-base conflict.**
`architecture/ux-flow.md` (always-loaded, binding) documents
`config.Validate()` returning a `diff` and lists `Diff` in the component map.
This spec withdraws `Diff`. Per the project rule that knowledge outranks code
and must not be silently overruled, I did not treat the entry as stale. Recorded
in research.md and raised with the user rather than decided unilaterally.
Rejected: quietly planning around it, which would leave a binding entry
contradicting shipped behaviour.

**D8. Treated `decisions/parser-refactor-analysis.md` as a completed record.**
Dated 2025-12-27, it describes the `parsed` type as "MISSING" though it now
exists at `parser.go:39`, and its checklist is done. Read as historical
rationale (which supports this work) rather than as outstanding instructions.
Rejected: treating its "Future" checklist as binding scope, which would pull
module-support work into this plan.

**D9. Recorded six open assumptions rather than resolving all of them now.**
Chiefly whether plugin schemas are fully loaded before validation runs, and
whether module outputs resolve statically without `SubContext`. Each is cheap to
check during implementation and expensive to prove exhaustively now; all are
marked STOP-and-ask. Rejected: blocking discovery to prove each end-to-end.

## Architecture step

**A1. CHOSEN DIRECTION — a three-stage exhaustive pass inside `internal/parser`,
gated between the parse loop and the walk.**
- **Decision**: insert validation at `parser.go:258`, running structure →
  references → properties, each accumulating every problem into the existing
  `errors.ConfigError`. `Config.Validate` returns `error` alone; `Diff` deleted.
- **Rationale**: both public entry points already funnel through `Parse`, so one
  gate satisfies both spec constraints without duplication. The error collection
  and per-problem location plumbing already exist.
- **Rejected**: (i) fixing the classifier in place — cannot hard-fail and tolerate
  on one shared path, and leaves "nothing is acted upon unless valid" unmet;
  (ii) keeping `Diff` via a walk inside `Validate` — requires resolution, the very
  thing that makes validation non-distinct; (iii) per-file validation — broken by
  cross-file references.

**A2. Stage 3 walks `reflect.Type` segment-by-segment rather than a generated
schema tree.**
- **Decision**: resolve one path segment at a time against the type reached so far.
- **Rationale**: measured three defects in reusing `GenerateSchemaFromInstance` —
  JSON-bytes return, silent depth truncation (would cause **false rejections**),
  no cycle guard (137 KB for a self-referential type). Segment-at-a-time needs no
  depth cap and is cycle-safe because path length bounds the walk.
- **Rejected**: walking the `Attribute` tree as the primary mechanism. Retained
  where it is the only source — plugin types exist only as JSON schema.

**A3. Validation lives in `internal/parser`, not a new package.**
- **Decision**: new files inside the existing package.
- **Rationale**: it operates on `parsed`, which is unexported (`parser.go:39`).
  A separate package would force exporting the working structure purely for
  layout reasons.
- **Rejected**: `internal/validator`. Rejected on the internal-surface cost, not
  on preference; the `/internal` convention is satisfied either way.

**A4. Single tag-parsing helper, written once.**
- **Decision**: one place implements `strings.Split(tag.Get("hcl"), ",")[0]` plus
  anonymous-embed flattening, handling both `hcl:",remain"` and `hcl:"rm,remain"`.
- **Rationale**: the authoritative rule lives in a forked dependency
  (`go.mod:98`), not this repo. Two copies would reintroduce exactly the
  disagreement about what a configuration means that this work removes.
- **Rejected**: inlining the split at each use site.

**A5. Added the module-recursion cycle guard to this work's scope.**
- **Decision**: add a visited-set guard to `parseModule` recursion, reported
  against the module per the unobtainable-module constraint.
- **Rationale**: no guard exists today (`parser.go:674-689`); a self-including
  source recurses to stack exhaustion. This work makes module contents
  load-bearing for validation, so the crash becomes reachable through a new path.
- **Rejected**: leaving it as a pre-existing bug. A stack overflow is not a
  reportable problem, and the spec requires module failures to be reported
  against the module.

**A6. Selected 11 conventions; dropped the database and service-lifecycle ones.**
- **Decision**: see `conventions.md`. Kept the Go-style, testing and dependency
  conventions; explicitly dropped prepared statements/connection pooling and the
  handler/context/shutdown items.
- **Rationale**: this is a parsing library with no database, no request path and
  no service lifecycle. Listing them would pad the list and signal the knowledge
  base was not actually consulted.
- **Rejected**: listing every convention for completeness.

## Components step

**C1. Four new components, not one monolithic validator.**
- **Decision**: separate the validator (stage ordering + accumulation) from the
  reference resolver, the property path checker, and the type name resolver.
- **Rationale**: the "small, focused interfaces" convention, and each answers a
  genuinely different question. The type name resolver in particular must be one
  component because its rule lives in a forked dependency — a second copy would
  reintroduce the disagreement this work removes.
- **Rejected**: a single validation function. It would bury the two subtle
  stopping rules inside stage 3 and make the tag-parsing rule easy to duplicate.

**C2. Reference discovery is reused, not made a component.**
- **Decision**: the existing expression traversal that feeds the dependency graph
  stays as-is and is called by stage 2.
- **Rationale**: the spec's Technical Approach explicitly prefers this; two
  extractors could disagree about what a configuration refers to.
- **Rejected**: wrapping it in a new component, which would imply it is being
  changed. Its known blind spots are pre-existing and shared with the DAG.

## Data structures step

**DS1. Removing `ContainsErrors()` and `ContainsWarnings()`, not just `Level`.**
- **Decision**: delete both methods from the public `ConfigError`; callers test
  whether `Errors` is empty.
- **Rationale**: both exist only to interrogate severity. With severity gone
  `isParserError` has nothing to branch on and `ContainsErrors()` would always
  return true for a non-empty collection — a method that always agrees with
  `len(Errors) > 0` is a trap, not an API. The spec's "removing it rather than
  fixing it in place is preferred, so that severity does not quietly continue to
  stand in for which stage a problem was found at" covers this.
- **Rejected**: keeping `ContainsErrors()` as a `len(Errors) > 0` alias for
  source compatibility. It would preserve the illusion that some collected
  problems are advisory. Note this goes one step beyond what the spec names
  explicitly; it is a public API break, acceptable because the user confirmed
  there are no consumers. **Flag at walkthrough.**

**DS2. `resolveReference` returns the leftover attribute path, not just a bool.**
- **Decision**: one call answers both "does it exist" (stage 2) and "what is left
  to check" (stage 3).
- **Rationale**: the split between the resource part and the attribute part of a
  reference is exactly where the existing code goes wrong today — keys never
  carry an attribute, references can. Computing that boundary in two places is
  how the current mismatch arose.
- **Rejected**: a bool-only resolver with stage 3 re-splitting the reference.

**DS3. `checkPropertyPath` returns at most one problem per path.**
- **Decision**: report the first non-existent segment and stop walking that path.
- **Rationale**: once a segment does not exist, the type beyond it is unknown, so
  later segments cannot be judged — reporting them would be noise. Exhaustiveness
  is across *references*, not within a single path, which still satisfies "every
  problem found within a stage".
- **Rejected**: attempting to continue after an unknown segment.

## Implementation detail step

**ID1. Kept this section free of file:line references.**
- **Decision**: described patterns and shapes only; every concrete site lives in
  context.md.
- **Rationale**: the step's own test — could it be written before the phases are
  defined? All of it could.
- **Rejected**: repeating the discovery citations here, which would duplicate
  context.md and rot when line numbers move.

## Dependencies step

**DE1. No new third-party dependency.**
- **Decision**: stage 3 uses `reflect` + `strings` from the standard library.
- **Rationale**: the conventions prefer stdlib and require justifying any
  third-party addition. Nothing here warrants one.
- **Rejected**: a struct-tag parsing library — the rule is four lines and must
  match the forked go-cty exactly, which a general-purpose library would not.

**DE2. Recorded the forked go-cty as the most important dependency.**
- **Decision**: called it out explicitly as authoritative and easiest to get wrong.
- **Rationale**: it is a `replace` directive, invisible in the import list, and
  reads a different struct tag than upstream. An implementer trusting the
  upstream library's documented behaviour would produce a checker that disagrees
  with the runtime on every field.
- **Rejected**: listing it as a routine pinned dependency.

## Testing approach step

**T1. All 21 acceptance criteria classified as behavioural tests; none manual.**
- **Decision**: no criterion flagged "Manual — captured in the implementation
  test plan".
- **Rationale**: every criterion describes an outcome of validating or applying a
  configuration, which a test can set up and assert directly. No infrastructure,
  load or production telemetry is involved.
- **Rejected**: flagging "still produces the same resources as previously" as
  manual. The in-process test plugin already records created/updated/destroyed
  resources, so it is directly assertable.

**T2. Error tests assert on problem *content*, not just failure or counts.**
- **Decision**: require that the reported problem names the offending item.
- **Rationale**: almost any defect makes a configuration fail, so asserting only
  "an error occurred" passes for the wrong reason. The spec requires each problem
  to say what and where, so content assertions test the requirement itself.
- **Rejected**: count-only assertions, which is the pattern in the existing tests
  and is precisely what let the current defect hide.

**T3. Existing exact-count assertions get decided numbers, not observed ones.**
- **Decision**: each changed count is justified by what the stage should find.
- **Rationale**: updating a count to match whatever the new code emits asserts
  nothing and would mask an over- or under-reporting bug.
- **Rejected**: running the suite and recording the output.

**T4. Second success metric recorded as not-yet-testable rather than dropped.**
- **Decision**: noted the "confusing error reports" signal as forward-looking.
- **Rationale**: the spec states it explicitly; silently omitting it would look
  like a missed metric.
- **Rejected**: inventing a proxy assertion for it.

## Milestones step

**M1. Four milestones, one per stage plus a cleanup.**
- **Decision**: M1 structure+gate, M2 references, M3 properties, M4 severity
  removal and docs.
- **Rationale**: mirrors the spec's own three-stage constraint, and each stage
  depends on the previous one having succeeded, so the build order is forced.
  Each milestone is independently deliverable and leaves the library working.
- **Rejected**: (i) one milestone per requirement — too granular, 14 milestones;
  (ii) a single "add validation" milestone — no intermediate validation points,
  and the riskiest part (properties) would land unverified with everything else.

**M2. The gate ships in M1, before the checks that justify it.**
- **Decision**: "nothing is acted upon unless valid" is delivered first, with only
  structural checks behind it.
- **Rationale**: it is the spec's central guarantee and the one that makes the
  rest meaningful. Building the checks first and gating last would mean every
  intermediate milestone still permits acting on invalid configuration.
- **Rejected**: gating last, once all three stages exist.

**M3. Diff withdrawal lands in M1, not the cleanup milestone.**
- **Decision**: remove `Diff` when the validation entry point changes shape.
- **Rejected**: deferring to M4. The entry point cannot both stop before the walk
  and return a comparison that requires walking — they are the same change, and
  splitting them would leave M1 unable to satisfy "validation acts on nothing".

**M4. Severity removal deferred to M4 rather than done in M1.**
- **Decision**: leave the classification in place until all three stages exist.
- **Rationale**: it is only safe to delete once checking is exhaustive enough that
  nothing needs downgrading. Removing it in M1 would hard-fail unresolved
  interpolation before the property stage exists to distinguish a real typo from
  a value not yet known — regressing the tolerance the user explicitly wants kept.
- **Rejected**: removing it in M1 alongside the parse-path fix. Tempting because
  the motivating test is about severity, but M1 only needs the *parse path* to
  stop defaulting to warning — not the whole system removed.

**M5. Knowledge-entry and doc updates placed in M4.**
- **Decision**: update `architecture/ux-flow.md` and the docs in the final milestone.
- **Rationale**: documentation should describe the finished behaviour; updating it
  in M1 would describe a half-built state.
- **Rejected**: M1, alongside the Diff removal that makes the entry wrong.
  Accepting a brief window where the entry is stale, closed before the work ends.

## Phases step

**P1. Twelve phases across four milestones (3+3+3+3).**
- **Decision**: each milestone splits into roughly three phases.
- **Rationale**: each phase is independently verifiable and sized for one agent
  context (~12-45k). The riskiest work (property checking) gets its own phase
  rather than riding along with the stopping rules.
- **Rejected**: one phase per milestone — Phase 3 alone would be ~100k and
  unreviewable.

**P2. Flagged the "Validate must not walk" problem rather than assuming the gate
solves it.**
- **Decision**: Phase 1.2 records that `Parse` **always** calls `p.walk`
  (`parser.go:269-271`, comment: "Always walk the DAG to decode resources"),
  regardless of `executePlugins`. So inserting a gate is *not* by itself enough
  for "validation acts on nothing" — `Validate` still decodes today, which is
  resolution work the spec forbids.
- **Rationale**: verified in source. Had the plan asserted the gate alone
  satisfied the constraint, the implementer would have shipped a `Validate` that
  still resolves.
- **Rejected**: silently assuming `executePlugins=false` means "does nothing" —
  it only skips providers (`callbacks.go:156`), not decoding.
- **Left to implementation**: whether to add a `Parse` variant or a parameter.
  Both are reasonable; the constraint is recorded, the mechanism is not
  over-specified.

**P3. Phase 1.3 says "verify first" on the unverifiable-type requirement.**
- **Decision**: instruct the implementer to write the failing test before any
  code, because `parser.go:404-414` may already satisfy the requirement.
- **Rationale**: it already produces a fatal error for an unregistered type. The
  genuine gap is that the message names `b.Type` (literally `"resource"`) rather
  than the unknown type, failing "each problem says what".
- **Rejected**: specifying an implementation for something possibly already done.

**P4. Reference-error position accuracy is block-granular, and said so.**
- **Decision**: positions come from the referring resource's `Meta`
  (`parser.go:502-505`), which points at the resource block, not the exact
  expression.
- **Rationale**: finer positions require capturing `Expression.Range()` during
  `processExpr` — a materially larger change touching the shared DAG path.
- **Rejected**: (i) claiming exact positions the design does not deliver;
  (ii) expanding scope to capture expression ranges. Recorded so the limit is
  visible rather than discovered.

**P5. Left cycle-detection consolidation as conditional scope.**
- **Decision**: Phase 2.2 says bring cycles into stage 2 "only if cheap",
  otherwise record as a known gap.
- **Rationale**: today real cycle detection is `d.Validate()` (`parser.go:894`)
  reporting with **no file/line**, which arguably violates "each problem says
  what and where" — but cycles are not named in any requirement or criterion.
  Mandating it would expand scope beyond the spec.
- **Rejected**: (i) mandating it; (ii) ignoring it entirely.

**P6. Phase 4.2 routes the knowledge edit through `spektacular knowledge write`
with a propose-and-confirm step.**
- **Decision**: not the `Write` tool, and not without asking.
- **Rationale**: the knowledge workflow requires proposing content and waiting
  for explicit confirmation. The user approved *that the entry is updated*, not
  its wording.
- **Rejected**: editing the file directly, bypassing the knowledge workflow.

**P7. Phase 4.3 exists purely to verify, and carries the baseline inline.**
- **Decision**: a dedicated closing phase restating the pre-change failures.
- **Rationale**: 10 tests fail and 3 packages do not build before any work
  starts. Without the baseline in the plan, an implementer cannot tell a
  regression from pre-existing noise, and "all tests pass" is unreachable.
- **Rejected**: folding verification into 4.1, which would bury it.

## Open questions step

**OQ1. Closed four of six open assumptions by reading code rather than parking them.**
- **Decision**: resolved #1, #2, #6 and the severity-rendering question; kept only
  two genuinely implementation-time items.
- **Rationale**: the step's own scope rule — anything answerable by reading code
  must be answered now. Each took one grep.
- **Findings**: `SubContext` (`callbacks.go:147-152`) carries only
  `"variable"`, so module outputs resolve statically and the spec-phase worry was
  misplaced; plugin hosts register before `Parse`, so the type roster is complete;
  the unverifiable-type case largely already works.
- **Rejected**: carrying all six forward, which would have made the section look
  thorough while hiding that most were cheap lookups.

**OQ2. Kept the "Validate must not walk" item as genuinely open.**
- **Decision**: it stays, with STOP-and-ask guidance.
- **Rationale**: whether anything depends on validation having decoded cannot be
  known from the code — there are **no tests at that layer** to reveal it. If
  something does, the spec's constraint conflicts with existing behaviour, and
  that is the user's call.
- **Rejected**: assuming nothing depends on it. The plan would then silently
  ship a behaviour change nobody decided on.

## Out of scope step

**OS1. Pre-existing test failures excluded explicitly rather than silently.**
- **Decision**: named them as out of scope, with the reason, and pointed at the
  baseline in the plan's technical notes.
- **Rationale**: eight of the ten failures pre-date this work. Leaving the
  exclusion implicit would let an implementer either chase them (scope creep) or
  mistake a real regression for pre-existing noise.
- **Rejected**: (i) fixing them — unrelated scope; (ii) not mentioning them.

**OS2. Recorded the reference-collection blind spots as a known gap.**
- **Decision**: reuse the existing traversal unchanged, including the expression
  forms it does not see.
- **Rationale**: it feeds the dependency graph too, so widening it changes what
  the graph acts on — a riskier change than validation itself, and not asked for.
- **Rejected**: fixing the blind spots while in the area. Tempting but would mean
  validation and the DAG start disagreeing mid-change.

**OS3. Left the plugin schema depth limit alone.**
- **Decision**: treat a truncated description as unknowable and accept.
- **Rationale**: accepting at an unknowable boundary is the safe direction — a
  false rejection breaks working configuration, while a missed typo is only the
  status quo. Raising the limit changes the plugin boundary for everyone.
- **Rejected**: raising the depth, or rejecting at truncation.

## Rehydration cues

To rebuild this context cold:

1. `spektacular repo list` — confirm the root; all code lives in `xclconfig`.
2. Read the spec: `spektacular spec file read 20260918075104-validation-phase-before-walk.md`.
3. Read `.spektacular/working-context.md` **in full** — it holds the spec-phase
   investigation (HCL diagnostic audit, probe tables, the splat correction, the
   decision to bin `Diff`) that is expensive to rediscover.
4. Read the sibling working files: `baseline.md` (what already fails),
   `schema_findings.md` (why not to reuse the serializer),
   `attribute_rules.md` (the forked-go-cty naming rules),
   `test_impact.md` (which tests assert the defect), `assumptions.md`.
5. Re-read [parser.go:182-320](internal/parser/parser.go#L182) — `Parse` and
   `parseResourcesInFile` — the insertion point and the stage-1 target.
6. Re-read [parser.go:697-820](internal/parser/parser.go#L697) — link extraction
   and the silent-miss cycle check, which is stage 2's foundation.
7. `spektacular knowledge always-applied --tier repo --filter xclconfig` — the
   Go conventions (testify `require`, Mockery, **never** table-driven, never mix
   positive and negative tests).
8. Baseline check before starting: `go test ./... 2>&1 | grep -E '^(---|ok|FAIL)'`
   — 10 tests fail and 3 packages do not build **before** any change.
9. To re-probe the schema: write a throwaway test in `internal/schema/` calling
   `GenerateSchemaFromInstance(structs.ContainerBase{}, N)` and print the JSON;
   vary `N` to see truncation. Delete it afterwards.
