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
2. Build a DAG from resource dependencies (`internal/parser/dag.go`) —
   explicit `depends_on`, plus implicit edges from cross-resource
   references discovered during parsing (`Meta.Links`).
3. Walk the DAG in dependency order.
4. Decode each resource body (`gohcl.DecodeBody`) once its dependencies'
   values are available.
5. Look up the resource in the previous state and call
   `Create`, or `Read`+`Changed`+`Update`, or `Destroy`+`Create`, on the
   resource's provider.

If a provider call fails, `Apply` returns the state the walk reached
together with the error, see
[State saved after a failed apply](#state-saved-after-a-failed-apply).

```go
func (p *Parser) Validate(paths ...string) error
```

is the checking half on its own: it runs `parseAndValidate` and stops there,
so it never reaches steps 2-5. This is what `Config.Validate` calls.

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
every `Apply`/`Validate` (see [Overview](overview.md)).

## `walkCallback` and `resourceLifecycle`

The DAG walker (`internal/dag`, a copy of `github.com/silas/dag`) invokes one callback per vertex.
[`walkCallback`](../internal/parser/callbacks.go#L31) is that callback: it
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

The only `Destroy` call made today is the one in a rebuild, above.

Resources removed from the configuration are not destroyed.
[`destroyWalkCallback`](../internal/parser/callbacks.go#L177) is a separate
callback written to walk the DAG in *reverse* dependency order and call
`adapter.Destroy` directly, but nothing runs that walk, and `Config.Destroy`
is a stub.

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
- **Removed resource** — nothing. Resources removed from the configuration
  are not destroyed, and `destroyWalkCallback`, which also fires `destroy`
  events, is never run.

### Builtin types

`variable`, `output` and `module` resources have no provider. They fire a
single `create` success event (or `destroy` success on the unused destroy walk) with
zero duration, no error and no data, and no `start` event. This keeps every
visited resource in the stream, which matters if you consume events to
reconstruct processing order.

### Errors and control flow

Events never affect control flow. Every provider error is handled the same
way, whichever operation it came from:

- An error from `create`, `read`, `changed`, `update` or `destroy` marks the
  resource `failed` (`destroy_failed` for the rebuild's `destroy`), stops the
  walk from reaching the resources that depend on it, and is returned from
  `Apply`. Resources that don't depend on it still complete.
- The one exception is `plugins.ErrNotFound` from `read`: the `error` event
  fires, but the lifecycle creates the resource again instead of failing.

### Who can subscribe

`Parser` lives under `internal/`, and `Config` has no option that sets
`OnParserEvent`, so today the stream is only reachable from inside this
module: tests use it to assert on provider calls and DAG-walk order.

## State saved after a failed apply

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

`Config.Apply` ([`config.go`](../config.go)) saves this state and then
returns the error, so the next apply picks up where this one stopped. When
parsing or validation fails, or the dependency graph can't be built,
`Parser.Apply` returns a nil state and nothing is saved.
