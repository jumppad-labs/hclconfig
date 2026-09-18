---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Plan: 20260918075104-validation-phase-before-walk

<!-- Metadata -->
<!-- Created: 2026-09-18T11:29:14+01:00 -->
<!-- Commit: 858013a4640c152f81600acc74ce754de3136961 -->
<!-- Branch: v2 -->
<!-- Repository: jumppad-labs/xclconfig -->

## Overview

This introduces a distinct checking stage that runs to completion before any
configuration is acted upon. Configuration is currently checked while it is
being applied, which means genuine mistakes — a malformed block, or a reference
to a resource or property that does not exist — are reported as advisory
warnings, and a configuration containing them can be treated as usable. After
this change, broken configuration is rejected outright, every problem found at a
given stage is reported together, and each one identifies what is wrong and
where.

Anyone authoring configuration benefits: they see the full set of mistakes at
once, and get errors naming the actual problem instead of a confusing downstream
symptom. Anyone applying configuration benefits from a validity check that is
trustworthy on its own — nothing is created, changed or removed unless the
configuration is valid. The dry-run change comparison that the validation entry
point previously returned is withdrawn as part of this, because producing it
requires resolving the configuration, which is precisely what stopped validation
being a distinct stage.

## Conventions

- **Testify `require` for unit tests** — every new validation test asserts and stops on first failure; the existing `internal/parser/parse_test.go` uses `require` exclusively (no `assert`), and the new tests sit alongside it.
- **NEVER use table-driven tests** — this work adds a large number of validation cases (each reference shape, each stopping rule), which is exactly the situation that tempts a case table. The existing 49 tests in `parse_test.go` contain no table; new ones must match, one behaviour per function.
- **NEVER mix positive and negative tests in the same function** — load-bearing here, because several cases come in pairs that differ only in outcome: `volume.*.destination` must pass where `volume.*.destnation` must fail. Each gets its own function, following the existing `Test...ReturnsConfigErrorWhen<Cause>` naming for the negative half.
- **Favour verbosity over abstraction in tests** — the stopping rules (member selection vs. type exhaustion) are subtle enough that a reader must see the exact reference and the exact expected outcome inline, not assembled by a helper.
- **Use Mockery for mocking interfaces** — the state store is already a generated mock (`state/mocks`), and `setupParser` depends on it; new tests reuse that rather than hand-rolling. Note the existing hand-written `TestPlugin` (`internal/parser/test_plugin.go`) stays as-is for resource-type registration — that split is the established practice in this package.
- **Always handle errors explicitly** — the defect being fixed is partly an unhandled case: the dependency lookup at `parser.go:788` has no `else` branch on a miss, and `getUniqueResourceLinks` discards an error at `parser.go:704`. Both are places this convention was not followed, and stage 2 is where that gets corrected.
- **Prefer small, focused interfaces** — the three stages are separable checks over the same parsed data; keeping each behind a narrow seam is what lets a stage be skipped when an earlier one has already failed.
- **Use `any` rather than `interface{}`** — validation touches `parsedResources.resources` (`map[string]any`) and reflective type walking, where the empty type appears constantly in new signatures.
- **Prefer the standard library** — stage 3's type walking is `reflect` plus `strings` for tag splitting. No new dependency is warranted, and the dependencies convention requires justifying any third-party addition.
- **`/internal` for private application code** — validation stays in `internal/parser`, consistent with this convention; it operates on the unexported `parsed` type, so placing it elsewhere would mean exporting internals purely for layout.
- **Follow standard Go conventions (gofmt, go vet)** — note `go vet` already reports pre-existing findings in `plugins/` and `example/`; new code must not add to them, but clearing the existing ones is out of scope.

Deliberately **not** applied: the database/external-services conventions (prepared statements, connection pooling) — this work touches no database or external service; and "keep handlers thin, business logic in services" / "context.Context for request-scoped values" / "graceful shutdown" — there is no request path or service lifecycle here, as this is a parsing library.

## Architecture & Design Decisions

All work lands in the single registered repo `xclconfig`
(`spektacular repo list` → root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).

### The shape

