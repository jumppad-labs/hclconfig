---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-19"
---

# Feature: 20260918165700-provider-lifecycle-read

<!-- OVERVIEW -->
## Overview

When xcl applies a configuration a second time, it should notice what has really changed — either because the user edited the configuration, or because the real resource drifted (a container stopped, a file's contents changed) — and update only those resources. Today every apply recreates everything, failures lose track of resources that were already built, and plugin authors have no written contract for how to report a resource's real state. This makes repeat applies safe and predictable for xcl users, lets failed resources be cleanly rebuilt on the next run, and gives plugin authors one clear, documented job — read the real resource — with change detection handled for them by default.

<!-- REQUIREMENTS -->
## Requirements

- [x] **Previous state drives each apply**
  The system must decide for every resource whether it is new, existing or previously failed, using the state saved by the last apply.
- [x] **New resources are created**
  The system must create a resource that isn't in the previous state, and must not ask the provider to read it first.
- [x] **Existing resources are read before comparison**
  The system must ask the provider to read the real resource before deciding whether it changed.
- [x] **Read never changes the real resource**
  Reading must not create, change or remove anything in the real world.
- [x] **Missing resources are recreated**
  When the provider reports that a previously saved resource no longer exists, the system must create it again.
- [x] **Read failures fail the apply**
  Any other read failure marks the resource `failed` and makes the apply return an error.
- [x] **A failure skips only dependent resources**
  When a provider call fails, resources that depend on the failed one aren't processed. Independent resources still complete and are saved.
- [x] **Computed fields are provider-owned**
  Resource types can declare fields as computed. They're optional, the provider may set them or leave them unset, and users can't set them.
- [x] **Computed fields are rejected in configuration**
  Validation reports an error naming the resource and field when a user sets a computed field, before any provider is called.
- [x] **Computed values carry over**
  For an existing resource, the computed values saved in state are on the configured resource before it's read. Unchanged resources keep them, and dependent resources can reference them, even when the provider's read adds nothing.
- [x] **Warn when a provider changes a configured value**
  After a create, read or update, the system logs a warning naming the resource and field if the provider returned a different value for a non-computed field. Fields set by reference to another resource are exempt, and the apply continues.
- [x] **Change detection sees reality**
  The system must decide whether a resource changed by comparing the previously saved resource with the current configuration plus what the read found, so that both configuration edits and drift in the real resource are detected.
- [x] **Default change detection**
  Providers get working change detection without writing any. Differences only in xcl's own metadata never count as a change.
- [x] **Change detection can be overridden**
  Providers can replace the default change detection for their resource type.
- [x] **Changed resources are updated**
  The system must update a resource when change detection reports a difference, and must leave it alone when it doesn't.
- [x] **Unchanged resources keep what was read and their status**
  The saved state for an unchanged resource reflects what the read found, and keeps its previous status.
- [x] **Failed resources are rebuilt**
  A resource marked `failed` in the previous state must be destroyed and then created on the next apply, whether or not its configuration changed.
- [x] **Failed rebuild whose destroy fails**
  If destroying a failed resource fails during a rebuild, it's saved as `destroy_failed` and not created. The next apply tries destroy then create again.
- [x] **Failed applies keep their progress**
  When an apply fails, the system must save state: resources that succeeded with their new status and values, the resource that failed as `failed`, and previously existing resources that weren't reached with their previous entry. New resources that were never reached aren't saved.
- [x] **One set of statuses**
  Every resource status the system records must be one of `created`, `updated`, `failed`, `destroyed` or `destroy_failed`, and the documentation must list exactly these.
- [x] **Events name the read**
  Parser events for reading a resource must use the operation name `read`.
- [x] **Plugin developer guide**
  A plugin developer guide must describe the lifecycle, what each provider operation receives, may change and must return, and the rules for reading, including that destroying a missing resource is the provider's decision. It covers computed fields, explains that changing a non-computed value produces a warning and an update on the next apply, and gives prominence to the rule against recording values that change on their own.

<!-- CONSTRAINTS -->
## Constraints

- **No compatibility with existing plugins.** The provider interface and the plugin wire protocol may change in breaking ways. The old refresh operation is removed, not kept alongside read. *(User: "No, break it". This is the v2 branch.)*
- **No compatibility with previously saved state.** State saved by earlier versions doesn't have to load. *(User: "Not required".)*
- **Refresh becomes `Read`, and it gets both the saved and the configured resource.** The provider's refresh operation is renamed `Read` and must receive the previously saved resource and the resource from the current configuration: `Read(ctx, old, new)`. Rejected: a read that sees only the configuration, because the configuration alone can't locate resources whose identity is assigned at create. *(User: "I was thinking Refresh should be Read" / "I think option 1 is correct".)*
- **Computed fields are marked with a tag.** A resource type declares its provider-owned fields with a `computed` tag on the field. *(User: "we need computed as a tag".)*
- **The system does not special-case destroying a missing resource.** Whether that counts as success is up to the provider. The system treats any destroy error as a failure. *(User: "Provider decides".)*
- **Project testing rules.** Tests use testify `require`, no table-driven tests, and positive and negative cases in separate test functions. *(Project CLAUDE.md.)*

<!-- ACCEPTANCE CRITERIA -->
## Acceptance Criteria

- [x] **Second apply of unchanged config does nothing**
  Applying the same configuration twice against a resource the provider reports as unchanged results in exactly one create on the first apply. The second apply performs a read, no create and no update, and the resource's saved status is still `created`.
- [x] **First apply never reads**
  On an apply with no saved state, the provider receives a create for each resource and no read calls.
- [x] **Read receives both copies**
  On the second apply, the read call for a resource receives the previously saved resource, including values set by the provider at create such as an ID, and the resource as described by the current configuration.
- [x] **Configuration edit triggers update**
  Changing a configuration value between two applies results in an update for that resource on the second apply, and the saved state shows the new value with status `updated`.
- [x] **Drift triggers update**
  When configuration is unchanged but the read reports a different observed value from the one saved (for example, a container reported as not running), the second apply performs an update for that resource.
- [x] **Changed configured value warns**
  A provider that changes a literally configured, non-computed field produces a warning naming the resource and field, and the apply succeeds.
- [x] **Computed and reference-set fields don't warn**
  A provider that sets a computed field produces no warning, and neither does a field set by reference to another resource that differs.
- [x] **Setting a computed field fails validation**
  Config that sets a computed field fails validation with an error naming the resource and field, and the provider receives no calls.
- [x] **Computed values survive an unchanged apply**
  With a provider that has no custom read and uses default change detection, a second apply of unchanged config performs no update. A dependent resource that references a computed value still sees the value from the first apply.
- [x] **Missing resource is recreated**
  When the read reports that the resource no longer exists, the apply performs a create for it and no update, and the saved status is `created`.
- [x] **Read failure fails the apply**
  When the read returns any other error, the apply returns an error naming the resource, performs no update for it, and the saved status for that resource is `failed`.
- [x] **Failure skips only dependents**
  When one resource's create fails, a resource that depends on it receives no calls, an independent resource is created and saved, and new resources never reached aren't in the saved state.
- [x] **Default change detection works without provider code**
  A provider that doesn't define its own change detection gets no update when nothing differs, and an update when a configured or read value differs. Differences only in xcl's metadata, such as source file and line, don't cause an update.
- [x] **Overridden change detection is used**
  A provider that defines its own change detection has that used instead of the default. For example, one that always reports "unchanged" gets no update even when values differ.
- [x] **Unchanged resource saves what was read**
  After an apply where nothing changed, the saved state for the resource contains the values the read returned.
- [x] **Unchanged resource keeps its status**
  A resource saved as `updated` that is unchanged on the next apply is still `updated`.
- [x] **Failed resource is destroyed then created**
  A resource saved as `failed` receives a destroy followed by a create on the next apply, in that order, even with no configuration change. Afterwards the saved status is `created`.
- [x] **Rebuild with failing destroy**
  A failed resource whose destroy fails is saved as `destroy_failed`, receives no create, and receives destroy then create on the next apply.
- [x] **Failed apply saves progress**
  When the second of two dependent resources fails to create, the apply returns an error. The saved state contains the first resource as `created` and the second as `failed`, and the next apply doesn't create the first resource again.
- [x] **Only the agreed statuses appear**
  Across all of the scenarios above, every saved status is one of `created`, `updated`, `failed`, `destroyed` or `destroy_failed`, and the resource-status documentation lists exactly these five.
- [x] **Read events**
  With an event callback set, reading a resource produces events with operation `read` and phases `start` then `success` or `error`. No event has operation `refresh`.
- [x] **Guide covers the contract**
  The plugin developer guide describes each provider operation's inputs, what it may change and what it returns. It states the read rules (don't record values that change on their own, don't change the real resource, report missing resources), how computed fields work, the warning and next-apply update caused by changing a non-computed value, how failed resources are handled, and that destroying a missing resource is up to the provider.
- [ ] **Test suite passes**
  All tests pass, with no lifecycle tests skipped.

<!-- TECHNICAL APPROACH -->
## Technical Approach

- Signal a missing resource with an exported "not found" error that providers return from `Read`, so the parser can tell "gone, create it" apart from a real failure.
- Supply the default change detection as something providers embed in their type, since Go interfaces can't have optional methods. Defining `Changed` yourself replaces it.
- Make the default a flat comparison of the whole resource, ignoring xcl's metadata. This works because computed values are carried over and `Read` has put reality into the configured copy, so one comparison catches both configuration edits and drift.
- Implement `computed` as a struct tag, checked by the existing validation step.
- Copy computed values from the saved resource onto the configured one before `Read`.
- Tell "set by reference" apart from "set literally" using what the parser already knows about each attribute's expression.
- Pass the state loaded at the start of an apply through to the lifecycle calls. Today it's discarded before the walk.
- Build the state for a failed apply from the walk's progress, so a failure no longer throws away what was done.
- Remove the unused duplicate lifecycle code in the parser, and update the example plugin so its read no longer rewrites a configured field.
- Start the plugin developer guide from the rough draft already written, and update it as the design is built.
- **Risk:** the default flat comparison reports a change every time if a provider's `Read` records values that change on their own.

<!-- SUCCESS METRICS -->
## Success Metrics

- A second apply of unchanged configuration makes zero create and zero update calls to providers. Today every resource is created again.
- A failed apply followed by a successful one never creates a resource that the failed apply had already created.
- The example plugin needs no `Changed` of its own and relies on the default change detection, which shows the default is enough for a typical provider.

<!-- NON-GOALS -->
## Non-Goals

- Resources removed from the configuration are not destroyed by apply; the only destroy added here is the one that rebuilds a failed resource.
- A plan or diff preview of what an apply would change, before it runs. (User: "we can add a Diff later".)
- Letting users of the public `Config` API subscribe to parser events. Events stay available only inside the module.
- A separate command or operation that only checks for drift. Reading happens only as part of an apply.
- Automatically retrying a failed provider call within the same apply. Failed resources are dealt with on the *next* apply.
