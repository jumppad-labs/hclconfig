---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Validation runs to completion before a configuration is acted upon

## What was built

Configuration is now checked in a distinct stage that runs to completion between
reading it and acting on it. Previously the two were one continuous pass:
problems were found while a configuration was being applied, genuine mistakes
were reported as advisory notes, and a configuration containing them could still
be treated as usable. Now a configuration is either valid or it is rejected, and
nothing is created, changed or removed unless it is valid.

The check runs three stages in order, each reporting everything it finds before
the next is considered:

- **Structure** — configuration too malformed to check any further. A file's
  problems are now all reported together instead of only the first, and a single
  malformation that produces several related complaints reports all of them.
- **References** — anything a resource names must be defined somewhere in the
  configuration. Resolution spans files and reaches into modules, and every
  unresolvable reference in the whole configuration is reported at once.
- **Properties** — a reference's trailing property path must name properties the
  referenced type actually has. Selecting a key, a position, or every member of a
  collection at once is accepted and checking continues into the members, so a
  misspelling written after such a selection is still caught. Checking stops and
  accepts the remainder only where the type itself stops being knowable.

A configuration can now be checked on its own: asking whether it is valid gives
either confirmation or the reasons it is not, and acts on nothing either way —
it decodes nothing, builds no dependency graph and reaches no provider. Applying
a configuration runs the same check first and refuses to proceed if it fails.

Two safety gaps were closed along the way. A resource whose type the system has
no definition for now names that type in the failure rather than reporting a
confusing downstream symptom. A module whose source directly or indirectly
includes itself is reported against the module instead of recursing until the
process runs out of stack.

The advisory/fatal classification is gone entirely. With checking exhaustive it
had nothing left to decide, and leaving it would have let severity quietly
continue standing in for which stage a problem was found at. Every reported
problem is a failure, and every one carries the file and position it occurs at.

The change-preview that the validation entry point used to return is withdrawn.
Producing it required resolving the configuration, which is exactly what stopped
validation being a check that could be trusted on its own.

## Why it matters

Anyone authoring configuration sees the full set of mistakes at once instead of
fixing one and rerunning to meet the next, and gets errors naming the actual
problem rather than a confusing consequence of it. Anyone applying configuration
gets a validity check that is trustworthy on its own: a hard stop before
anything is touched, rather than discovering the problem partway through. Because
each problem is reported individually with its own location, the output is usable
by an interface that shows problems against the configuration as it is written.

## Deviations from the plan

Four, each recorded in the plan's phase-by-phase changelog with its reasoning.

1. **Validation kept the dependency walk until the final milestone.** The plan
   had validation stop before the walk immediately. Investigation showed that
   would lose two checks nothing yet replaced — detection of cycles longer than
   two resources, and schema errors raised while decoding — leaving validation
   weaker than before the work until the property stage existed. Notably the
   existing tests would not have caught the cycle regression, since only the
   short-cycle case was covered. The user chose to keep the walk and remove it
   once the reference and property stages could replace what it provided, which
   is what happened; the constraint is satisfied in the finished work.

2. **Reference resolution is scoped to the referring resource's module.** Not
   anticipated by the plan, and load-bearing: references written inside a module
   are unqualified while the things they name are module-scoped. Without this,
   six perfectly valid references in the project's own module fixture were
   reported as undefined — an implementation that would have passed every
   negative test while breaking every module configuration.

3. **One acceptance criterion is only partially met, and its checkbox is left
   unticked.** A misspelling written after a splat cannot be caught end to end,
   because reference collection drops everything after the `*` before validation
   sees it. The checker handles the case correctly and its unit tests prove it,
   and the equivalent dotted and bracket forms *are* reported. Closing the gap
   means widening a traversal shared with the dependency graph, which the plan
   placed out of scope. Recorded rather than quietly ticked.

4. **The module-reference suppression rule was not needed.** The plan expected a
   filter to stop references into an unobtainable module being reported
   alongside the module itself. It turned out to be structural: such a module is
   reported while parsing, so the configuration never reaches the reference
   stage. Writing the filter would have been dead code.

The library's public surface changed in ways worth knowing about: the validation
entry point returns an error alone, the change-preview type and its builder are
deleted, and the two methods that asked whether reported problems were advisory
are gone along with the severity field itself. No rendered output changed —
the renderer never consulted severity, and the test asserting its exact output
passes unchanged.
