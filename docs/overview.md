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
  does one Apply() call: load previous state -> parse HCL -> destroy
  removed resources -> build DAG -> walk DAG, decoding each resource and
  calling its provider
  or one Destroy() call: walk the saved state children first, calling
  each resource's provider

PluginRegistry    (plugins/registry)
  aggregates PluginHosts, answers "what Go type is resource X" and
  "what ProviderAdapter handles resource X"
```

`Config` is the only piece meant to be constructed directly by a library
user. `Parser` is constructed fresh, internally, on every
`Apply`/`Validate`/`Destroy` call — it is not held onto between calls.
`PluginRegistry` and `StateStore` are the two pieces of long-lived state
`Config` owns and passes into each new `Parser` via `ParserOptions`.

## Entry point

```go
cfg := xcl.NewConfig(
    xcl.WithPluginRegistry(pluginRegistry),
    xcl.WithStateStore(stateStore),
    xcl.WithVariables(vars),
)

err := cfg.Validate("./infra")         // checks only, acts on nothing
err := cfg.Apply("./infra")            // parses + executes provider lifecycle
err := cfg.Destroy()                   // destroys everything in the saved state
```

[`config.go:27`](../config.go#L27) `NewConfig` applies functional options
([`options.go`](../options.go)) onto a `Config{currentState: state.NewState()}`.
With no options, you get a config that parses and validates HCL but never
touches a real provider or disk — useful for testing.

## What `Apply` actually does

[`config.go:110`](../config.go#L110):

1. Construct a `parser.Parser` for this call only, handing it `Config`'s
   `StateStore`, `PluginRegistry`, and variables via `ParserOptions`.
2. Call `p.Apply(paths...)`, see
   [Parser & Resource Lifecycle](parser-lifecycle.md).
3. Adopt the returned `*state.State` as `c.currentState`.
4. If a `StateStore` is configured, `Save` the new state.
5. Return the error from `p.Apply`, if any.

Before anything is created or changed, `p.Apply` rejects a configuration
with no blocks (`xcl.ErrEmptyConfiguration`: use `Destroy` to remove
everything), then destroys the resources in the previous state that are no
longer in the configuration, children first, saving the state after each one.

State is saved even when the apply failed. When a provider call fails,
`p.Apply` returns the state the walk reached along with the error: reached
resources with their new status, the failing resource as `failed` (or
`destroy_failed`), and the previous entry of resources that were not reached.
When destroying a removed resource fails, nothing is created or changed, and
the state returned is the previous state minus what was destroyed, with the
failures as `destroy_failed`; the next apply retries them first.
`Config.Apply` saves that state and then returns the error. Only when
`p.Apply` returns no state at all (the configuration didn't parse or
validate, declared no blocks, or the dependency graph couldn't be built) is
nothing saved. See
[State & Persistence](state.md#state-saved-after-a-failed-apply).

`Config.Validate` ([`config.go:77`](../config.go#L77)) answers only whether a
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

## What `Destroy` does

[`config.go:155`](../config.go#L155) needs no configuration:

1. Load the saved state from the `StateStore` (or use the in-memory state
   when there is none). Nothing saved, or an empty state, returns nil and
   writes nothing; a load error is returned as `failed to load state: ...`.
2. Construct a `parser.Parser` and call `p.Destroy(saved)`, which destroys
   every resource children first, using the parents each resource recorded
   in `meta.parents` when it was applied. Unrelated resources are destroyed
   in parallel. Variables, outputs, modules, registered types and disabled
   blocks never reach a provider.
3. The state is saved after every resource, so an interrupted destroy
   resumes from it. A resource whose destroy fails stays as
   `destroy_failed`, together with everything it depends on, and is named in
   the returned error; calling `Destroy` again retries it.
4. Adopt what is left as `c.currentState`.

See [Parser & Resource Lifecycle](parser-lifecycle.md#destroy) and
[State & Persistence](state.md#state-during-a-destroy).

## Resource metadata convention

Every resource type — builtin (`resources.Module`, `resources.Output`, ...)
or plugin-defined — embeds [`types.ResourceBase`](../types/resource.go#L58),
which in turn embeds [`types.Meta`](../types/resource.go#L5):

```go
type ResourceBase struct {
    DependsOn []string `xcl:"depends_on,optional"`
    Disabled  bool     `xcl:"disabled,optional"`
    Meta      Meta     `xcl:"meta,optional"`
}

type Meta struct {
    ID, Name, Type, Module, File string
    Line, Column                 int
    Properties                   map[string]any
    Links                        []string // unresolved cross-resource references
    Parents                      []string // resolved dependencies, orders a destroy
    Status                       string   // see below
}
```

`Status` is set by xcl, never by providers, and is one of `created`,
`updated`, `failed`, `destroyed` or `destroy_failed`
([`types/status.go`](../types/status.go)). The status saved by the last apply
decides what the next apply does with the resource: `created` and `updated`
resources are read and updated if they changed, `failed` and
`destroy_failed` resources are destroyed and created again. `destroyed` is
never saved: a destroyed resource leaves the state. See
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
