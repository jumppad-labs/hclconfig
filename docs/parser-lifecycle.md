# Parser & Resource Lifecycle

This is the engine that turns parsed HCL into a dependency-ordered sequence
of provider calls. It lives in `internal/parser/`.

## `Parser.Apply` — the single entry point

```go
func (p *Parser) Apply(paths ...string) (*state.State, error)
```

([`internal/parser/parser.go`](../internal/parser/parser.go)) does,
in order:

1. `parseAndValidate` — load previous state, parse all HCL files under
   `paths` into Go resource instances (via `PluginRegistry.CreateResource`
   for typing), then **validate the configuration as a whole**. Nothing is
   acted upon unless validation passes, and validation itself decodes no
   bodies and reaches no provider.
2. Reject a configuration that declares no blocks with
   [`ErrEmptyConfiguration`](../internal/parser/errors.go) ("the configuration
   declares no blocks, use Destroy to remove everything"). Applying it would
   remove everything, so nothing is destroyed, created, changed or saved.
3. Destroy the resources that are in the previous state but no longer in the
   configuration ([`removedResources`](../internal/parser/parser.go#L332)),
   children first, before anything is created or changed. This uses the same
   destroyer as `Destroy`, see [Destroy](#destroy).
4. Build a DAG from resource dependencies (`internal/parser/dag.go`) —
   explicit `depends_on`, plus implicit edges from cross-resource
   references discovered during parsing (`Meta.Links`). The parents each
   resource ends up with are recorded in `Meta.Parents`.
5. Walk the DAG in dependency order.
6. Decode each resource body (`gohcl.DecodeBody`) once its dependencies'
   values are available.
7. Look up the resource in the previous state and call
   `Create`, or `Read`+`Changed`+`Update`, or `Destroy`+`Create`, on the
   resource's provider.

If a removal in step 3 fails, `Apply` stops there; if a provider call fails
in the walk, `Apply` returns the state the walk reached. Either way the error
comes back with a state to save, see
[State saved after a failed apply](#state-saved-after-a-failed-apply). An
apply that removes nothing skips step 3 entirely.

```go
func (p *Parser) Validate(paths ...string) error
```

is the checking half on its own: it runs `parseAndValidate` and stops there,
so it never reaches steps 2-7. This is what `Config.Validate` calls.

### The validation gate

Validation sits between parsing and walking, and runs three stages in order —
**structure**, then **references**, then **properties**
([`internal/parser/validate.go`](../internal/parser/validate.go)):

- **structure** — resources too malformed to check further. This is also
  where computed fields are checked: a computed field that is not optional,
  and a computed field set in configuration, are both reported here (see the
  [Plugin Developer Guide](plugin-developer-guide.md#computed-fields)).
- **references** — everything a resource refers to must be defined somewhere
  in the configuration ([`references.go`](../internal/parser/references.go)).
  A reference is resolved against its referring resource's module scope first
  and the whole configuration second.
- **properties** — a reference's trailing property path must name properties
  the target's type actually has
  ([`properties.go`](../internal/parser/properties.go)). Selecting a member of
  a collection continues the walk against the member's type; the walk stops
  and accepts the remainder only where the type itself stops being knowable.

Each stage gathers every problem it finds before returning, and a later stage
is skipped when an earlier one found anything.

`Parser` is stateless across calls — `Config` constructs a new one for
every `Apply`/`Validate`/`Destroy` (see [Overview](overview.md)).

## `walkCallback` and `resourceLifecycle`

The DAG walker (`internal/dag`, a copy of `github.com/silas/dag`) invokes one callback per vertex.
[`walkCallback`](../internal/parser/callbacks.go#L37) is that callback: it
decodes the resource's HCL body, handles module-specific evaluation-context
setup, and then hands the resource to `resourceLifecycle.apply`
([`internal/parser/lifecycle.go`](../internal/parser/lifecycle.go)).

`resourceLifecycle` picks the provider calls from the resource's entry in the
previous state, the state saved by the last apply:

```
not in previous state
    -> Create                                  status created

saved as created or updated
    -> carry computed values from the saved copy onto the configured copy
    -> Read(saved, configured)
         ErrNotFound -> reset to the configured copy, Create   status created
         other error -> status failed, the apply fails
    -> Changed(saved, read result)
         true  -> Update(read result)          status updated
         false -> keep the read result and the previous status

saved as failed, destroy_failed, or anything else (rebuild)
    -> Destroy(saved copy)
         error -> keep the saved copy          status destroy_failed
    -> Create(configured copy)                 status created
```

A rebuild happens whether or not the resource's configuration changed. When
the rebuild's `Destroy` fails, `Create` is not called; the saved copy is kept
because it holds the identity needed to try the destroy again on the next
apply.

After `Create`, `Read` and `Update`, the lifecycle compares the resource it
sent with what the provider returned and logs a warning for every configured
(non-computed) value the provider changed
([`configured_check.go`](../internal/parser/configured_check.go)). The
warning never fails the apply. The
[Plugin Developer Guide](plugin-developer-guide.md) describes what providers
may change in each call.

Builtin types with no provider (`variable`, `output`, `module`, the DAG
root) are skipped early — see "Instrumentation" below for a subtlety here.

### Destroy

```go
func (p *Parser) Destroy(saved *state.State) (*state.State, error)
```

([`internal/parser/parser.go`](../internal/parser/parser.go#L301)) destroys
every resource in `saved` and needs no configuration. `Config.Destroy`
([`config.go`](../config.go#L155)) passes it the state loaded from the
`StateStore` (or the in-memory state when there is no store); when nothing
has been saved, or the state is empty, it returns nil without calling the
parser and writes nothing. A load failure is returned as
`failed to load state: ...`.

The work is done by the
[`destroyer`](../internal/parser/destroy.go#L22), which the removal phase of
`Apply` uses too:

- [`buildDestroyDAG`](../internal/parser/dag.go#L124) builds a graph with the
  same shape as the create graph, from each resource's recorded
  `Meta.Parents`: an edge from each parent in the set to the resource, and
  resources with no parent in the set hang off a root. Parents that are not
  being destroyed are ignored.
- The graph is walked with `dag.Walker{Reverse: true}`, so every child is
  destroyed before its parents and unrelated resources are destroyed in
  parallel. A parent is never visited once one of its children has failed.
- [`destroyWalkCallback`](../internal/parser/callbacks.go#L188) handles one
  resource. `variable`, `output`, `module`, registered (config-only) types and
  disabled blocks never reach a provider: they fire a `destroy` success event
  and are removed. Every other resource is passed to its provider's `Destroy`
  as the saved copy, with `force` always false.
- After each resource the working state is updated and saved through the
  `StateStore`: a destroyed resource is removed, a failed one is kept and
  marked `destroy_failed`. The saved state is correct at every step, so an
  interrupted destroy resumes from it. A failed save fails the step, so that
  resource's parents are not visited either.

When a destroy fails, the failed resource and everything it depends on (its
parents, transitively) stay in the state, while unrelated resources are still
destroyed. The error names every failed resource (`destroy failed for <id>:
...`), and calling `Destroy` again retries what is left. `Parser.Destroy`
returns the remaining state, never nil, and `Config.Destroy` adopts it as its
current state.

Resources saved before `Meta.Parents` was recorded have no parents, so they
are destroyed with no ordering guarantee between them.

## Resolving the provider: `ProviderResolver`

Both callbacks need to turn a resource into a `plugins.ProviderAdapter`.
Rather than depending on the concrete `*registry.PluginRegistry`, they
depend on a narrow interface defined in this package:

```go
// internal/parser/callbacks.go
type ProviderResolver interface {
    GetProviderForResource(resource any) plugins.ProviderAdapter
}
```

`*registry.PluginRegistry` satisfies this structurally (Go interfaces are
implicit), so production code is unaffected — `ParserOptions.PluginRegistry`
is still what gets passed in practice. But `walkCallback`,
`destroyWalkCallback`, and `resourceLifecycle` only ever call this one
method, so tests can substitute
[`internal/parser/mocks.MockProviderResolver`](../internal/parser/mocks/mock_provider_resolver.go)
instead of standing up a real registry + real (or fake) plugin.

`ParserOptions` exposes this as its own field, defaulting to
`PluginRegistry` when unset:

```go
// internal/parser/parser.go
ProviderResolver ProviderResolver // overrides provider lookup; defaults to PluginRegistry
```

This is what `TestParserProcessesResourcesInCorrectOrder` uses to verify
DAG-walk ordering with a single mock adapter/resolver pair instead of the
hand-written `TestPlugin` fake ([`internal/parser/test_plugin.go`](../internal/parser/test_plugin.go)).
Note `PluginRegistry.CreateResource` (used in step 1 of `Apply`, to
instantiate resources from HCL) is a *different* method not covered by
`ProviderResolver` — a real registry is still needed for that part even in
tests that mock the lifecycle-call path.

## Instrumentation: `ParserEvent`

[`internal/parser/events.go`](../internal/parser/events.go) defines a
lightweight, purely observational event stream. Set
`ParserOptions.OnParserEvent` and the parser calls it once per event, in the
order the DAG walk produces them. `fireParserEvent(options, ...)` is a
nil-safe helper — a no-op when no callback is set.

```go
type ParserEvent struct {
    Operation    string        // "create", "read", "changed", "update", "destroy"
    ResourceType string        // "<type>.<name>", e.g. "container.base"
    ResourceID   string        // full resource ID, e.g. "resource.container.base"
    Phase        string        // "start", "success", "error"
    Duration     time.Duration // time spent in the provider call; 0 for "start"
    Error        error         // the provider's error; only set for "error"
    Data         []byte        // the resource serialized to JSON; nil for builtin types
}
```

### Operation and phase

`Operation` names the provider method being called. `Phase` says where in
that call the event was fired:

| Phase | Fired | `Duration` | `Error` |
|---|---|---|---|
| `start` | immediately before the provider method is called | 0 | nil |
| `success` | after the method returns without error | time the call took | nil |
| `error` | after the method returns an error | time the call took | the provider's error |

Every provider call is bracketed: one `start`, then exactly one of `success`
or `error`.

### Event sequences per resource

Which operations fire depends on the resource's entry in the previous state
([`lifecycle.go`](../internal/parser/lifecycle.go)):

- **New resource** — `create` start, then `create` success or error.
- **Existing resource** (saved as `created` or `updated`) — `read`
  start/success-or-error, then `changed` start/success-or-error, then, only
  if `Changed` reported a change, `update` start/success-or-error. When
  `Read` returns `ErrNotFound`, the `read` error event is followed by
  `create` start/success-or-error instead.
- **Rebuilt resource** (saved as `failed` or `destroy_failed`) — `destroy`
  start/success-or-error, then, if the destroy succeeded, `create`
  start/success-or-error.
- **Removed resource** (in the previous state, no longer configured) —
  `destroy` start, then `destroy` success or error, before any other
  resource is processed.
- **Destroyed resource** (`Config.Destroy`) — `destroy` start, then
  `destroy` success or error.

### Builtin types

`variable`, `output` and `module` resources have no provider. They fire a
single `create` success event on apply, or `destroy` success when destroyed,
with zero duration, no error and no data, and no `start` event. Registered
(config-only) types and disabled blocks do the same on destroy. This keeps
every visited resource in the stream, which matters if you consume events to
reconstruct processing order.

### Errors and control flow

Events never affect control flow. Every provider error is handled the same
way, whichever operation it came from:

- An error from `create`, `read`, `changed`, `update` or `destroy` marks the
  resource `failed` (`destroy_failed` for any `destroy`), stops the
  walk from reaching the resources that depend on it (on a destroy walk, the
  resources it depends on), and is returned from `Apply` or `Destroy`.
  Resources that don't depend on it still complete.
- The one exception is `plugins.ErrNotFound` from `read`: the `error` event
  fires, but the lifecycle creates the resource again instead of failing.

### Who can subscribe

`Parser` lives under `internal/`, but `Config` forwards the stream to the
handler set with `xcl.WithEventHandler` ([`events.go`](../events.go)) during
`Apply`, `Destroy` and, for parse events, `Validate`. Tests inside this module
also set `OnParserEvent` directly to assert on provider calls and DAG-walk
order.

## State saved after a failed apply

When destroying a removed resource fails, nothing is created or changed.
`Parser.Apply` returns the previous state minus the resources that were
destroyed, with the failed ones (and, as with `Destroy`, everything they
depend on) still in it, the failures marked `destroy_failed`. The removal has
already saved this after each resource, and `Config.Apply` saves it again
before returning the error. The next apply retries the removal first.

When the walk fails, `Parser.Apply` still returns a state, built by
`applyProgress.buildState`
([`internal/parser/progress.go`](../internal/parser/progress.go)), together
with the error:

- resources the walk reached are included as they are now, with their new
  values and status;
- the failing resource is included as `failed`, or `destroy_failed` if the
  rebuild's destroy failed;
- resources that existed in the previous state but were not reached keep
  their previous entry;
- new resources that were not reached are left out.

A resource whose error came before any provider call (a decode error, or no
provider for its type) is treated as not reached.

`Config.Apply` ([`config.go`](../config.go#L110)) saves this state and then
returns the error, so the next apply picks up where this one stopped. When
parsing or validation fails, the configuration declares no blocks, or the
dependency graph can't be built, `Parser.Apply` returns a nil state and
nothing is saved.
