# Plugin Architecture

A provider (plugin) is the thing that actually creates, updates, and
destroys whatever a resource represents (a container, a network, a cloud
resource, ...). This page covers how a provider is authored, how it's
exposed to the engine, and the two ways it can be hosted.

## The chain of interfaces

There are three distinct interfaces in play, each solving a different
problem. Understanding why there are three (not one) is the key to this
package:

```
ResourceProvider[T]   -- what a plugin author writes: typed, one Go struct T
        |             (plugins/provider.go)
        v
ProviderAdapter       -- uniform, untyped ([]byte in/out) contract
        |             (plugins/adapter.go)
        v
PluginHost            -- adds GetTypes()/Stop(), abstracts in-process vs gRPC
                      (plugins/plugin_host.go)
```

### 1. `ResourceProvider[T]` — what you write

A plugin author writes one of these per resource type, with `T` being their
own concrete Go struct (e.g. `*ContainerResource`):

```go
type ResourceProvider[T any] interface {
    Init(state State, functions ProviderFunctions, logger Logger) error
    Create(ctx context.Context, resource T) (T, error)
    Destroy(ctx context.Context, resource T, force bool) error
    Refresh(ctx context.Context, resource T) (T, error)
    Update(ctx context.Context, resource T) (T, error)
    Changed(ctx context.Context, old T, new T) (bool, error)
}
```

This is the ergonomic, type-safe surface — no manual JSON marshaling, no
`any`.

### 2. `ProviderAdapter` — the uniform contract

