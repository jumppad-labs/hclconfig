---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Feature: 20260918075104-validation-phase-before-walk

## Overview

Configuration files are currently checked for correctness while they are being
applied, which means some genuine mistakes — a malformed block, or a reference to
a resource or property that doesn't exist — are reported as advisory warnings
rather than failures, and a configuration containing them can be treated as
usable. This introduces a distinct checking stage that runs to completion before
any configuration is acted upon, so that broken configuration is rejected
outright and every problem found is reported together, each identifying what is
wrong and where.

The people who benefit are anyone authoring configuration, who sees the full set
of mistakes at once and gets errors that name the actual problem instead of a
confusing downstream symptom, and anyone applying it, who can rely on a validity
check that is trustworthy on its own. The dry-run comparison that previously
reported what would change is withdrawn as part of this.

## Requirements

- [x] **A configuration that is not well-formed is rejected**
  Users receive a failure, not an advisory note, when a configuration contains a
  block or expression that is not well-formed.

- [x] **A reference to something defined nowhere is rejected**
  Users receive a failure when a reference names a resource, variable, output, or
  module that is not defined anywhere in the configuration.

- [x] **A reference to a property a type does not have is rejected**
  Users receive a failure when a reference names a property that the referenced
  item's type does not have, except where the path has already reached something
  whose type cannot be established before the configuration is applied.

- [x] **A reference that selects a member of a collection is accepted**
  Users find that picking a key from a map, a position in a collection, or every
  member of one at once is accepted, rather than reported as a problem because
  which member it will be is not yet known.

- [ ] **A property named after selecting a member is still checked** — *partially delivered: reported for the `x.0.prop` and `x[0].prop` forms; the splat form `x.*.prop` is not reached because reference collection drops everything after the `*`, a pre-existing blind spot the plan placed out of scope*
  Users receive a failure when a reference selects a member of a collection and
  then names a property that members of that collection do not have.

- [x] **A reference is accepted once its type is no longer knowable**
  Users find that the remainder of a reference is accepted, rather than reported
  as naming unknown properties, once the path reaches something whose type cannot
  be established before the configuration is applied.

- [x] **A reference between files resolves**
  Users can refer from one file to an item defined in another and have it
  accepted, and are told when no file defines it.

- [x] **A reference into a module resolves**
  Users can refer to an output of a module and have it accepted, and are told
  when the module does not declare it.

- [x] **A resource whose type cannot be checked is rejected**
  Users receive a failure, rather than silent acceptance, when a configuration
  contains a resource of a type that is not available to check against.

- [x] **Nothing is acted upon unless the configuration is valid**
  Users find that no resource has been created, changed, or removed when the
  configuration they applied turns out to be invalid.

- [x] **A configuration can be checked without acting on it**
  Users can ask whether a configuration is valid and receive either confirmation
  or the reasons it is not, with nothing created, changed, or removed either way.

- [x] **Every problem found at the same stage is reported together**
  Users see all the problems found at a given stage of checking in one go, rather
  than fixing one and rerunning to discover the next one like it.

- [x] **Each problem says what and where**
  Users are told, for each problem, which item could not be found or used and the
  file and position it appears at.

- [x] **A configuration free of the newly-rejected faults still works**
  Users find that a configuration accepted before this change, and containing
  none of the faults this work turns from advisory into failures, is still
  accepted and still produces the same resources when applied.

## Constraints

- **Validation must complete before any configuration is acted upon.**
  Acting on a configuration that has not been found valid is not permitted, so
  validation cannot be performed lazily or interleaved with execution.

- **Validation must be available on its own, and must also be applied
  automatically before acting on a configuration.**
  Both entry points are required: requesting validation alone, and validation
  running as a precondition of applying. Providing only one does not satisfy this.

- **Validation must treat a whole configuration as a single unit.**
  Files within a configuration may reference one another, so validating a file in
  isolation cannot establish that the configuration is valid.

- **Syntax errors must be fatal.**
  A configuration that is not well-formed must fail. The tolerance that exists for
  values which cannot be interpolated does not extend to syntax.

- **Attribute validation must tolerate path segments that select a member of a
  collection, without abandoning the rest of the path.**
  Selecting a key within a map, a position within a collection, or every member of
  one at once identifies *which* member is wanted, which is not knowable until the
  configuration is resolved. It does not obscure what *type* that member is.
  Validation must not reject a reference merely because it contains such a
  segment. Where the type of the selected member is known, validation must
  continue checking the properties named after it against that type; where it is
  not, validation must stop there and accept the remainder of the path.

