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
  does one Apply() call: load previous state -> parse HCL -> build DAG
  -> walk DAG, decoding each resource and calling its provider

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

err := cfg.Validate("./infra")         // checks only, acts on nothing
err := cfg.Apply("./infra")            // parses + executes provider lifecycle
```

[`config.go:23`](../config.go#L23) `NewConfig` applies functional options
([`options.go`](../options.go)) onto a `Config{currentState: state.NewState()}`.
With no options, you get a config that parses and validates HCL but never
touches a real provider or disk — useful for testing.

## What `Apply` actually does

[`config.go:95`](../config.go#L95):

1. Construct a `parser.Parser` for this call only, handing it `Config`'s
   `StateStore`, `PluginRegistry`, and variables via `ParserOptions`.
2. Call `p.Apply(paths...)`, see
   [Parser & Resource Lifecycle](parser-lifecycle.md).
3. Adopt the returned `*state.State` as `c.currentState`.
4. If a `StateStore` is configured, `Save` the new state.
5. Return the error from `p.Apply`, if any.

State is saved even when the apply failed. When a provider call fails,
`p.Apply` returns the state the walk reached along with the error: reached
resources with their new status, the failing resource as `failed` (or
`destroy_failed`), and the previous entry of resources that were not reached.
`Config.Apply` saves that state and then returns the error. Only when
`p.Apply` returns no state at all (the configuration didn't parse or
validate, or the dependency graph couldn't be built) is nothing saved. See
[State & Persistence](state.md#state-saved-after-a-failed-apply).

`Config.Validate` ([`config.go`](../config.go)) answers only whether a
configuration is valid, returning `error` alone. It calls `p.Validate(paths...)`,
which parses every file and then runs validation to completion — **without**
decoding bodies, walking the DAG or reaching a provider. A nil error means the
configuration is valid; otherwise the returned `*errors.ConfigError` collects
every problem found.

Validation runs three stages in order — structure, then references, then
properties — and each gathers all of its own findings before the next is
considered. A later stage is skipped when an earlier one found anything, because
checking properties on a reference that resolves nowhere would only report
consequences of a problem already reported.

`Config.Destroy` ([`config.go:130`](../config.go#L130)) is currently a stub
(`// TODO: Implement destroy logic`) — the destroy DAG-walk machinery exists
in the parser (`destroyWalkCallback`, see [Parser & Resource
Lifecycle](parser-lifecycle.md#destroy)) but nothing calls it yet. Resources
removed from the configuration are not destroyed either; the only `Destroy`
call an apply makes is when it rebuilds a resource saved as `failed` or
`destroy_failed`.

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
    Status                       string   // see below
}
```

`Status` is set by xcl, never by providers, and is one of `created`,
`updated`, `failed`, `destroyed` or `destroy_failed`
([`types/status.go`](../types/status.go)). The status saved by the last apply
decides what the next apply does with the resource: `created` and `updated`
resources are read and updated if they changed, `failed` and
`destroy_failed` resources are destroyed and created again. See
[State & Persistence](state.md#resource-statuses).

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
