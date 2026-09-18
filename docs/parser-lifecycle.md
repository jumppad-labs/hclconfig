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
5. Compare against previous state and call
   `Create`/`Refresh`+`Changed`+`Update` on the resource's provider.

```go
func (p *Parser) Validate(paths ...string) error
```

is the checking half on its own: it runs `parseAndValidate` and stops there,
so it never reaches steps 2-5. This is what `Config.Validate` calls.

### The validation gate

Validation sits between parsing and walking, and runs three stages in order —
**structure**, then **references**, then **properties**
([`internal/parser/validate.go`](../internal/parser/validate.go)):

- **structure** — resources too malformed to check further.
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

## `walkCallback` and `callProviderLifecycle`

The DAG walker (`github.com/silas/dag`) invokes one callback per vertex.
[`walkCallback`](../internal/parser/callbacks.go#L31) is that callback: it
decodes the resource's HCL body, handles module-specific evaluation-context
setup, and then calls
[`callProviderLifecycle`](../internal/parser/callbacks.go#L267).

`callProviderLifecycle` decides which provider methods to call based on
whether the resource existed in the previous state:

```
new resource (not in previous state)
    -> Create

existing resource
    -> Refresh (pull current provider-side state)
    -> Changed (compare previous vs. current)
    -> Update   (only if Changed returned true)
```

Builtin types with no provider (`variable`, `output`, `module`, the DAG
root) are skipped early — see "Instrumentation" below for a subtlety here.

Destroys are handled by a separate, simpler callback,
[`destroyWalkCallback`](../internal/parser/callbacks.go#L182), walked over
the DAG in *reverse* dependency order, calling `adapter.Destroy` directly
(no Refresh/Changed step — a destroy is unconditional).

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
`destroyWalkCallback`, and `callProviderLifecycle` only ever call this one
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
    Operation    string        // "create", "refresh", "changed", "update", "destroy"
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

Which operations fire depends on whether the resource exists in the previous
state ([`callbacks.go`](../internal/parser/callbacks.go)):

- **New resource** — `create` start, then `create` success or error.
- **Existing resource** — `refresh` start/success-or-error, then `changed`
  start/success-or-error, then, only if `Changed` reported a change, `update`
  start/success-or-error.
- **Removed resource** (destroy walk) — `destroy` start, then `destroy`
  success or error. `destroyWalkCallback` defines these events, but nothing
  runs the destroy walk yet, so today no `destroy` event is ever fired.

### Builtin types

`variable`, `output` and `module` resources have no provider. They fire a
single `create` success event (or `destroy` success on the destroy walk) with
zero duration, no error and no data, and no `start` event. This keeps every
visited resource in the stream, which matters if you consume events to
reconstruct processing order.

### Errors and control flow

Events never affect control flow. Whether they abort the walk is decided by
the operation, not by the event:

- `create`, `changed`, `update` and `destroy` errors abort the walk and are
  returned from `Apply`.
- A `refresh` error does **not** abort. The `error` event fires and the walk
  continues to `changed` using the unrefreshed resource.

### Who can subscribe

`Parser` lives under `internal/`, and `Config` has no option that sets
`OnParserEvent`, so today the stream is only reachable from inside this
module: tests use it to assert on provider calls and DAG-walk order.