- **Attribute validation must stop where the type of what is being referenced
  stops being known.**
  Properties populated only while applying, and anything reached through a value
  whose type cannot be established ahead of time, cannot be checked. Validation
  must accept these rather than reject them.

- **Resource types that cannot be verified must fail validation.**
  A type is unverifiable when the configuration names it but the system has no
  definition for it. Such a type cannot be accepted unchecked, as that would
  leave a hole in the guarantee that invalid configuration is never acted upon.

- **Checking proceeds in three stages: structure, then references, then
  properties.**
  Structure covers whether the configuration is well-formed, including whether
  every resource is of a type that can be checked and every module's contents can
  be obtained; references covers whether the things referred to exist; properties
  covers whether the things named on them exist. Each stage depends on the one
  before it having succeeded.

- **Every problem found within a stage must be reported, not only the first.**
  Where a stage finds several problems, all of them must be reported together.
  A stage is not required to run once an earlier stage has failed, so a
  configuration may need correcting and rechecking to reach the next stage — but
  no stage may stop at its own first problem.

- **Module references must be validated rather than deferred.**
  References into a module whose contents are available are subject to the same
  checks as references to items declared directly; deferring them until the
  configuration is acted upon is not permitted.

- **A module whose contents are unavailable must fail on the module itself, not
  on references into it.**
  Where a module's contents cannot be obtained — including because the module is
  held somewhere validation does not retrieve from — the failure must name the
  module, and references into it must not additionally be reported as naming
  things that do not exist.

- **Only modules held on the local filesystem are retrieved for checking.**
  Modules held elsewhere are treated as contents that cannot be obtained, and a
  declared module version has no bearing on what is retrieved.

- **What a valid configuration produces must not change, aside from the
  withdrawn dry-run comparison.**
  Applying a configuration that was already valid must still produce the same
  resources. This preservation covers the outcome of applying a configuration, not
  the shape of what is returned when checking one: the dry-run comparison is
  withdrawn, and how problems are classified and reported is expected to change.

- **Validation must not perform resolution work.**
  Establishing validity must not require resolving the configuration's values.
  This is what makes validation a distinct stage, and is the reason the dry-run
  comparison cannot be retained in its current form.

## Acceptance Criteria

- [x] **A malformed block is reported as a failure**
  Given a configuration file containing a block that is not well-formed,
  validating or applying it reports failure rather than success.

- [x] **A reference to an undefined resource is reported**
  Given a configuration referencing a resource that is defined nowhere,
  validating it fails and names the resource that could not be found.

- [x] **A reference to an undefined variable is reported**
  Given a configuration referencing a variable that is defined nowhere,
  validating it fails and names the variable that could not be found.

- [x] **A reference to an undefined output is reported**
  Given a configuration referencing an output that is defined nowhere, validating
  it fails and names the output that could not be found.

- [x] **A reference to an undefined module is reported**
  Given a configuration referencing a module that is defined nowhere, validating
  it fails and names the module that could not be found.

- [x] **A reference to a non-existent property is reported**
  Given a configuration referencing a property that does not exist on an
  otherwise valid resource, validating it fails and names that property.

- [x] **A reference through a map key is accepted**
  Given a configuration referencing a key within a map-valued property, and a
  further property beneath that key that members of the map do have, validating it
  succeeds rather than reporting the key as an unknown property.

- [x] **A reference through a collection position is accepted**
  Given a configuration referencing a position within a collection-valued
  property, or every member of one at once, followed by a property those members
  do have, validating it succeeds.

- [ ] **A bad property after a collection selection is reported** — *partially delivered: reported for the `x.0.prop` and `x[0].prop` forms; the splat form `x.*.prop` is not reached (see the matching requirement)*
  Given a configuration referencing every member of a collection at once, or one
  position or key within it, followed by a property those members do not have,
  validating it fails and names that property.

- [x] **A reference to a property populated during apply is accepted**
  Given a configuration referencing a property that is only populated while the
  configuration is being applied, validating it succeeds.

- [x] **A reference beneath a value of indeterminate type is accepted**
  Given a configuration naming properties beneath a value whose type cannot be
  established before applying, validating it succeeds and does not report those
  trailing properties as unknown.

