---
created_date: "2026-09-18"
document_status: draft
project: xclconfig
spec: 20260918075104-validation-phase-before-walk
plan: 20260918075104-validation-phase-before-walk
---

# Configuration is validated before anything is acted upon

Broken configuration is now rejected outright instead of being reported as an
advisory note and treated as usable. You can ask whether a configuration is
valid and get a straight answer that acts on nothing, and applying one runs the
same check first — so nothing is created, changed or removed unless the whole
configuration is valid. When something is wrong you are told every problem at
once, each naming what is wrong and the file and position it occurs at, rather
than fixing one and rerunning to meet the next.

> Derived from project xclconfig (file), spec/plan
> 20260918075104-validation-phase-before-walk. See the project-level record for
> the full feature.

## What changed in this repo

**Checking is now a distinct stage.** Validation runs to completion between
parsing a configuration and acting on it, in three ordered stages — structure,
then references, then properties. Each reports everything it finds before the
next is considered, and a later stage is skipped when an earlier one found
problems, so you are not shown consequences of a fault you have already been
told about.

**What the stages catch:**

- A malformed block is a failure, not a warning. Every problem in a file is
  reported, and a single malformation producing several complaints reports all
  of them.
- A reference naming a resource, variable, output or module defined nowhere
  fails and names it. Resolution spans files and reaches into modules; a
  reference written inside a module resolves against that module first and the
  whole configuration second.
- A reference naming a property its target's type does not have fails and names
  the property. Picking a map key, a position, or every member of a collection
  at once is accepted and checking continues into the members, so a misspelling
  after such a selection is still caught. Checking stops and accepts the rest
  only where the type itself stops being knowable — a collection of scalars, a
  value whose shape is decided when the configuration is evaluated, or an
  output's held value.

**Two crashes and gaps closed.** A resource of a type the system has no
definition for now names that type. A module whose source directly or
indirectly includes itself is reported against the module instead of recursing
until the process runs out of stack.

**Breaking API changes.** These are real; the library is pre-consumer, which is
why they were taken.

- `Config.Validate(paths ...string)` returns `error` alone. It previously
  returned `(*Diff, error)`.
- The `Diff` type and its builder are **deleted**. Reporting what would change
  requires resolving the configuration, which is precisely what stopped
  validation being trustworthy on its own. It may return as separate work, on
  the resolving side of the new boundary.
- `ConfigError.ContainsErrors()` and `ContainsWarnings()` are **deleted** —
  every reported problem is now a failure, so test whether `Errors` is empty.
- `ParserError.Level` is **deleted**, along with both level constants and the
  `level` argument to `NewParserError` and `NewParserErrorFromResource`. A dead
  `Details` field went with them.

**No rendered output changed.** The renderer never consulted severity, so an
advisory problem already printed identically to a fatal one. Only the decision
about whether a configuration is usable changed. The test asserting the exact
rendered string passes unchanged, which is the evidence.

**One behaviour you may notice:** a configuration that previously applied with
warnings — a property typo, a dangling reference — is now rejected. That is the
intent of this change, not a regression.

## Why

Checking used to happen while a configuration was being applied, which meant it
could not be strict: a path that has to tolerate values it cannot yet resolve
cannot also hard-fail on them. Severity ended up standing in for which stage a
problem was found at, so genuine mistakes surfaced as advice. Splitting the
purposes means neither half has to hedge — validation answers yes or no, and
applying resolves without classifying. Reporting each problem individually with
its own location is also what makes an editor-facing interface possible later.
