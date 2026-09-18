---
created_date: "2026-09-18"
document_status: final
closed_date: "2026-09-18"
---

# Test Plan: 20260918075104-validation-phase-before-walk

## Success metrics

The plan's `## Testing Approach` states that success is measured by the spec's
21 acceptance criteria rather than by any separate quantitative metric, and
classifies **all 21 as behavioural tests, none manual**. That held: every
criterion is covered by an automated test in the suite, and no metric the plan
expected to be automatable proved otherwise.

**No manual procedure is required for any success metric.**

The spec's second metric — that a configuration error reported as confusing is
the signal to re-examine where problems surface — is forward-looking and has
nothing to assert today. It is recorded here so it is not mistaken for an
omission.

## Manual checks arising from implementation

Two items are not success metrics, but were discovered during implementation and
cannot be settled by the test suite. They are recorded so they are not lost.

### 1. The splat form of a post-selection typo is not reachable

**What to check.** That a misspelled property written after a splat —
`resource.container.x.volume.*.destnation` — is still not reported, and decide
whether that gap is worth closing.

**Why it cannot be automated.** The checker handles this case correctly, and its
unit tests in `internal/parser/properties_test.go` prove it. But reference
collection drops everything written after a `*`: `processExpr`'s `SplatExpr`
case in `internal/parser/exp.go` traverses the expression's `Source` and never
its `Each`, so the misspelling is gone before validation sees it. No
end-to-end test can exercise the path, which is why this is a judgement call
rather than an assertion.

**How.** Write a configuration containing
`value = resource.container.<name>.volume.*.destnation` against a real container
resource, and run `Config.Validate` over it. It will report nothing.

**Expected result.** No problem reported — this is the known limitation, not a
failure. The dotted and bracket forms (`volume.0.destnation`,
`volume[0].destnation`) *are* reported, and that difference is the thing to
confirm.

**Who / when.** Whoever picks up the follow-up work on reference collection.
Closing this means widening the shared expression traversal that also feeds the
dependency graph, which this plan placed out of scope because it would change
what the graph acts on rather than only what validation sees.

### 2. Problem reports are legible at block granularity

**What to check.** That a report naming several problems across more than one
file reads clearly enough to act on.

**Why it cannot be automated.** A test can assert that each problem carries a
file and a position; it cannot judge whether the result is usable. Positions
point at the resource block holding a reference, not at the reference within it,
because references are collected without their own source positions.

**How.** Validate `internal/test_fixtures/config/undefined_references_multi_file/`
and read the rendered output. Each problem prints its message, its
`file:line,column`, and a source excerpt around that line.

**Expected result.** Every problem names the resource, the thing that could not
be found, and its location; the offending reference is visible in the excerpt
beneath the highlighted block header. If it proves too coarse in real use,
raise it rather than expanding scope unilaterally — finer positions mean
changing the shared collection path described in item 1.

**Who / when.** Whoever builds the editor-facing interface the spec names as the
intended consumer.

## Pre-existing failures — not this work's

`go test ./...` is **not** green, and was not before this work began. Eight tests
fail and three test packages do not compile, all for reasons unrelated to
validation. Anyone verifying this work should compare against that baseline
rather than expecting a clean run; the Phase 4.3 changelog entry records the
audit that established each one as pre-existing and unchanged.
