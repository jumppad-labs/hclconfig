# Parser & Resource Lifecycle

This is the engine that turns parsed HCL into a dependency-ordered sequence
of provider calls. It lives in `internal/parser/`.

## `Parser.Parse` — the single entry point

```go
func (p *Parser) Parse(executePlugins bool, paths ...string) (*state.State, error)
```

([`internal/parser/parser.go:182`](../internal/parser/parser.go#L182)) does,
in order:

1. Load previous state from `p.stateStore` (if configured and `Exists()`),
   for the Create-vs-Update comparison in step 6.
2. Parse all HCL files under `paths` into Go resource instances (via
   `PluginRegistry.CreateResource` for typing, `gohcl.DecodeBody` for
   values).
3. Build a DAG from resource dependencies (`internal/parser/dag.go`) —
   explicit `depends_on`, plus implicit edges from cross-resource
   references discovered during parsing (`Meta.Links`).
4. Walk the DAG in dependency order.
5. Decode each resource body once its dependencies' values are available.
6. If `executePlugins` is `true`: compare against previous state and call
   `Create`/`Refresh`+`Changed`+`Update` on the resource's provider.
   If `false`: skip all provider calls, tolerate missing interpolated
   values (this is the mode `Config.Validate` uses — schema/DAG checking
   only).

`Parser` is stateless across calls — `Config` constructs a new one for
every `Apply`/`Validate` (see [Overview](overview.md)).

## `walkCallback` and `callProviderLifecycle`

The DAG walker (`github.com/silas/dag`) invokes one callback per vertex.
[`walkCallback`](../internal/parser/callbacks.go#L31) is that callback: it
decodes the resource's HCL body, handles module-specific evaluation-context
setup, and — only if `executePlugins` is true — calls
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
Note `PluginRegistry.CreateResource` (used in step 2 of `Parse`, to
instantiate resources from HCL) is a *different* method not covered by
`ProviderResolver` — a real registry is still needed for that part even in
tests that mock the lifecycle-call path.

## Instrumentation: `ParserEvent`

[`internal/parser/events.go`](../internal/parser/events.go) defines a
lightweight, purely observational event stream:

```go
type ParserEvent struct {
    Operation    string        // "create", "destroy", "update", "refresh", "changed", "validate"
    ResourceType string
    ResourceID   string
    Phase        string        // "start", "success", "error"
    Duration     time.Duration
    Error        error
    Data         []byte
}
```

`fireParserEvent(options, ...)` is a nil-safe helper — a no-op unless
`ParserOptions.OnParserEvent` is set. Every provider call in both callbacks
is bracketed: fire `"start"`, invoke the adapter method with timing, fire
`"success"` or `"error"`. This never affects control flow — an adapter
error still propagates and aborts the walk regardless of whether an
`OnParserEvent` callback is configured.

Builtin types (`variable`/`output`/`module`/root) that are skipped in
`callProviderLifecycle` still fire a `"create"`/`"success"` event (0
duration, no error) before returning — this mirrors what
`destroyWalkCallback` already does on its skip path, and matters if you're
consuming `OnParserEvent` to reconstruct processing order: without it, a
resource like an `output` block would never appear in the event stream at
all, even though it was visited and "succeeded" in the trivial sense of
having nothing to do.

`Config` itself doesn't set `OnParserEvent` today — it's a hook for direct
`Parser` users and tests (metrics, logging, or — as in the ordering test
above — asserting DAG-walk order without depending on internal walk
state).