The rest of the engine (the parser's DAG walk, the plugin registry) can't
work with a different generic type per resource — it needs one interface it
can call regardless of which plugin or resource type it's dealing with:

```go
// plugins/adapter.go
type ProviderAdapter interface {
    Init(state State, functions ProviderFunctions, logger Logger) error
    Validate(ctx context.Context, entityData []byte) error
    Create(ctx context.Context, entityData []byte) ([]byte, error)
    Destroy(ctx context.Context, entityData []byte, force bool) error
    Refresh(ctx context.Context, entityData []byte) ([]byte, error)
    Update(ctx context.Context, entityData []byte) ([]byte, error)
    Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error)
}
```

Everything is `[]byte` (JSON) in and out. This is the interface
`internal/parser/callbacks.go` actually calls during the DAG walk (see
[Parser & Resource Lifecycle](parser-lifecycle.md)) — it never knows or
cares whether the concrete implementation is local or remote.

`TypedProviderAdapter[T]` ([`plugins/adapter.go`](../plugins/adapter.go))
is the bridge between the two: it wraps a `ResourceProvider[T]`, and each
method does `json.Unmarshal([]byte) -> T`, calls the typed provider, then
`json.Marshal(T) -> []byte`. A plugin author never constructs this
directly — `RegisterResourceProvider` does it for you (see below).

A second implementation, `GRPCResourceProviderAdapter`
([`plugins/grpc_resource_adapter.go`](../plugins/grpc_resource_adapter.go)),
satisfies the same interface but forwards each call over gRPC to a plugin
running in a separate process. From the parser's point of view these two
are indistinguishable — same interface, same call sites.

### 3. `PluginHost` — in-process vs. out-of-process

```go
// plugins/plugin_host.go
type PluginHost interface {
    GetTypes() []RegisteredType
    Validate(entityType, entitySubType string, entityData []byte) error
    Create(entityType, entitySubType string, entityData []byte) ([]byte, error)
    Destroy(entityType, entitySubType string, entityData []byte) error
    Refresh(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)
    Update(entityType, entitySubType string, entityData []byte) ([]byte, error)
    Changed(entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error)
    Stop()
}
```

Two implementations:

- **`DirectPluginHost`** ([`plugins/direct_plugin_host.go`](../plugins/direct_plugin_host.go)) —
  wraps a `Plugin` living in the same process. Every method is a direct
  passthrough call. Used for embedded/in-process plugins and in tests
  (see `internal/parser/test_plugin.go`).
- **`GRPCPluginHost`** ([`plugins/grpc_plugin_host.go`](../plugins/grpc_plugin_host.go)) —
  starts a plugin binary as a subprocess and talks to it over gRPC
  (`plugins/grpc_server.go` is what runs *inside* the plugin process).
  `GetTypes()` calls the remote `GetTypes` RPC once, then builds and caches
  one `GRPCResourceProviderAdapter` per returned type (`h.cachedTypes`,
  `h.typesCached`) — so the gRPC round-trip for type discovery happens
  once per host, not once per lifecycle call.

## Registering a provider

A plugin embeds `PluginBase` ([`plugins/plugin.go`](../plugins/plugin.go))
and, per resource type, calls:

```go
func RegisterResourceProvider[T any](
    p *PluginBase, logger Logger, state State,
    typeName, subTypeName string,
    resourceInstance T, provider ResourceProvider[T],
) error
```

This does three things:

1. Wraps `provider` in a `TypedProviderAdapter[T]` and calls `Init` on it.
2. Generates a JSON schema from `resourceInstance` via
   `schema.GenerateSchemaFromInstance` (reflects over the struct — see
   `internal/schema/serialize.go`).
3. Appends a `RegisteredType{Type, SubType, Schema, Adapter}` to
   `PluginBase.registeredTypes`.

`PluginBase.GetTypes()` just returns that slice — it's what both
`DirectPluginHost.GetTypes()` (directly) and `GRPCPluginHost.GetTypes()`
(via a `GetTypes` RPC call to the plugin process) expose upward.

The out-of-process case is why the schema exists at all: the host process
doesn't have `T` compiled in, so it can't decode `entityData` itself. When a
resource of a given type needs to be *instantiated* from HCL (not just
passed through as `[]byte`), the host reconstructs a Go type dynamically
from the schema via `schema.CreateInstanceFromSchema` — see
[`plugins/registry/plugin_registry.go:createResourceFromPlugins`](../plugins/registry/plugin_registry.go).

## `PluginRegistry` — tying it together

[`plugins/registry/plugin_registry.go`](../plugins/registry/plugin_registry.go)
holds a `[]plugins.PluginHost` (populated via `RegisterPlugin` for
in-process plugins, `RegisterPluginWithPath`/`DiscoverAndLoadPlugins` for
gRPC ones) plus the compiled-in builtin types. Its two jobs, used from two
different places in the parser:

- **`CreateResource(resourceType, resourceName) (any, error)`** — instantiate
  a new (empty) resource instance for an HCL block. Tries builtins first,
  then walks plugin hosts' `GetTypes()` looking for a schema match, and
  builds a dynamic instance via `schema.CreateInstanceFromSchema` if found.
  Used while *parsing* HCL, before any dependency graph exists.
- **`GetProviderForResource(resource any) plugins.ProviderAdapter`** —
  given an already-decoded resource, find the `ProviderAdapter` that
  handles its type (matches `types.GetMeta(resource).Type` against each
  host's `GetTypes()`). Used during the DAG walk to actually invoke
  lifecycle methods (see [Parser & Resource Lifecycle](parser-lifecycle.md)).

These are two different methods on the same struct because they solve two
different problems (build a Go value vs. look up an RPC target) — code that
only needs the second one can depend on the narrower
`parser.ProviderResolver` interface instead of the concrete
`*PluginRegistry` (see the "testing" note in [Parser & Resource
Lifecycle](parser-lifecycle.md)).

## Mocks

Both `plugins.ProviderAdapter` and `plugins.State` have generated
`testify`/`mockery` mocks under [`plugins/mocks/`](../plugins/mocks/)
(config: [`.mockery.yml`](../.mockery.yml)). `internal/parser.ProviderResolver`
(a narrow interface covering just `GetProviderForResource`) has its own
mock under [`internal/parser/mocks/`](../internal/parser/mocks/). Together
these let tests exercise the DAG-walk/lifecycle logic without a real plugin
process or a hand-written fake plugin.