Validation becomes a distinct, exhaustive pass that runs to completion between
parsing and walking, inside the existing `internal/parser` package. `Parse`
gains one call at [parser.go:258](internal/parser/parser.go#L258) — after the
parse gate, before resources move into state and before `p.walk` at
[parser.go:272](internal/parser/parser.go#L272). Because `Config.Validate`
([config.go:80](config.go#L80)) and `Config.Apply`
([config.go:107](config.go#L107)) both funnel through `Parse`, differing only by
the `executePlugins` flag, a single gate there satisfies both halves of the
spec's "available on its own, and automatically before applying" constraint
without duplicating logic. `Config.Validate` changes to return `error` alone and
stops before the walk; `Diff` and `buildDiff` are deleted.

The pass runs three stages in order — **structure**, then **references**, then
**properties** — each gathering every problem it finds into the existing
`errors.ConfigError` collection before deciding whether the next stage has
anything meaningful to say. That collection type already fits: `ConfigError`
holds `Errors []error` with `AppendError`
([errors/config_error.go:11-23](errors/config_error.go#L11)), and `ParserError`
already carries `Filename`, `Line` and `Column`
([errors/parser_error.go:17-24](errors/parser_error.go#L17)) — exactly the
per-problem "what and where" the spec requires for a future editor interface. No
new error plumbing is introduced; what changes is that validation *accumulates*
where the current code returns at its first fault.

Stages 2 and 3 are built on machinery that already exists rather than new
traversals. Stage 2 resolves each `Meta.Links` entry against
`p.parsedResources.resources`, which is exactly the lookup the cycle check
performs today at [parser.go:788](internal/parser/parser.go#L788) — except that
today a miss falls through silently with no `else` branch. That silent miss *is*
the spec's "reference to something defined nowhere", so stage 2 is largely a
matter of giving an existing lookup a failure branch, applying
`FQRN.StringWithoutAttribute()` ([fqrn.go:205](internal/resources/fqrn.go#L205))
**before** the lookup to reconcile the long-standing mismatch between reference
strings (which carry attributes) and map keys (which never do,
[fqrn.go:167-177](internal/resources/fqrn.go#L167)). Links come from
`processExpr` ([exp.go:22](internal/parser/exp.go#L22)), reused rather than
reimplemented so validation checks precisely what the DAG will act on — the
spec's Technical Approach rules out a second extractor, and two extractors could
disagree about what a configuration refers to.

### Key design decisions

**Stage 3 resolves paths against `reflect.Type`, one segment at a time, rather
than walking a generated schema tree.** This is the load-bearing decision and
the one that most affects correctness. Reusing `GenerateSchemaFromInstance`
([internal/schema/serialize.go:13](internal/schema/serialize.go#L13)) was the
obvious move, and it was measured and rejected: it returns JSON bytes rather
than the tree, it truncates silently at a depth cap (depth 1 yields 8 of 86
nodes for `ContainerBase`, and the cap returns `nil` with no marker, so a
checker cannot tell "absent" from "not looked at" and would **reject valid
configuration**), and it has no cycle guard — a self-referential struct produced
137 KB at depth 100. Segment-at-a-time resolution needs no depth cap, is
naturally cycle-safe because the path length bounds the walk, and only resolves
as deep as the reference actually goes. Where a plugin type has no real Go type
— plugin types cross the process boundary as JSON schema and are rebuilt as
anonymous `reflect.StructOf` structs
([internal/schema/deserialize.go:130](internal/schema/deserialize.go#L130)) —
the same walk still applies, because the rebuilt struct carries the original
`hcl` tags verbatim. The depth-10 truncation on that pre-generated schema
([plugins/plugin.go:107](plugins/plugin.go#L107)) is handled by treating a
truncation boundary as *unknowable* and accepting the remainder, never
rejecting.

**A member-selecting segment changes what the rest of the path is checked
against; it does not stop the check.** Selecting a map key, a collection index,
or every member at once tells us *which* member is unknown, never *what type* it
is. So the walk steps through to the element type and keeps checking — which is
what catches `volume.*.destnation` while accepting `volume.*.destination`. The
check stops only where the **type** genuinely runs out: a scalar-valued map such
as `Env map[string]string`, a `cty.Value` field (which becomes
`cty.DynamicPseudoType`), or a truncated schema. This two-rule split is the
distinction the spec spent three review rounds arriving at, and collapsing it
back into a single "stop at the first dynamic segment" rule would silently
accept typos after a splat.

**Property names come from a single tag-parsing helper, written once.** The
authoritative rule is not in this repo: [go.mod:98](go.mod#L98) pins a **fork**
of go-cty that reads `hcl` struct tags where upstream reads `cty`, giving
`strings.Split(tag.Get("hcl"), ",")[0]`, with an empty first segment meaning "no
name of its own" and anonymous embeds flattening recursively. No first-party
file parses HCL tags today, so stage 3 introduces this — and it must exist in
exactly one place, because a second divergent copy would reintroduce precisely
the disagreement about what a configuration means that this work exists to
remove. It must handle both embed spellings found in the tree: the nine
`hcl:",remain"` embeds and the lone `hcl:"rm,remain"` at
[structs/container.go:14](internal/test_fixtures/plugin/structs/container.go#L14),
which registers a real attribute named `rm` *and* flattens. Fields with no `hcl`
tag — `Meta.Properties`, `Meta.Links`, `Meta.Status`
([types/resource.go:36](types/resource.go#L36)) — are invisible to HCL and must
be reported as not existing, even though they are real Go fields.

**Severity classification is deleted rather than corrected.** With validation
exhaustive, the level has nothing left to decide. It is removed from the
`NewParserError` / `NewParserErrorFromResource` signatures, taking with it both
copies of the classifier — the inlined block at
[parser.go:290-302](internal/parser/parser.go#L290) and
`checkIfErrorInFunction` at [parser.go:923](internal/parser/parser.go#L923) —
along with the dead `"Error in function call"` comparison, which cannot fire on
the parse path because `ParseXCLFile` never evaluates. The change is narrower
than its 57 references suggest: only **two** read sites exist
([config_error.go:28](errors/config_error.go#L28) and
[config_error.go:49](errors/config_error.go#L49)), and `ParserError.Error()`
never reads the level ([parser_error.go:27-74](errors/parser_error.go#L27)), so
no rendered output changes. Leaving the level in place would let severity
quietly continue standing in for which stage a problem was found at, which is
the confusion the spec removes.

**Validation lives inside `internal/parser`, not a new package.** It operates on
`p.parsedResources`, whose type `parsed` is unexported
([parser.go:39](internal/parser/parser.go#L39)). A separate package would force
that working structure to be exported purely to be validated — widening the
internal surface to satisfy a file-layout preference. New files within the
package keep the concern separable without that cost, and the project-structure
convention (`/internal` for private application code) is already satisfied.

### Why this beats the alternatives

Fixing the classifier in place was the cheapest option and cannot work: the
level is doing double duty as a phase marker *because* validation and resolution
share one path, so that path cannot hard-fail on unresolved interpolation while
also tolerating it. It would also leave the spec's central guarantee unmet —
resources would still be decoded and providers still reached before any verdict
existed. Keeping `Diff` by walking inside `Validate` fails for the same reason:
producing a diff requires resolution, which is exactly what makes validation
non-distinct. Validating files independently is ruled out by cross-file
references, and is unnecessary anyway — the parse loop at
[parser.go:249-254](internal/parser/parser.go#L249) already puts every file's
resources in one place before anything else runs. Full evidence and citations
for each rejected option are in
`research.md#alternatives-considered-and-rejected`.

Two obstacles that surfaced during discovery shape the phasing rather than the
design. Stage 1 cannot simply flip a level: `parseResourcesInFile` returns a
one-element slice on every error path and reports only `diag[0]`
([parser.go:305-315](internal/parser/parser.go#L305)) while a single malformed
input can produce three diagnostics — so meeting "report every problem" requires
restructuring that function, not re-classifying it. And module recursion has no
visited-set guard ([parser.go:674-689](internal/parser/parser.go#L674)), so a
self-including module source recurses to stack exhaustion; since this work makes
module contents load-bearing for validation, that guard is added here, reported
against the module itself as the spec's unobtainable-module constraint requires.

## Component Breakdown

### New components

- **Validator** — owns the decision "is this configuration valid?" for one whole
  configuration. It runs the three stages in order, decides after each whether
  the next has anything meaningful to say, and returns every problem it found
  rather than the first. It reads the parser's working set of parsed resources
  and their links; it never resolves values, never decodes bodies, and never
  reaches a provider. It is the only component that knows the stage ordering, so
  the parser asks it one question and gets one verdict.

- **Reference resolver** — owns the question "does the thing this reference names
  exist?" Given a reference string, it reconciles the reference's shape against
  the parsed resource keys and reports the ones that resolve nowhere. This is the
  component that closes the long-standing mismatch where reference strings carry
  a trailing attribute path and resource keys never do; it strips the attribute
  before looking up, rather than after. It serves stage 2, and hands stage 3 the
  resolved target plus the leftover attribute path.

- **Property path checker** — owns the question "does this dotted path name real
  properties on this type?" It walks the path one segment at a time against the
  type reached so far, and it owns the two stopping rules that the spec spent
  three review rounds arriving at: a segment that *selects a member* of a
  collection changes what the remainder is checked against and the walk
  continues; the walk stops and accepts the remainder only where the *type*
  itself stops being knowable. It serves stage 3 and depends on the type-name
  resolver below for every lookup.

- **Type name resolver** — owns the single authoritative answer to "what property
  names does this type expose?" It is deliberately one component because the rule
  lives in a forked dependency rather than in this codebase, and a second copy
  would reintroduce exactly the disagreement about what a configuration means
  that this work removes. It handles both embedded-field spellings present in the
  tree, and treats a field with no HCL tag as not existing even though it is a
  real Go field. It is used only by the property path checker.

### Meaningfully changed existing components

- **Parser** — gains one call to the validator between finishing the parse loop
  and starting the walk, and stops on an invalid verdict. It loses its severity
  classification entirely: both copies of the classifier go, along with the dead
  comparison that could never fire on the parse path. Its per-file parsing
  changes from returning at the first problem in a file to reporting every
  problem that file has, which is what the "report them all" requirement needs
  and which re-classification alone would not deliver.

- **Module parsing** — gains a guard against recursing into a module source that
  transitively includes itself. Today there is no such guard and this work makes
  module contents load-bearing for validation, so the failure becomes reachable;
  the guard reports against the module, matching how the spec requires every
  unobtainable-module failure to be attributed.

- **Walk** — loses its error classification. With validation exhaustive, it has
  nothing left to downgrade, so it reports what it finds without deciding how
  serious it is. Its traversal and resolution behaviour are otherwise unchanged;
  this work does not add checks during application.

- **Config (public entry points)** — `Validate` changes shape: it answers only
  whether the configuration is valid, and stops before the walk rather than
  walking to produce a comparison. `Apply` is unchanged in signature but now
  cannot reach the walk with invalid configuration.

- **Parser error** — loses its severity field and the level argument from its
  constructors. It keeps the file, line and column it already carries, which is
  what makes each problem individually locatable for the interface the spec
  names as the intended consumer.

- **Config error** — keeps its role as the collection of problems, which already
  matches what this work needs. It loses the two methods that exist only to ask
  about severity.

### Removed

- **Diff** — withdrawn with its builder. It had one production consumer and no
  test coverage. Producing it requires resolving the configuration, which is
  precisely what validation must not do.

### How they fit together

The parser owns the pipeline and calls the validator once. The validator owns
stage ordering and error accumulation, delegating the two hard questions: the
reference resolver answers "does this exist", and the property path checker
answers "does this type have that", itself leaning on the type name resolver for
every name it needs. Nothing below the validator decides whether the
configuration is acceptable — each reports what it found — and nothing above it
proceeds until it has answered. Reference discovery is **not** a new component:
the existing expression traversal that already feeds the dependency graph is
reused, so validation checks exactly the same references the graph will act on.

## Data Structures & Interfaces

Most of what this work needs already exists. `ConfigError` is already a
collection of problems and `ParserError` already carries the file, line and
column that make a problem individually locatable — so no new error type is
introduced. What changes is that two fields and two methods are *removed*, and
three small new contracts are added for the checks themselves.

### Changed: the error contract

`ParserError` loses its severity. The field and the constructor argument both
go, which is what stops severity quietly standing in for which stage a problem
was found at.

```
ParserError
    Filename, Line, Column   // kept — this is the "where" the spec requires
    Message                  // kept — this is the "what"
    Level                    // REMOVED

NewParserError(filename, line, column, message)          // level argument removed
NewParserErrorFromResource(resource, message)            // level argument removed
```

`ConfigError` keeps its shape and its role as the accumulator — this is already
the "report every problem together" contract the spec asks for — but loses the
two methods that exist only to ask about severity.

```
ConfigError
    Errors []error           // kept
    AppendError(err)         // kept
    Error() string           // kept
    ContainsErrors() bool    // REMOVED — every collected problem is now a failure
    ContainsWarnings() bool  // REMOVED — nothing is advisory any more
```

Removing `ContainsErrors` is a deliberate contract change rather than a
simplification: with validation exhaustive, a non-empty collection *is* the
failure, so asking "but are any of them real?" no longer means anything. Callers
test whether the collection is empty.

### Changed: the public validation contract

```
Config.Validate(paths ...string) error     // was (*Diff, error)
```

`Validate` answers only whether the configuration is valid. `Diff` and its
builder are deleted; nothing replaces them.

### New: the validator contract

One entry point, returning every problem found rather than the first.

```
validate(parsed resources, plugin registry) []error
```

It takes the parser's already-populated working set — every resource from every
file, each with its extracted links — and returns the accumulated problems.
An empty result means valid. It performs no resolution and produces no new
state, which is what keeps validation a distinct stage.

### New: the reference resolution contract

```
resolveReference(reference string, parsed resources)
    -> (target, remaining attribute path, found bool)
```

Given a reference, it answers whether the named thing exists and, when it does,
hands back both the target and the attribute path left over. That split is the
contract's whole point: stage 2 needs the boolean, stage 3 needs the leftover
path, and computing them in one place is what keeps the two stages agreeing
about where a reference stops naming a resource and starts naming properties
on it.

### New: the property path contract

```
checkPropertyPath(targetType, attributePath) -> problem or nil
```

It answers whether a dotted path names real properties, returning at most one
problem per path — the first segment that genuinely does not exist. It embodies
the two stopping rules:

- a segment that **selects a member** of a collection (a key, an index, or every
  member at once) changes what the remainder is checked against, and checking
  continues against the member's type;
- checking **stops and accepts** the remainder only where the type itself stops
  being knowable — a scalar-valued collection, a dynamically-typed value, or a
  schema that was truncated when it was generated.

The distinction between "unknown member" and "unknown type" is the contract
here; collapsing it into a single "stop at anything dynamic" rule would silently
accept a typo written after a splat.

### New: the type name contract

```
propertyNames(type) -> map of HCL name -> type
```

The single authoritative answer to what a type exposes. It follows the rule
used by the forked dependency that governs lookup at runtime — the HCL name is
the part of the tag before the first comma; an empty name means the field
contributes no name of its own; embedded fields flatten their names into the
parent, recursively. A field carrying no HCL tag is **absent** from the result
even though it is a real Go field, so referencing it is correctly reported as
naming something that does not exist.

### Not changed

The parsed working set (`resources` and `bodies` keyed by fully-qualified name),
`State`, `FQRN`, and the plugin registry contracts are all untouched.
Validation reads the working set and reports; it adds nothing to it.

## Implementation Detail

### The new pattern: a gate, not a filter

The central shape change is that parsing acquires a **gate**. Today the pipeline
is one continuous pass — parse, then walk, deciding as it goes how seriously to
take each problem it meets. After this change it is two passes with a verdict
between them: everything that can be judged statically is judged, exhaustively,
and only a configuration that survives reaches the walk.

This is what lets a whole category of judgement disappear. The severity level
exists today because one code path serves two purposes, and a path that must
tolerate unresolved interpolation cannot also hard-fail on it. Splitting the
purposes means neither half needs to hedge: validation says yes or no, and the
walk resolves without classifying. A developer reading the changed code should
find that the question "is this a warning or an error?" simply has nowhere left
to be asked.

### Accumulate, then decide

Every check gathers all of its findings before returning, and the caller decides
whether the next check is worth running. This inverts the prevailing habit in
the current code, which returns at the first problem — and it is not merely a
style preference, because two of the requirements depend on it. Within a stage,
nothing stops early. Between stages, stopping is expected: once references are
known to be broken, checking properties on them would report confusing
consequences of a problem already reported.

The per-file parsing has the same inversion applied to it. Today a file stops at
its first problem and reports only the first diagnostic of that problem, which
is why a single malformed input can hide two-thirds of what is wrong with it.
That is a restructuring of existing code rather than a new pattern, but it is
the part most likely to be underestimated: re-classifying the problem does not
make the other problems appear.

### Type-directed path walking

The property check introduces a genuinely new pattern to this codebase:
resolving a dotted reference by stepping through types one segment at a time,
carrying "the type we have reached so far" as the walk proceeds. Each segment
either names a property on that type (advance), selects a member of a collection
(advance to the member's type), or is unknown (report and stop). The walk ends
early and accepts whatever remains as soon as the type it is standing on stops
being knowable.

The important property of this shape is that it is bounded by the reference
rather than by the type. It asks only what the path actually names, so it needs
no depth limit, terminates naturally on self-referential types, and does no work
proportional to the size of a resource schema. The alternative shape — generate
a full description of a type, then look the path up in it — is what the codebase
does elsewhere for a different purpose, and adopting it here would make the
checker's correctness depend on a depth limit chosen in advance.

Developers should expect the two stopping rules to be the subtle part. "Which
member" being unknown is not the same as "what type" being unknown, and the
distinction is invisible in the reference string itself — a map key and a field
name look identical. Only the type at that position separates them, which is why
the walk has to be type-directed rather than string-directed.

### One place that knows how names work

Deciding what a type's properties are called is currently done by a dependency,
not by this codebase, and the rule it uses is not the obvious one. Introducing
that rule here means introducing a second implementation of something that must
not drift — so it lands in exactly one place, with the awkward cases (embedded
fields that contribute their names to the parent, embedded fields that also have
a name of their own, fields that are invisible despite existing) covered by
tests rather than by comment. Any future need for the same answer should reach
for that one place.

### Following existing patterns

Nothing about the surrounding architecture changes. Validation reads the same
working set the walk reads, keyed the same way; reference discovery reuses the
existing expression traversal rather than introducing a second one, so the
references validated are exactly the references the dependency graph acts on.
Errors keep their existing types and their existing accumulation container. The
new code lives beside the parser rather than in a package of its own, because
the working set it reads is internal to the parser and exporting it purely to
validate it would widen the internal surface for no benefit.

### What a reader will notice

Three things should stand out to someone reading the result. Control flow
through parsing becomes linear and blunt — parse, validate, stop or walk — with
no branch that decides how much a problem matters. Error construction gets
shorter and more uniform, because the level argument is gone from every call
site. And a cluster of small, individually-testable checks replaces the current
arrangement where correctness decisions are scattered through decoding, with the
silent fall-through on an unresolved dependency being the most consequential one
to disappear.

## Dependencies

**No new third-party dependency is introduced.** Everything this work needs is
either already imported or in the standard library, which satisfies the
project's "prefer the standard library" and "document reasoning for third-party
dependencies" conventions without needing to invoke the latter.

### Nothing must land first

This plan has **no blocking planning dependency**. There is no prior plan for
this repo (the plan store is empty), and the spec it implements is final. Work
can begin immediately.

### Internal packages

- **`internal/parser`** — hosts the change. Provides the parse loop, the working
  set of parsed resources and their extracted links, the module recursion, and
  the walk. **Changes substantially**: gains the validation gate and the new
  checks, loses both copies of the severity classifier, and its per-file parsing
  is restructured to report every problem rather than the first.
- **`internal/resources`** — provides fully-qualified reference parsing and the
  string forms used as working-set keys, including the attribute-stripping
  variant the reference check needs. **No change expected**; this work uses an
  existing capability that the current code applies at the wrong moment.
- **`internal/schema`** — provides type description for plugin-supplied resource
  types, which cross the plugin boundary as a generated schema rather than as a
  Go type. **Read-only for this work**, and used only where no real type is
  available; its depth-capped output is treated as possibly-truncated rather
  than authoritative. The property check does not route through it for types
  that have a genuine Go type.
- **`types`** — provides the resource metadata accessors and the registry that
  maps a type name to an instance. **No change expected.**
- **`errors`** — provides the problem type and the collection that accumulates
  problems. **Changes**: loses the severity field, the level constructor
  arguments, and the two severity-interrogating methods.
- **`plugins/registry`** — provides resource-type instantiation and the roster of
  types plugin hosts supply. **No change expected**, but the validation of
  "a resource whose type cannot be checked is rejected" depends on how it reports
  an unavailable type; see the open assumption below.
- **`state`** — receives resources after validation passes. **No change**; the
  gate sits before the transfer into state.

### External libraries (all already present and pinned)

- **`hashicorp/hcl/v2`** (patched, v2.21.0) — supplies file parsing and the
  diagnostics that stage 1 reports. **No version change.** Relevant property:
  every diagnostic reachable from the parse entry point is an error, never a
  warning, so hard-failing that path cannot regress the tolerance that exists for
  values which cannot be interpolated — that tolerance lives in evaluation, which
  the parse path never performs.
- **`jumppad-labs/go-cty`** (a fork replacing `zclconf/go-cty`) — **the
  authoritative definition of what a type's properties are called at runtime.**
  It reads a different struct tag than the upstream library does, which is why
  property names cannot be derived from Go field names or from the more obvious
  tag. **No change**, but the property check must mirror its rule exactly; a
  divergence would make validation disagree with what the runtime accepts. This
  is the single most important dependency in this plan and the easiest to get
  wrong.
- **`hashicorp/terraform/dag`** — supplies the graph walk that runs after
  validation. **No change**; this work does not alter traversal.
- **`stretchr/testify`** — `require` for the new tests, per convention.
  **No change.**
- **`go-getter` / module retrieval** — **explicitly not used.** Only modules on
  the local filesystem are retrieved for checking; a module held elsewhere is
  treated as contents that cannot be obtained. This matches what the code does
  today, so nothing is added.

### Upstream spec

- **Spec `20260918075104-validation-phase-before-walk`** — final, and the source
  of truth for scope. Its constraints on stage ordering, exhaustiveness within a
  stage, and where property checking must stop are binding on the design.

### Knowledge-base dependency

- **`architecture/ux-flow.md`** (repo knowledge, always-applied) — currently
  documents the validation entry point as returning a change comparison, which
  this work withdraws. Because project knowledge is binding and outranks the code
  it describes, **this entry is updated as part of this work** rather than left
  to contradict shipped behaviour. Confirmed with the user during planning.

### Flagged risk rather than dependency

- **Plugin type availability at validation time.** The requirement that an
  uncheckable resource type must fail depends on every plugin-supplied type being
  known before validation runs. If a plugin host can register types later, during
  the walk, validation would see an incomplete roster and could reject a type
  that is in fact available. This is recorded as an open assumption; if it proves
  wrong, implementation must stop and ask rather than work around it.

## Testing Approach

### Success metrics: what the spec asks to be verified

The spec states plainly that **success is measured by its Acceptance Criteria
rather than by any separate quantitative or behavioural metric** — this is a
correctness change to a library with no external consumers yet, so there is no
baseline to measure against and no telemetry a change would show up in. So there
is no latency, throughput or adoption metric to carry forward.

All 21 acceptance criteria are verifiable as **behavioural tests**. None requires
a manual check, because every one describes an outcome of validating or applying
a configuration — something a test can set up and assert directly. Nothing is
classified as *Manual — captured in the implementation test plan*.

The spec's second metric is forward-looking rather than testable now: once there
are consumers, a configuration error reported as confusing is the signal to
re-examine whether problems surface where they can be understood. There is
nothing to assert today; it is recorded so it is not mistaken for an omission.

### How the criteria map to tests

Grouped by what they guarantee, in plain language:

**That broken configuration fails at all** — a malformed block, a reference to a
resource, variable, output or module defined nowhere, a property that a type
does not have, and a resource whose type the system has no definition for. Each
is its own test asserting failure *and* that the reported problem names the
thing that was wrong. Asserting only "it failed" would pass for the wrong
reason, since almost any defect makes a configuration fail.

**That valid configuration still passes** — picking a key from a map, a position
in a collection, or every member at once; naming a property after such a
selection; referring to a property populated only during apply; naming
properties beneath a value whose type cannot be established; selecting from a
collection whose member type is unknown. These are the tests that stop an
over-eager implementation, and they matter more than the failure cases: a
checker that rejects everything would pass every negative test.

**That the two rules do not collapse into one** — the pairing of "a property
named after a collection selection is still checked" against "a selection itself
is accepted" is the single most important assertion in the suite. A correct
implementation accepts a valid property after a splat and rejects a misspelled
one. An implementation that stops checking at the splat passes the first and
fails the second; one that refuses to step through the collection fails both.
These are written as separate positive and negative tests, never combined.

**That the gate actually gates** — applying an invalid configuration creates,
changes and destroys nothing, and validating a valid one likewise acts on
nothing. These assert on observable effects rather than on the returned error,
because the guarantee is about what did *not* happen. The existing test plugin
already records which resources it was asked to create, update and destroy,
which is exactly the observation these need.

**That every problem is reported, not just the first** — a configuration with
several faults found at the same stage, spread across more than one file, has
every one of them reported. This is asserted on the *content* of what was
reported rather than on a count alone, because a count that matches for the
wrong reason tells us nothing.

**That each problem says what and where** — every reported problem carries the
file and the position within it. Asserted across the error-producing tests
rather than as one isolated test, since location is a property every problem
must have.

**That nothing previously working broke** — a configuration accepted before this
change, containing none of the faults this work turns from advisory into
failures, still applies and still produces the same resources.

**That references reach across files and into modules** — a reference from one
file to an item defined in another resolves; a reference to an output a module
declares resolves, and one to an output it does not declare fails naming that
output; a module whose contents cannot be obtained fails against the module
itself rather than reporting every reference into it as undefined.

### Kinds of tests and where they slot in

These are **unit tests in the existing parser test package**, following the
conventions already established there: one behaviour per function, no case
tables, positive and negative cases in separate functions, testify `require`,
and the existing per-test parser setup. The naming convention for failure cases
already exists in that package and the new tests extend it rather than
introducing a second style.

Coverage concentrates on the property path check, because it carries the two
subtle stopping rules and is the only genuinely new algorithm. The type-name
resolution it depends on gets direct tests of its own — including the embedded
field that contributes a name *and* flattens, and the fields that exist in Go
but are invisible to configuration — because it mirrors a rule that lives in a
forked dependency, and a drift there would be invisible at the surface while
making validation disagree with what the runtime accepts.

**Fixtures mostly already exist.** The repository already contains a
configuration for nearly every shape the spec names — a malformed block, a
property typo, a splat into an element type, a collection index, a map key
followed by a property, a provider-populated field, cross-file and module
references. New fixtures are needed chiefly for the negative cases nothing
currently covers: a misspelled property *after* a collection selection, and a
resource of an unregistered type.

### Tests that must change, not just be added

Two existing tests assert the **opposite** of what the spec requires, because
they encode the behaviour being removed: one asserts that a reference to a
property a type does not have is advisory rather than a failure, and another
asserts that a directory containing several broken files reports a single
problem. Inverting these is part of the work. They are called out here so they
are changed deliberately rather than discovered as failures and "fixed" back to
their current expectations.

Several other tests assert exact problem counts that will legitimately change
once every problem is reported. Each needs a **decided** expected count, derived
from what the stage should find — not a number read back from a test run, which
would assert nothing.

### Deliberate gaps

- **No end-to-end test through a real plugin process.** The gate's behaviour is
  fully observable through the in-process test plugin, and the plugin transport
  is unchanged by this work.
- **No performance assertion.** The property check is bounded by reference length
  rather than by schema size, so it does not introduce the kind of cost that
  would warrant one — and the spec sets no performance target.
- **No test for the pre-existing failures unrelated to this work.** Several tests
  fail before any change is made, mostly from fixture naming drift. They are out
  of scope, and the plan's exit criterion names them explicitly so they are not
  mistaken for regressions.
- **The public validation entry point has no existing test coverage at all**, so
  the tests covering "validation alone acts on nothing" are the first at that
  layer. They are called out because there is no established local style to
  follow there, unlike in the parser package.

## Milestones & Phases

Four milestones, ordered so each stage of checking becomes trustworthy before
the next one is built on it. The first delivers the guarantee that makes the
rest meaningful — that nothing is acted upon unless the configuration is valid —
even though at that point only the cheapest checks exist. Each milestone leaves
the library in a working state.

### Milestone 1 — Broken configuration is rejected before anything is acted upon

**What changes.** A configuration that is not well-formed now fails outright
instead of being reported as an advisory note and treated as usable. Anyone
applying a configuration gets a hard stop before any resource is created,
changed or removed, rather than discovering the problem partway through. A
configuration can also be checked on its own: asking whether it is valid gives
either confirmation or the reasons it is not, and acts on nothing either way.
Where a single file has several things wrong with it, all of them are now
reported together instead of just the first, so authors stop fixing one problem
only to rerun and meet the next. The change-preview that the validation entry
point used to return is withdrawn as part of this — producing it required
resolving the configuration, which is exactly what stops validation being a
check you can trust on its own.

**Validation point.** A malformed configuration fails rather than succeeding,
and applying it leaves nothing created, updated or destroyed. Asking to validate
a good configuration reports success and acts on nothing. A file containing
several malformations reports all of them. The test that motivated this work —
which currently fails because a syntax error is classified as a warning — passes.

### Milestone 2 — References to things that do not exist are reported

**What changes.** Naming a resource, variable, output or module that is defined
nowhere is now a failure that names what could not be found and where, rather
than passing silently and surfacing later as a confusing downstream symptom.
This works across a whole configuration rather than file by file, so a reference
from one file to something defined in another resolves correctly, and one that
resolves nowhere in any file is reported. References into a module are checked
the same way as references to anything else. Where a module's contents cannot be
obtained at all, the failure names the module itself rather than flooding the
report with every reference into it. Every bad reference in the configuration is
reported in one pass.

**Validation point.** References to an undefined resource, variable, output and
module each fail and name the missing item. A reference spanning two files
resolves. A reference to an output a module declares resolves; one to an output
it does not declare fails naming that output. A configuration with several bad
references across more than one file reports every one of them. A module whose
contents cannot be obtained is reported against the module, and references into
it are not additionally reported as undefined.

### Milestone 3 — Properties that a type does not have are reported

**What changes.** Referring to a property that the referenced item's type does
not have is now a failure naming that property, instead of being tolerated.
Crucially, this does not come at the cost of rejecting valid configuration:
picking a key from a map, a position in a collection, or every member of one at
once is accepted, and checking continues into whatever those members are — so a
misspelled property written after such a selection is still caught. Where a
reference reaches something whose type cannot be established before the
configuration is applied, the rest of the reference is accepted rather than
reported as naming unknown properties. In short, authors get told about real
typos in property names without being told off for references the system cannot
yet know the value of.

**Validation point.** A reference to a property that does not exist fails naming
it. A reference through a map key, through a collection position, and through
every member at once each succeed when the property that follows is real — and
fail naming the property when it is not. A reference to a property populated
only during apply succeeds, as does one naming properties beneath a value of
indeterminate type, and one selecting from a collection whose member type cannot
be established. A resource whose type the system has no definition for fails.

### Milestone 4 — Problem reporting is consistent and the old severity system is gone

**What changes.** This milestone is mostly internal cleanup, and it is worth its
own milestone for one reason: until it lands, two ways of judging a problem
coexist. The old system classified problems as advisory or fatal while a
configuration was being resolved, which is what allowed genuine mistakes to be
reported as warnings in the first place. With checking now exhaustive, that
classification has nothing left to decide, and leaving it in place would let
severity quietly continue standing in for which stage a problem was found at —
the exact confusion this work exists to remove. Users see one visible
difference: every reported problem now carries the file and position it occurs
at, uniformly, making the whole report usable by an interface that shows
problems against the configuration. The project's own documentation is brought
in line with the withdrawn change-preview at the same time, so the recorded
description of how validation behaves matches what it actually does.

**Validation point.** No problem can be classified as advisory anywhere in the
codebase — the classification and both of its copies are gone. Every problem
reported by any stage carries a file and a position. A configuration that was
accepted before this work, and contains none of the faults this work turns from
advisory into failures, still applies and produces the same resources. The
project documentation no longer describes the withdrawn change-preview.

---

### Milestone 1 — Broken configuration is rejected before anything is acted upon

#### - [x] Phase 1.1: Report every problem in a file, and treat malformed configuration as a failure

**Repo:** xclconfig

Today a file stops at its first problem and reports only the first detail of
that problem, so a single malformed input can hide two-thirds of what is wrong
with it. This phase changes per-file parsing to gather everything it finds
before returning, and stops treating a malformed block as advisory. The
tolerance that exists for values which cannot be interpolated is untouched —
that lives in a later stage of processing which this phase does not reach.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-report-every-problem-in-a-file-and-treat-malformed-configuration-as-a-failure)

**Acceptance criteria**:
- [x] A configuration file containing a block that is not well-formed is reported as a failure rather than as an advisory note
- [x] A file with several separate malformations reports all of them, not only the first
- [x] A single malformation that produces several related complaints reports all of them rather than only the first
- [x] Each reported problem states the file and the position within it
- [x] A configuration whose only fault is a value that cannot yet be interpolated is still tolerated at this stage

#### - [x] Phase 1.2: Add the validation gate and make validation a standalone answer

**Repo:** xclconfig

This phase introduces the checking stage itself — running to completion after a
configuration has been read and before any of it is acted upon — and wires it
into both entry points at once. Asking whether a configuration is valid now
gives a straight answer and acts on nothing; applying one runs the same check
first and refuses to proceed if it fails. The change-preview the validation
entry point used to return is withdrawn here, because producing it requires
resolving the configuration, which is what stopped validation being trustworthy
on its own.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-add-the-validation-gate-and-make-validation-a-standalone-answer)

**Acceptance criteria**:
- [x] Asking whether a valid configuration is valid reports success and creates, changes or removes nothing
- [x] Applying an invalid configuration reports failure and creates, changes or removes nothing
- [x] Validation considers the whole configuration as one unit, so a file is never judged in isolation
- [x] The change-preview is no longer produced or offered by the validation entry point
- [x] A configuration that was valid before this change is still accepted and still produces the same resources

#### - [x] Phase 1.3: Fail on a resource whose type cannot be checked, and guard module recursion

**Repo:** xclconfig

A configuration naming a resource type the system has no definition for cannot
be checked at all, so accepting it would leave a hole in the guarantee that
nothing invalid is acted upon — this phase makes it a failure. It also adds a
guard against a module whose source includes itself, which currently recurses
until the process runs out of stack; that failure is now reported against the
module, like any other module whose contents cannot be obtained.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-fail-on-a-resource-whose-type-cannot-be-checked-and-guard-module-recursion)

**Acceptance criteria**:
- [x] A configuration containing a resource of a type the system has no definition for is reported as a failure
- [x] The reported problem names the type that could not be checked and where it appears
- [x] A module whose source directly or indirectly includes itself is reported as a failure instead of exhausting the stack
- [x] A module whose contents cannot be obtained is reported against the module itself

### Milestone 2 — References to things that do not exist are reported

#### - [x] Phase 2.1: Resolve references against the whole configuration

**Repo:** xclconfig

Establishes how a reference is matched against everything the configuration
defines. Today a reference that matches nothing is silently ignored, partly
because references carry a trailing property path that the things they name do
not — so the comparison never had a chance to succeed. This phase puts that
reconciliation in one place, so that the answer to "does this exist" is decided
once and the leftover property path is handed on for later checking.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-resolve-references-against-the-whole-configuration)

**Acceptance criteria**:
- [x] A reference is matched against everything defined anywhere in the configuration, including in other files
- [x] A reference that names something real is recognised as such regardless of whether it also names properties on it
- [x] The property path following a reference is separated from the part that names the thing, so each can be judged on its own
- [x] Resolution reaches items defined inside modules as readily as those declared directly

#### - [x] Phase 2.2: Report references that name nothing

**Repo:** xclconfig

Turns the resolution from the previous phase into reported failures. A
reference naming a resource, variable, output or module that is defined nowhere
now fails and says what could not be found and where. All such problems across
the configuration are reported together, so an author is not made to fix one and
rerun to discover the next.

*Technical detail:* [context.md#phase-22](./context.md#phase-22-report-references-that-name-nothing)

**Acceptance criteria**:
- [x] A reference to a resource that is defined nowhere fails and names it
- [x] A reference to a variable that is defined nowhere fails and names it
- [x] A reference to an output that is defined nowhere fails and names it
- [x] A reference to a module that is defined nowhere fails and names it
- [x] A configuration with several unresolvable references, spread across more than one file, reports every one of them
- [x] Each reported problem states the file and position where the reference appears
- [x] A reference from one file to an item defined in another is accepted

#### - [x] Phase 2.3: Check references into modules, and attribute unobtainable modules correctly

**Repo:** xclconfig

References into a module are checked like any other reference rather than
deferred. The important part is what happens when a module's contents cannot be
obtained at all — for instance because it is held somewhere validation does not
retrieve from. In that case the failure names the module, and references into it
are not additionally reported as naming things that do not exist, so one problem
does not produce a page of misleading consequences.

*Technical detail:* [context.md#phase-23](./context.md#phase-23-check-references-into-modules-and-attribute-unobtainable-modules-correctly)

**Acceptance criteria**:
- [x] A reference to an output a module declares is accepted
- [x] A reference to an output a module does not declare fails and names that output
- [x] A module whose contents cannot be obtained is reported once, against the module
- [x] References into a module whose contents cannot be obtained are not reported as naming undefined things
- [x] A module held somewhere other than the local filesystem is treated as contents that cannot be obtained

### Milestone 3 — Properties that a type does not have are reported

#### - [x] Phase 3.1: Establish what properties a type has

**Repo:** xclconfig

Introduces the single authoritative answer to what a given type's properties are
called. This is subtler than it appears: the naming rule that governs what the
system accepts at runtime is not the obvious one, and it is currently defined
outside this codebase entirely. Getting it wrong in either direction would make
checking disagree with what actually works, so this phase establishes it once,
with the awkward cases pinned down by tests.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-establish-what-properties-a-type-has)

**Acceptance criteria**:
- [x] A type's property names match exactly what the system accepts for it when a configuration is applied
- [x] Properties contributed by an embedded part of a type are available under the containing type
- [x] An embedded part that also carries a name of its own is reachable both ways
- [x] A field that exists in the code but is not addressable in configuration is reported as not existing
- [x] Types supplied by a plugin are handled the same way as those built in

#### - [x] Phase 3.2: Check property paths, continuing through member selection

**Repo:** xclconfig

Walks a property path one step at a time against the type it is being read from.
The heart of this phase is the distinction between not knowing *which* member of
a collection is meant and not knowing *what type* that member is. Picking a key,
a position, or every member at once is accepted, and checking then continues
against whatever those members are — so a misspelling written after such a
selection is still caught rather than waved through.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-check-property-paths-continuing-through-member-selection)

**Acceptance criteria**:
- [x] A reference to a property that does not exist on an otherwise valid resource fails and names that property
- [x] Picking a key within a map, followed by a property those members do have, is accepted
- [x] Picking a position within a collection, or every member at once, followed by a property those members do have, is accepted
- [ ] A property that members of a collection do not have is reported, even when named after selecting from that collection — *partially met: reported for the `volume.0.x` and `volume[0].x` forms; the splat form `volume.*.x` is unreachable because reference collection drops everything after the `*` (`exp.go` `SplatExpr` traverses `Source` but not `Each`), a pre-existing blind spot this plan puts out of scope under "Reference collection is not widened"*
- [x] Every property problem in the configuration is reported, not only the first

#### - [x] Phase 3.3: Stop checking where the type is no longer knowable

**Repo:** xclconfig

Bounds the check so it does not reject configuration it cannot judge. Where a
path reaches a property populated only while a configuration is being applied,
or passes through a value whose type cannot be established beforehand, the rest
of the path is accepted rather than reported as naming unknown properties. This
is the phase that keeps the previous one from rejecting valid configuration.

*Technical detail:* [context.md#phase-33](./context.md#phase-33-stop-checking-where-the-type-is-no-longer-knowable)

**Acceptance criteria**:
- [x] A reference to a property that is only populated while a configuration is being applied is accepted
- [x] Properties named beneath a value whose type cannot be established beforehand are accepted rather than reported
- [x] Selecting a member of a collection whose member type cannot be established, then naming a property beneath it, is accepted
- [x] Nothing is rejected merely because the description of a type was incomplete
- [x] Configurations already present in the project that use these shapes continue to be accepted

### Milestone 4 — Problem reporting is consistent and the old severity system is gone

#### - [x] Phase 4.1: Remove the advisory/fatal classification

**Repo:** xclconfig

With checking now exhaustive, classifying a problem as advisory rather than
fatal has nothing left to decide. This phase removes it entirely — including
both copies of the classifier and a comparison that could never match on the
path it was written for — rather than correcting it in place, so that severity
cannot quietly continue standing in for which stage a problem was found at.

*Technical detail:* [context.md#phase-41](./context.md#phase-41-remove-the-advisoryfatal-classification)

**Acceptance criteria**:
- [x] No problem anywhere in the codebase can be marked as advisory rather than fatal
- [x] Both copies of the classification logic are gone, not just one
- [x] Asking whether a set of reported problems contains real errors is no longer possible or necessary — any reported problem is a failure
- [x] No rendered problem output changes as a result of this removal
- [x] Tests that previously asserted a problem was advisory now assert it is a failure

#### - [x] Phase 4.2: Bring documentation and project knowledge in line

**Repo:** xclconfig

The project's recorded description of how validation behaves still describes the
withdrawn change-preview, and project knowledge is treated as binding here — so
leaving it would mean the authoritative description contradicts the shipped
behaviour. This phase updates both the documentation and the knowledge entry to
describe validation as it now works.

*Technical detail:* [context.md#phase-42](./context.md#phase-42-bring-documentation-and-project-knowledge-in-line)

**Acceptance criteria**:
- [x] The project's recorded description of the validation entry point matches its actual behaviour
- [x] No documentation or knowledge entry still describes the withdrawn change-preview
- [x] The three stages of checking are described where a contributor would look for them
- [x] A reader of the project knowledge would not be misled about what validation returns

#### - [x] Phase 4.3: Confirm nothing that worked before has broken

**Repo:** xclconfig

A closing check across the whole change. Because several tests fail in this
project before any of this work begins, "everything passes" is not a usable
signal — this phase establishes what did change against what was already
failing, so a genuine regression cannot hide among pre-existing noise.

*Technical detail:* [context.md#phase-43](./context.md#phase-43-confirm-nothing-that-worked-before-has-broken)

**Acceptance criteria**:
- [x] A configuration accepted before this work, containing none of the faults this work turns from advisory into failures, still applies and produces the same resources
- [x] Every test that passed before this work still passes
- [x] Every test that previously asserted behaviour this work deliberately changes has been updated on purpose, with the new expectation justified rather than read back from a test run
- [x] Tests that were already failing before this work, for reasons unrelated to it, are identified as such rather than silently adopted or "fixed"
- [x] The test that motivated this work passes

## Open Questions

Two questions remain that genuinely cannot be settled before implementation
begins. Four others that were open after discovery have since been resolved by
reading the code, and are recorded here as closed so a future reader does not
reopen them.

### Open — resolve during implementation

**1. Whether `Validate` can stop before resolution without a wider change than
Phase 1.2 anticipates.**

The parse entry point currently always runs the dependency walk, including when
plugins are switched off — the walk is what decodes each resource's fields, so
skipping it changes what the entry point produces, not merely what it executes.
Satisfying the spec's requirement that validation performs no resolution work
therefore needs the walk made genuinely optional, and it is not knowable ahead of
time whether anything downstream quietly depends on validation having decoded.
There are no tests at that layer to tell us.

**Depends on:** whether any caller or test relies on a side effect of the walk
during validation.
**What to do:** proceed with making the walk conditional, as Phase 1.2 describes.
If it turns out something downstream depends on decoded values during
validation — that is, if validation cannot answer its question without
resolving — **STOP and ask the user**, because that would put the spec's
"validation must not perform resolution work" constraint in direct conflict with
existing behaviour, and the resolution is theirs to choose.

**2. Whether reporting problems at block granularity is sufficient in practice.**

Each problem will report the file and the position of the resource block the
reference appears in, rather than the exact position of the reference within
that block, because references are collected without their source positions
attached. This satisfies the spec's requirement that each problem states where
it occurs, and is enough to place a marker on the right resource. Whether it is
precise enough for the interface the spec names as the intended consumer can only
be judged once problems are actually being reported.

**Depends on:** how the reported problems read once a real configuration with
several faults is validated.
**What to do:** deliver block granularity as planned and look at the output. If
it proves too coarse to be useful, **raise it with the user** rather than
expanding scope unilaterally — capturing exact positions means changing the
shared reference-collection path that also feeds the dependency graph, which is
a materially larger change than this plan sizes for.

### Closed during planning — do not reopen

- **Do module outputs resolve without the module's evaluation context?**
  **Yes.** That context holds only the module's supplied *variables*, never its
  outputs. Module outputs are ordinary output declarations already parsed into
  the working set at parse time, so a reference to one resolves by lookup.
  The concern raised during specification was misplaced.
- **Are plugin-supplied types known before validation runs?**
  **Yes.** Plugins register their hosts into the registry before parsing starts,
  and the type roster is read from those hosts. Validation sees a complete
  roster; there is no late registration during the walk.
- **Does a resource of an unavailable type already fail?**
  **Largely yes** — it already produces a fatal error at parse time. The genuine
  gap is that the message does not name the offending type. Phase 1.3 begins by
  confirming this rather than assuming a gap exists.
- **Does removing the advisory classification change any reported output?**
  **No.** The rendering never consulted it — an advisory problem already printed
  identically to a fatal one. Only the decision about whether a configuration is
  usable changes.

## Out of Scope

### From the spec's non-goals

- **The change-preview is withdrawn, not reworked.** Reporting what would change
  requires resolving the configuration, so it cannot be produced by validation
  alone. It is removed rather than preserved by some other route. It may return
  as a later piece of work if it is wanted — that would need its own spec, since
  it would have to sit on the resolving side of the new boundary.
- **Retrieving modules from anywhere but the local filesystem.** This work does
  not add the ability to fetch a module held elsewhere, nor to act on a declared
  module version. Such modules are treated as contents that cannot be obtained
  and reported against the module. Note the version field already exists in the
  configuration format and is already never read — this work does not start
  reading it. Supporting remote modules is separate work with no spec yet.
- **Editor-facing diagnostics.** Reporting problems individually, each with its
  own location, is intended to make such an interface possible — but building
  one, surfacing problems as a configuration is typed, is not part of this. No
  follow-up spec exists yet.
- **New checks during application.** This work moves checks earlier and makes
  their outcome decisive. It does not add checks to the stage where a
  configuration is acted upon, and does not change how resources are acted upon.

### Decided during planning

- **Pre-existing test failures are not fixed.** Ten tests fail and three test
  packages do not compile before any of this work begins. Two of those failures
  are the defect this work removes and will be fixed; the rest — chiefly tests
  pointing at fixture files under an old extension, and one with an incomplete
  mock setup — are unrelated and stay as they are. They are recorded in the
  plan's technical notes so a reader can tell them apart from a regression. A
  separate tidy-up would be worth doing and needs no spec.
- **Existing lint findings are not cleared.** The static checker already reports
  findings in the plugin and example packages. This work must not add to them,
  but clearing them is unrelated tidy-up.
- **Reference collection is not widened.** The existing expression traversal is
  reused rather than replaced, and its known blind spots — certain expression
  forms that yield no references at all — are left exactly as they are. Widening
  it would change what the dependency graph sees, not just what validation sees,
  which is a materially different and riskier change. Recorded as a known gap.
- **Improving cycle detection is conditional, not committed.** Today only the
  simplest cycles are caught where a problem can be attributed to a file and
  position; longer cycles surface later with no location attached. Bringing them
  into the new checking stage would improve them, but no requirement or
  acceptance criterion in the spec mentions cycles, so it is done only if it
  falls out cheaply and otherwise recorded as a gap.
- **Exact positions within a resource block are not delivered.** Problems are
  reported against the resource block a reference appears in, not the exact
  position of the reference within it. Capturing finer positions means changing
  the shared collection path that also feeds the dependency graph. Flagged as an
  open question to revisit once real output can be judged.
- **The historical to-do notes are not rewritten.** They mention the withdrawn
  change-preview as part of the project's record; the documentation and project
  knowledge are corrected, but the historical record is left intact.

### Deliberately not changed by the design

- **No new dependency.** The type walking uses the standard library only.
- **The plugin transport is untouched.** Nothing about how plugin-supplied types
  cross the process boundary changes, so no end-to-end test through a real
  plugin process is added.
- **Resource state and its persistence are untouched.** Validation runs before
  anything reaches state and adds nothing to it.
- **The depth limit on plugin-supplied type descriptions is not raised.** A
  description truncated at that limit is treated as unknowable and accepted
  rather than rejected. Raising the limit would be a change to the plugin
  boundary and risks a false rejection today; accepting at the boundary is the
  safe behaviour and is what this work does.

## Changelog

### 2026-09-18 — Phase 1.1: Report every problem in a file, and treat malformed configuration as a failure

**What was done**: `parseResourcesInFile` now reports every problem it finds in a
file instead of the first. The HCL diagnostic path emits one `ParserError` per
diagnostic — each carrying its own line and column — replacing a single error
built from `diag[0]`, and the dead severity classifier that defaulted every
syntax error to advisory was deleted. The block loop likewise accumulates rather
than returning at the first bad block, so a file with two malformed blocks
reports both.

**Deviations**: The unknown-stanza error in the `default:` branch was promoted
from `ParserErrorLevelWarning` to `ParserErrorLevelError`. The plan assigns
severity removal to Phase 4.1, but that branch already aborted the file while
being labelled advisory, and leaving it as a warning while making the loop
accumulate would have let a malformed stanza pass the Phase 1.2 gate —
`ContainsErrors()` reads the level until 4.1 deletes it. Phase 4.1 removes the
constant entirely, so this is the same direction, taken earlier.

**Files changed**:
- `internal/parser/parser.go`
- `internal/parser/parse_test.go`
- `internal/test_fixtures/config/multiple_errors/two_blocks_missing_names.xcl`
- `internal/test_fixtures/config/multiple_errors/unterminated_string.xcl`

**Discoveries**:
- **`NewParserErrorFromHCLDiag` needed no extension.** context.md flagged it as
  possibly not setting line/column; it already does, and already hardcodes
  `ParserErrorLevelError` (`errors/parser_error.go:162-177`). Reused as-is. Its
  other call site, `internal/parser/util.go:131`, still reads only `diag[0]` and
  is a candidate for the same treatment.
- **HCL halts on a malformed block *header*.** A file with two bad block headers
  yields only **one** diagnostic, so a fixture proving within-file accumulation
  must use two *recoverable* faults — two blocks missing their name labels, which
  travel the block loop rather than the diagnostic loop. This shaped the fixture
  design and is easy to get wrong.
- **Measured diagnostic counts** (for deciding test expectations rather than
  reading them back): `bad_format.xcl` → 1; an unterminated string → 3; an
  unclosed `${` → 2; `bad_interpolation.xcl`, `function_error.xcl` and all three
  `config/invalid/*.xcl` → 0 parse diagnostics, their errors coming from the
  block loop instead. Consequently the counts at `parse_test.go` for
  `bad_format.xcl` (1) and `config/invalid/` (3) legitimately stay unchanged.
- **`TestParseFileReturnsConfigErrorWhenInvalidFileFails` is not Phase 1.1's to
  fix.** Its fixture is a *nonexistent* path, so `findXclFiles` fails at
  `parser.go:243-246` and `Parse` returns a `fmt.Errorf` wrap, never reaching
  `parseResourcesInFile`. Making `Parse` return a `ConfigError` consistently
  belongs with the **Phase 1.2** gate work.

### 2026-09-18 — Phase 1.2: Add the validation gate and make validation a standalone answer

**What was done**: `Parse` was split so that everything up to and including the
verdict now lives in a new unexported `parseAndValidate`, which loads state,
discovers and parses every file, applies the parse gate, then runs the new
validation gate before any resource moves into state. A new
`internal/parser/validate.go` holds `(*Parser).validate()`, which owns stage
ordering and accumulates every problem rather than returning the first; its
stage 1 is a deliberate no-op anchoring the ordering until stages 2 and 3 land.
`Config.Validate` now returns `error` alone, `diff.go` was deleted outright, and
the example was rewritten to the error-only contract.

**Deviations**: Two, both deliberate and both agreed with the user.

1. **`Config.Validate` still runs the walk.** The plan required validation to
   stop before the walk. Research showed that would lose two checks the walk
   currently provides and that nothing yet replaces: indirect cycle detection
   (`d.Validate()` lives inside the walk, and only direct 2-node cycles are
   caught earlier, so the existing suite would not catch the regression) and
   schema/decode errors from `gohcl.DecodeBody`. Validation would have become
   strictly weaker than before this work until Milestone 3. The user chose to
   keep the walk for now and remove it at the **end of Milestone 3**, once
   stages 2 and 3 replace those checks. **This is an outstanding task, not a
   dropped one** — the spec's "validation performs no resolution work"
   constraint is deferred, not abandoned.
2. **Mechanism**: an unexported `parseAndValidate` rather than a new parameter
   on `Parse`. The `executePlugins` bool is overloaded — 38 of 41 test call
   sites pass `false` and then read decoded fields — so redefining `false` to
   skip the walk would have broken the parser suite wholesale. Renaming the
   public `Parse` to `Apply` was raised and explicitly deferred by the user.

Additionally, path-discovery failures now return a `ConfigError` rather than a
bare `fmt.Errorf` wrap, which fixes the second in-scope baseline failure.

**Files changed**:
- `internal/parser/validate.go` (new)
- `internal/parser/parser.go`
- `internal/parser/parse_test.go`
- `config.go`
- `config_validate_test.go` (new)
- `diff.go` (deleted)
- `example/main.go`

**Discoveries**:
- **A root-package test CAN import `internal/parser`.** The plan asserted
  otherwise and it is wrong: the module is `github.com/jumppad-labs/xcl` and the
  root package is `xcl`, so `internal/...` is importable from the repo root —
  Go's internal rule only blocks importers outside the subtree rooted at
  `internal`'s parent. `TestPlugin` also lives in `test_plugin.go`, a non-test
  file, so it is ordinary package surface. This allows direct
  `GetCreatedResources()` assertions from root-package tests instead of a weaker
  state-store proxy, and applies to every later phase.
- **`executePlugins=false` gates exactly one thing** — `callProviderLifecycle`
  at `callbacks.go:156`. `gohcl.DecodeBody` runs regardless, so today `Validate`
  and `Apply` perform identical resolution work and differ only in whether
  providers are invoked. The doc comment at `parser.go:179` claiming `false`
  "tolerates missing interpolated values" is not backed by any code.
- **Indirect cycle detection is inside the walk** (`d.Validate()`,
  `parser.go:882`). Direct 2-node cycles are caught at parse time, so
  `TestParserCyclicalReferenceReturnsError` would keep passing even if the check
  were lost. Anything that moves or removes the walk must account for this.
- **The failing `TestParseFileReturnsConfigErrorWhenInvalidFileFails` errored in
  `findVarsFiles`, not `findXclFiles`** — vars files are discovered ~30 lines
  earlier, so execution never reached the xcl lookup. Both sites were converted.
- **`TestParserErrorsOnPluginCreateError` is flaky**, not a regression. It failed
  once under `go test ./...` parallel load and passed 10 consecutive isolated
  runs plus repeated full-suite runs. It spawns plugin processes and is sensitive
  to contention. Do not add it to the baseline.
- **`TODO.md:146-147` is an unchecked roadmap item asking for *enhanced* Diff**,
  which directly contradicts deleting it. Flagged to the user; deletion proceeded
  per the spec, since the library has no consumers yet.
- **`docs/parser-lifecycle.md:28`** documents `executePlugins=false` as the mode
  `Config.Validate` uses for "schema/DAG checking". It becomes wrong under this
  work — add it to **Phase 4.2** scope alongside `docs/overview.md` and
  `docs/README.md`.

### 2026-09-18 — Phase 1.3: Fail on a resource whose type cannot be checked, and guard module recursion

**What was done**: A resource whose type has no definition already failed, so the
work here was to make the report name the offending type rather than the literal
word "resource". Module parsing gained a guard against a source that includes
itself: source directories are resolved to a canonical path and recorded as they
are entered, so re-entry is reported against the module instead of recursing
until the stack is exhausted. A module source that cannot be resolved at all is
now reported against the module too, and module child files accumulate their
problems rather than stopping at the first.

**Deviations**: None. The plan instructed verifying before coding on the type
half, which was correct: an unregistered type already produced a fatal error,
and only the message needed changing.

**Files changed**:
- `internal/parser/parser.go`
- `internal/parser/util.go`
- `internal/parser/parse_test.go`
- `internal/test_fixtures/config/unregistered_type/unknown.xcl`
- `internal/test_fixtures/config/self_including_module/self.xcl`
- `internal/test_fixtures/config/mutual_modules/a/a.xcl`
- `internal/test_fixtures/config/mutual_modules/b/b.xcl`
- `internal/test_fixtures/config/missing_module_source/missing.xcl`
- `internal/test_fixtures/config/module_leaf/leaf.xcl`
- `internal/test_fixtures/config/module_reused_twice/reuse.xcl`

**Discoveries**:
- **The visited set must be scoped to the recursion, not the whole parse.** The
  guard removes each source from the set once that module's children are parsed
  (`defer delete`). Without that, a module legitimately used twice as siblings
  would be wrongly rejected the second time. The positive-control test covering
  this is the most valuable one in the phase — a guard that over-blocks passes
  every negative test.
- **A ninth pre-existing failure exists that the plan's baseline table misses.**
  `TestPluginResourceCreationWithFallback`
  (`internal/parser/parser_plugin_test.go:51`) panics on a nil `PluginRegistry`
  at `plugins/registry/plugin_registry.go:37`. It is masked in ordinary runs
  because `TestDestroyLifecycle` panics earlier in the same package and aborts
  the run before it executes. Verified pre-existing by stashing all work and
  running it alone. It is out of scope, but **Phase 4.3 must count it** or it
  will read as a regression introduced by this work.
- **Enumerating parser failures needs an explicit `-run` list.** Because two
  tests panic rather than fail, a plain package run stops early and under-reports
  what is broken.

### 2026-09-18 — Phase 2.1: Resolve references against the whole configuration

**What was done**: A new reference resolver answers whether the thing a
reference names exists and hands back the property path left over. The part
naming the resource is separated from the trailing property path *before* the
lookup, which is what makes the match succeed at all: the working set is keyed
without a trailing property path while references routinely carry one, and the
existing dependency lookup stripped it afterwards and so missed. Nothing
consumes the resolver yet; Phase 2.2 turns its answers into reported failures.

**Deviations**: One addition the plan did not anticipate, described under
Discoveries: resolution is scoped to the referring resource's module before
falling back to the whole configuration. Without it the resolver reports valid
references as missing.

**Files changed**:
- `internal/parser/references.go` (new)
- `internal/parser/references_test.go` (new)
- `internal/parser/parser.go`

**Discoveries**:
- **References made inside a module are written unqualified, so resolution must
  be module-scoped first.** A resource inside module `consul_1` refers to
  `variable.cpu_resources`, while the working-set key is
  `module.consul_1.variable.cpu_resources`. The referring resource's
  `Meta.Module` supplies the missing scope, so a reference is resolved against
  `module.<Meta.Module>.<base>` first and the bare `<base>` second. Measured
  before adding this: `config/modules/modules.xcl` reported **6 valid references
  as unresolvable**. A single-lookup implementation would therefore have broken
  every module configuration while still passing every negative test — the same
  over-eager failure mode the plan warns about for Phase 3.2. Measured after:
  17/39/12/2 links across the simple, modules, interpolation and cyclical-pass
  fixtures, **zero misses**. Those counts are asserted directly so the sweep
  cannot pass on an empty set.
- **`getUniqueResourceLinks` discarded an error**, as the plan noted; it now
  handles it, matching the sibling call site a few lines below that always did.

### 2026-09-18 — Phase 2.2: Report references that name nothing

**What was done**: Validation gained its second stage, which turns the previous
phase's resolver into reported failures. Every reference held by every resource
is resolved, and each one that names nothing is reported together with the rest
rather than stopping at the first, naming both the resource that holds the
reference and the thing that could not be found. Stage 3 is skipped whenever
stage 2 finds anything, so properties are never checked on a reference that
resolves nowhere.

**Deviations**: None.

**Files changed**:
- `internal/parser/validate.go`
- `internal/parser/validate_test.go` (new)
- `internal/test_fixtures/config/undefined_resource_reference/container.xcl`
- `internal/test_fixtures/config/undefined_variable_reference/container.xcl`
- `internal/test_fixtures/config/undefined_output_reference/output.xcl`
- `internal/test_fixtures/config/undefined_module_reference/output.xcl`
- `internal/test_fixtures/config/undefined_references_multi_file/a.xcl`
- `internal/test_fixtures/config/undefined_references_multi_file/b.xcl`
- `internal/test_fixtures/config/resolving_cross_file_reference/network.xcl`
- `internal/test_fixtures/config/resolving_cross_file_reference/container.xcl`

**Discoveries**:
- **Block-granular positions are sufficient; the plan's open question is
  closed.** The plan deferred judging this until problems were actually being
  reported. They now are, and the rendered error includes a source excerpt
  around the reported line, so the offending reference appears directly beneath
  the highlighted block header. Capturing exact expression positions would mean
  changing the shared reference-collection path that also feeds the dependency
  graph — materially larger, and unnecessary given how the output reads. No
  escalation needed.
- **Report order had to be made deterministic.** The working set is a map, so
  iterating it directly reported the same faults in a different order on every
  run. Resource keys are now sorted before iteration, which is what lets a test
  assert on several problems by position in the collection.
- **Reporting every problem depends on Phase 2.1's module scoping.** Without it
  this stage would have reported six valid references in the modules fixture as
  undefined. The four fixture-wide acceptance tests (simple, modules,
  interpolation, cyclical-pass) exist to catch exactly that regression.

### 2026-09-18 — Phase 2.3: Check references into modules, and attribute unobtainable modules correctly

**What was done**: Nothing, in production code. Every requirement of this phase
turned out to be satisfied already by the phases before it, so the work was to
probe each one, confirm it, and write the tests that lock the behaviour in
against future change.

**Deviations**: The plan expected this phase to add a suppression filter, run
after stage 2 collected its findings, to stop references into a broken module
being reported alongside the module itself. That filter is not needed and was
not written — see Discoveries. Adding it would have been dead code.

**Files changed**:
- `internal/parser/validate_test.go`
- `internal/test_fixtures/config/undeclared_module_output/main.xcl`
- `internal/test_fixtures/config/unobtainable_module_references/main.xcl`
- `internal/test_fixtures/config/remote_module_source/main.xcl`

**Discoveries**:
- **Suppression is structural, not a filter.** An unobtainable module is
  reported by `parseModule` during *parsing*, so the parse gate returns before
  stage 2 ever runs and references into that module are never examined. Probed
  with an unobtainable module and three inbound references: **exactly one
  problem**, against the module. The test asserts the count is one and that no
  reported message says "not defined anywhere in the configuration", so a future
  reordering that broke this would be caught.
- **The plan's open assumption about module outputs is confirmed.** Module
  outputs resolve statically without the module's `SubContext`: they are
  ordinary output blocks already in the working set at parse time. A declared
  output resolves, an undeclared one does not. The spec-phase concern was
  misplaced, as the plan suspected but could not prove.
- **A remote-looking module source reports the first missing path segment, not
  the whole source.** `source = "github.com/org/repo"` is joined onto the
  declaring file's directory, and the resolution fails at `<dir>/github.com`
  while the reported source is the full join. The message is therefore
  asymmetric between its two halves, which a test asserting on it must account
  for.

### 2026-09-18 — Phase 3.1: Establish what properties a type has

**What was done**: A single place now answers what a type's properties are
called in configuration. The rule is not this codebase's own — it belongs to the
fork of go-cty the project pins, which reads a different struct tag than the
upstream library — so it was read from the fork directly and mirrored, with
every awkward case pinned down by a test rather than described in a comment.

**Deviations**: None.

**Files changed**:
- `internal/parser/types.go` (new)
- `internal/parser/types_test.go` (new)

**Discoveries**:
- **The resolver can be checked against the runtime itself, and is.** Rather
  than only asserting hand-written name lists, the tests compute what the forked
  go-cty derives for a type and require the two sets match in **both**
  directions. Measured agreement is exact — Container 21 vs 21, Network 4 vs 4,
  Template 8 vs 8, with nothing on either side the other lacks. This turns the
  phase's central criterion into an executable check that cannot silently drift
  as the fork changes, which a hand-maintained list would not survive.
- **That cross-check was mutation-tested rather than trusted.** Injecting a
  spurious name into the resolver, and separately dropping a real one, each made
  it fail with a message naming the offending key. A green run alone would not
  have shown the assertion had teeth.
- **An embedded field's two behaviours are independent, not alternatives.** A
  named embed registers its own name *and* flattens its contents into the
  parent, so the two rules must be applied separately. The one type in the tree
  that does this contributes both `rm` and the flattened `meta`, `depends_on`
  and `disabled`; treating the name and the flattening as mutually exclusive
  would lose one or the other.
- **Real Go fields can be invisible to configuration**, and must be reported as
  not existing. Three fields on the metadata type carry only `json` tags and are
  correctly absent from the result.

### 2026-09-18 — Phase 3.2: Check property paths, continuing through member selection

**What was done**: A property path is now walked one segment at a time against
the type reached so far, so a reference naming a property its target's type does
not have is reported, naming that property. Selecting a member of a collection
continues the walk against the member's type rather than stopping it, which is
what allows a misspelling written after a selection to be caught. Phase 3.3's
stopping rules were built into the same walk rather than bolted on afterwards,
because the two are meaningless apart: every stopping rule below was needed to
stop the walk rejecting configuration that already works.

**Deviations**: One acceptance criterion is only partially met, and its checkbox
is deliberately left unticked with the reason recorded inline in the plan — see
Discoveries.

**Files changed**:
- `internal/parser/properties.go` (new)
- `internal/parser/properties_test.go` (new)
- `internal/parser/validate.go`
- `internal/parser/validate_test.go`
- `internal/parser/parse_test.go`
- `internal/test_fixtures/config/bad_property_after_selection/container.xcl`
- `internal/test_fixtures/config/multiple_property_problems/a.xcl`
- `internal/test_fixtures/config/multiple_property_problems/b.xcl`
- `internal/test_fixtures/config/valid_property_paths/main.xcl`

**Discoveries**:
- **A misspelling after a splat cannot be caught end-to-end, and the criterion
  is left unticked because of it.** Reference collection drops everything
  written after a `*`: the splat case traverses the expression's source but not
  the part being selected, so `volume.*.destnation` reaches validation as
  `volume`, with the misspelling already gone. The checker itself handles it
  correctly, which its unit tests prove, but no real configuration can exercise
  that path. The dotted and bracket forms survive collection and *are* reported,
  so the criterion holds for those. Fixing this means widening the shared
  expression traversal that also feeds the dependency graph — which this plan
  explicitly places out of scope, since it would change what the graph acts on
  rather than only what validation sees.
- **Three separate causes of false rejection surfaced only by running against
  real fixtures**, none of them anticipated:
  1. A map key is indistinguishable in writing from a property name, so any
     segment standing against a collection must be treated as selecting a
     member. Before this, a typo after a map key was silently accepted.
  2. A reference to an output or variable names the *value it holds*, not the
     declaration's own fields. Checking the path against the declaration
     rejected valid references and broke nine tests, including the whole modules
     fixture.
  3. Dynamically typed values and interfaces stop the walk, both as plain fields
     and as a collection's element type.
- **The headline behaviour change of the whole plan landed here.** The test that
  asserted a property typo was an advisory warning, and that a configuration
  containing it was usable, now asserts it is a failure naming the property.
  That inversion is the defect this work exists to remove.

### 2026-09-18 — Phase 3.3: Stop checking where the type is no longer knowable

**What was done**: Delivered as part of the same walk as Phase 3.2 rather than
as a later pass. The walk stops and accepts the remainder of a path wherever the
type stops being knowable — a value whose shape is decided when the
configuration is evaluated, an interface, or a collection whose members are
scalars — and never rejects at such a boundary.

**Deviations**: Implemented alongside Phase 3.2 instead of after it. Keeping
them apart was not possible in any useful sense: without these rules the
previous phase rejected working configuration, so the two were written and
verified together.

**Files changed**: as Phase 3.2 above.

**Discoveries**:
- **Accepting at an unknowable boundary is not a concession, it is the point.**
  A false rejection breaks configuration that works today, while a missed typo
  merely leaves things as they were. Every stopping rule was added because a
  real fixture failed without it, not on principle.
- **The project's own fixture for this shape cannot be parsed end-to-end**, for
  reasons unrelated to validation: it calls a file-reading function against a
  file that does not exist in the repository, resolved relative to the process
  working directory. Its regression test therefore asserts through parse and
  validation rather than a full parse. Its only other consumer is one of the
  three packages that already fail to build, so the fixture is otherwise
  unexercised.

### 2026-09-18 — Phase 4.1: Remove the advisory/fatal classification

**What was done**: The advisory/fatal distinction is gone from the codebase
entirely — the severity field, both level constants, the level argument on every
error constructor, both copies of the classifier, and the two methods that
existed only to interrogate severity. With checking now exhaustive, a reported
problem simply is a failure, so callers test whether the collection is empty.

This phase also delivered the change deferred from Phase 1.2 by an explicit user
decision: validation no longer walks. The condition attached to that deferral —
that stages 2 and 3 exist first to replace what the walk provided — is now met,
so checking a configuration decodes nothing, builds no dependency graph and
reaches no provider. The spec's constraint that validation performs no
resolution work is satisfied.

**Deviations**: None. The deferral recorded in Phase 1.2 is closed here as
agreed, not carried further.

**Files changed**:
- `errors/parser_error.go`
- `errors/config_error.go`
- `errors/config_error_test.go`
- `internal/parser/parser.go`
- `internal/parser/callbacks.go`
- `internal/parser/dag.go`
- `internal/parser/util.go`
- `internal/parser/validate.go`
- `internal/parser/parse_test.go`
- `internal/parser/validate_test.go`
- `config.go`

**Discoveries**:
- **No rendered output changed, and there is a test that proves it.** The
  problem renderer never consulted the severity level, so an advisory problem
  already printed identically to a fatal one. The test asserting the exact
  rendered string passes byte-for-byte unchanged — the diff to that test file is
  sixteen deletions and no insertions, which is the evidence that only the two
  severity tests were removed and nothing about rendering shifted.
- **Removing the walk from validation broke nothing.** The two capabilities it
  provided are now covered elsewhere: malformed syntax and missing names are
  caught during parsing, before the gate, and reference and property faults are
  caught by stages 2 and 3. All the tests of the public validation entry point
  pass without a single assertion being weakened.
- **A second dead field went with the severity.** The error type carried a
  details field that was never read or written anywhere in the repository; it
  was removed alongside the level rather than left as the next reader's puzzle.
- **One test name is now misleading and was left for a decision.** A test called
  "reports malformed block as error not warning" asserts a distinction the
  codebase no longer draws; it now only checks that every problem surfaces as a
  parser error. Renaming it is cosmetic and was not done unilaterally.

### 2026-09-18 — Phase 4.2: Bring documentation and project knowledge in line

**What was done**: The project's recorded description of validation now matches
what validation does. The change-preview is gone from the documentation and from
the knowledge base, the error-only contract is described in its place, and the
three stages of checking are written down where a contributor would look for
them.

**Deviations**: One file was updated that the plan did not list — see
Discoveries.

**Files changed**:
- `docs/overview.md`
- `docs/README.md`
- `docs/parser-lifecycle.md`
- knowledge entry `architecture/ux-flow.md` (repo tier, store `xclconfig`)

**Discoveries**:
- **A fourth document was wrong and the plan did not list it.**
  `docs/parser-lifecycle.md` described the plugin-execution flag as the mode
  validation uses for "schema/DAG checking only", which was already inaccurate
  before this work — that flag only ever gated provider calls, never decoding —
  and is doubly wrong now that validation does not walk at all. Its numbered
  description of the parse flow was rewritten around the validation gate, and a
  section describing the three stages added.
- **The knowledge entry was updated through the knowledge workflow, with the
  user confirming the wording.** The plan recorded that the user had agreed the
  entry would be updated within this work, but not what it should say; the
  content was proposed and confirmed before being written, rather than the
  earlier agreement being treated as blanket approval.

### 2026-09-18 — Phase 4.3: Confirm nothing that worked before has broken

**What was done**: A closing audit against the pre-change baseline, separating
genuine regressions from the noise that was already there. **No regressions
were found.** Every one of the eight remaining failures was shown to be
pre-existing and failing for the same reason as before, the two failures this
work set out to fix now pass, and configurations valid before the change still
produce exactly the same resources.

**Deviations**: None.

**Files changed**: None. This phase verifies.

**Discoveries**:
- **Comparing against a pristine extracted baseline beat stashing.** With a
  deleted file and thirty-odd untracked paths in the tree, stashing risked the
  work. Extracting the base commit to a scratch directory gave two independently
  buildable copies to run the real suite against and diff, and left the working
  tree untouched throughout.
- **A test failing before and after is not automatically fine.** Each of the five
  tests the plan flagged as "must be decided" was checked for whether the
  *reason* had changed, not merely the outcome. Their bodies were extracted from
  both trees and compared byte for byte, and none touches the severity API this
  work removed. The three parser-event tests fail identically in both trees
  because the event callback never reaches the callback layer; the two rendering
  tests fail because they point at a fixture under an extension the repository
  no longer uses. All five: pre-existing and unchanged.
- **Behavioural equivalence was measured, not assumed.** The simple, modules and
  interpolation fixtures produce 9, 41 and 10 resources on both trees. Two of
  those counts are already asserted by existing tests that pass; the third had
  no assertion and was measured directly on both.
- **Formatting improved rather than regressed.** The baseline had two files
  failing gofmt; the current tree has one, and that one is untouched by this
  work. Static-analysis findings are identical to baseline — none in any file
  this work changed.
- **The masked ninth failure stayed masked and unchanged.** Enumerating the
  parser package past its early panic confirmed the nil-registry panic behaves
  identically to baseline, with nothing new hiding behind it.
- **The suite grew from 48 parser tests to 178**, all passing except the
  pre-existing failures above.
