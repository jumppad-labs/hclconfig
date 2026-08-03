# Overview

xclconfig parses HCL configuration describing resources, resolves them into a
dependency graph, and — for each resource — calls out to a *provider* (a
plugin) to actually create/update/destroy whatever the resource represents.
It is architecturally close to Terraform: HCL in, provider RPCs out, state
persisted in between runs.

## The three-layer split

```
Config            (repo root, package xcl)
  owns: PluginRegistry, StateStore, in-memory current State
  entry point: NewConfig(opts...), then Apply()/Validate()/Destroy()

Parser            (internal/parser)
  does one Parse() call: load previous state -> parse HCL -> build DAG
  -> walk DAG, decoding each resource and (if requested) calling its provider

PluginRegistry    (plugins/registry)
  aggregates PluginHosts, answers "what Go type is resource X" and
  "what ProviderAdapter handles resource X"
```

`Config` is the only piece meant to be constructed directly by a library
user. `Parser` is constructed fresh, internally, on every `Apply`/`Validate`
call — it is not held onto between calls. `PluginRegistry` and `StateStore`
are the two pieces of long-lived state `Config` owns and passes into each
new `Parser` via `ParserOptions`.

## Entry point

```go
cfg := xcl.NewConfig(
    xcl.WithPluginRegistry(pluginRegistry),
    xcl.WithStateStore(stateStore),
    xcl.WithVariables(vars),
)

diff, err := cfg.Validate("./infra")   // dry-run, no provider calls
err := cfg.Apply("./infra")            // parses + executes provider lifecycle
```

[`config.go:23`](../config.go#L23) `NewConfig` applies functional options
([`options.go`](../options.go)) onto a `Config{currentState: state.NewState()}`.
With no options, you get a config that parses and validates HCL but never
touches a real provider or disk — useful for testing.

## What `Apply` actually does

[`config.go:94`](../config.go#L94):

1. Construct a `parser.Parser` for this call only, handing it `Config`'s
   `StateStore`, `PluginRegistry`, and variables via `ParserOptions`.
2. Call `p.Parse(true, paths...)` — the `true` is `executePlugins`, see
   [Parser & Resource Lifecycle](parser-lifecycle.md).
3. Adopt the returned `*state.State` as `c.currentState`.
4. If a `StateStore` is configured, `Save` the new state.

`Config.Validate` ([`config.go:67`](../config.go#L67)) is almost identical
but calls `p.Parse(false, ...)` — decode and DAG-validate the HCL, skip
every provider call — then diffs the result against `c.currentState` via
`buildDiff` ([`diff.go`](../diff.go)) to produce a `*Diff{ToCreate, ToUpdate,
ToDestroy}`.

`Config.Destroy` ([`config.go:128`](../config.go#L128)) is currently a stub
(`// TODO: Implement destroy logic`) — the destroy DAG-walk machinery exists
in the parser (`destroyWalkCallback`, see [Parser & Resource
Lifecycle](parser-lifecycle.md)) but nothing in `Config` calls it yet.

## Resource metadata convention

Every resource type — builtin (`resources.Module`, `resources.Output`, ...)
or plugin-defined — embeds [`types.ResourceBase`](../types/resource.go#L50),
which in turn embeds [`types.Meta`](../types/resource.go#L5):

```go
type ResourceBase struct {
    DependsOn []string `hcl:"depends_on,optional"`
    Disabled  bool     `hcl:"disabled,optional"`
    Meta      Meta     `hcl:"meta,optional"`
}

type Meta struct {
    ID, Name, Type, Module, File string
    Line, Column                 int
    Properties                   map[string]any
    Links                        []string // unresolved cross-resource references
    Status                       string   // "pending" | "created" | "failed"
}
```

`types.GetMeta(resource any) (*Meta, error)` ([`types/resource_helpers.go`](../types/resource_helpers.go))
is the canonical way engine code reads this off an arbitrary resource value —
it walks embedded fields by reflection, so it works uniformly whether the
resource is a compiled-in Go struct or one dynamically built by
`schema.CreateInstanceFromSchema` (see [Plugin Architecture](plugins.md)).
This is what lets `internal/parser` and `plugins/registry` operate on
resources as `any` without knowing their concrete type.

## Where to go next

- Writing or hosting a provider: [Plugin Architecture](plugins.md)
- How HCL becomes provider calls, in what order: [Parser & Resource
  Lifecycle](parser-lifecycle.md)
- What gets persisted between runs: [State & Persistence](state.md)
- How `module` blocks are resolved: [Module System](modules.md)