- [x] **A collection whose member type is unknown does not cause rejection**
  Given a configuration selecting a member of a collection whose member type
  cannot be established, and naming a property beneath it, validating it
  succeeds.

- [x] **A cross-file reference resolves**
  Given a configuration split across multiple files where one file references an
  item defined in another, validating it succeeds.

- [x] **A reference to a module output resolves**
  Given a configuration referencing an output declared by a module, validating it
  succeeds; given one referencing an output that module does not declare,
  validating it fails and names that output.

- [x] **Nothing is acted upon when validation fails**
  Given an invalid configuration, applying it reports failure and no resource is
  created, updated, or destroyed.

- [x] **Validation alone acts on nothing**
  Given a valid configuration, requesting validation reports success and no
  resource is created, updated, or destroyed.

- [x] **An unverifiable resource type is reported**
  Given a configuration containing a resource of a type the system has no
  definition for, validating it fails rather than succeeding.

- [x] **Multiple faults of the same kind are all reported**
  Given a configuration containing several problems that are found at the same
  stage of checking — for example three references to items that do not exist,
  spread across more than one file — validating it reports every one of them, not
  only the first.

- [x] **Each reported problem carries a location**
  Given a configuration containing a problem, each problem reported states the
  file and the position within it where the problem occurs.

- [x] **A configuration free of the newly-rejected faults still applies**
  Given a configuration that was accepted before this change and contains none of
  the faults this work turns from advisory into failures, applying it succeeds and
  produces the same resources as it did previously.

- [x] **An unobtainable module is reported against the module**
  Given a configuration using a module whose contents cannot be obtained,
  validating it fails naming that module, and does not additionally report
  references into it as naming things that do not exist.

## Technical Approach

- The three stages build on one another, each needing the one before it to have
  succeeded before it has anything meaningful to say. Whether a later stage is
  worth attempting at all on a configuration that has already failed an earlier
  one is a judgement worth making per stage rather than globally.

- An interface presenting problems against the configuration is the intended
  consumer, so prefer keeping each problem individually addressable and located,
  over formatting for a terminal reader. A combined human-readable rendering is
  better derived from the individual problems than the other way round.

- Prefer deriving what a type's valid properties are from the type itself rather
  than by attempting to populate it, since the latter is what currently ties
  property checking to resolution.

- The reference extraction that already exists to build the dependency graph is a
  natural basis for the reference checks. Introducing a second way of finding
  references would risk the two disagreeing about what a configuration refers to,
  so reusing the existing one is preferred.

- A segment that selects a member of a collection is worth treating as a change
  of what the rest of the path is being checked against, rather than as a point
  to stop at — the member selected is unknown, but its type usually is not. The
  harder question is where the type genuinely does run out, which is what bounds
  the check.

- With validation exhaustive, the existing severity classification applied during
  resolution should no longer have anything to decide; removing it rather than
  fixing it in place is preferred, so that severity does not quietly continue to
  stand in for which stage a problem was found at.

- The main area of uncertainty is how far property checking can reach through
  module boundaries and provider-supplied types before it hits values that are
  only known once applied. Worth establishing early, as it bounds how much of the
  property check is achievable.

## Success Metrics

- Success is measured by the Acceptance Criteria above rather than by any
  separate quantitative or behavioural metric. This is a correctness change to a
  library with no external consumers yet, so there is no existing baseline to
  measure against and no telemetry in which a change would show up.

- Once there are consumers, a configuration error that a user reports as
  confusing or misleading is the signal to re-examine whether the checking stage
  is reporting problems at the point they can be understood.

## Non-Goals

- **The dry-run comparison is withdrawn, not reworked.** Reporting what would
  change requires resolving the configuration, so it cannot be produced by
  validation alone. It is removed rather than preserved through some other route,
  and may return in a later piece of work if it is wanted.

- **Retrieving module sources from anywhere but the local filesystem is not
  addressed.** This work does not add the ability to fetch a module held
  elsewhere, or to act on a declared module version, so such modules are handled
  as contents that cannot be obtained. Supporting them is separate work.

- **Editor-facing diagnostics are not part of this work.** Reporting problems
  individually and with their locations is intended to make such an interface
  possible, but building one — surfacing problems as a configuration is typed —
  is a separate piece of work.

- **No change to what is checked once a configuration is being applied.** This
  work moves checks earlier and makes their outcome decisive; it does not add new
  checks during application or change how resources are acted upon.

