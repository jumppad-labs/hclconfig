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
    Read(ctx context.Context, old T, new T) (T, error)
    Update(ctx context.Context, resource T) (T, error)
    Changed(ctx context.Context, old T, new T) (bool, error)
    Functions() ProviderFunctions
}
```

This is the ergonomic, type-safe surface — no manual JSON marshaling, no
`any`. What each method receives, may change and returns, and when xcl calls
it, is in the [Plugin Developer Guide](plugin-developer-guide.md). In short:

- `Read(ctx, old, new)` reports the real resource. `old` is the copy saved by
  the last apply, `new` is the configured copy; `Read` fills `new` in and
  returns it. It is only called for resources in the previous state.
- `Read` returns `plugins.ErrNotFound`
  ([`plugins/errors.go`](../plugins/errors.go)) when the real resource no
  longer exists, and xcl creates it again. xcl checks for it with
  `errors.Is`, so it can be wrapped.
- `Changed(ctx, old, new)` compares the saved copy with what `Read` returned.
  Embed `plugins.DefaultChanged[T]`
  ([`plugins/changed.go`](../plugins/changed.go)) to get a comparison of the
  JSON form of both copies that ignores `meta`, `depends_on` and `disabled`;
  define `Changed` on the provider to override it.

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
    Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error)
    Update(ctx context.Context, entityData []byte) ([]byte, error)
    Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error)
}
```

Everything is `[]byte` (JSON) in and out. This is the interface
`internal/parser/lifecycle.go` actually calls during the DAG walk (see
[Parser & Resource Lifecycle](parser-lifecycle.md)) — it never knows or
cares whether the concrete implementation is local or remote.

`TypedProviderAdapter[T]` ([`plugins/adapter.go`](../plugins/adapter.go))
is the bridge between the two: it wraps a `ResourceProvider[T]`, and each
method does `json.Unmarshal([]byte) -> T`, calls the typed provider, then
`json.Marshal(T) -> []byte`. A plugin author never constructs this
directly — `RegisterResourceProvider` does it for you (see below).
`TypedProviderAdapter.Read` returns the provider's error unwrapped, so
`ErrNotFound` reaches the parser intact.

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
    Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)
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

Error values don't survive gRPC: the plugin process sends an error back as a
string. So that `ErrNotFound` still means "not found" out of process,
`ReadResponse` in [`plugins/plugin.proto`](../plugins/plugin.proto) has a
`not_found` field. The plugin side (`GRPCServer.Read`) sets it when the
adapter's error `errors.Is` `ErrNotFound`, and the host side
(`grpcPluginWrapper.Read`) turns it back into an error wrapping
`ErrNotFound`. Every other error arrives at the host as a plain error with
the provider's message.

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

A plugin provides as many block types as it likes: call
`RegisterResourceProvider` once per type in `Init`, each with its own
provider. The adapter tags each provider's logger with its block type, so the
providers of one plugin are told apart in the log. Both plugins in
[`example/plugin`](../example/plugin) provide two types this way, the
in-process one `postgres` and `redis`, the external one `app` and `ingress`.

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
  then [configuration-only types](#configuration-only-types) (a real
  instance of the registered Go type), then walks plugin hosts' `GetTypes()`
  looking for a schema match, and builds a dynamic instance via
  `schema.CreateInstanceFromSchema` if found. Used while *parsing* HCL,
  before any dependency graph exists.
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

## Plugin logging

A plugin logs to the logger its host was given, with every message tagged so
it can be told apart from the host's own logs. Every line, from a plugin or
from the host, leads with the event it belongs to, `event=<name>`, taken from
an `event` argument; a message logged without one is written with
`event=log`. Log with an event, and the resource with `resource`:

```go
p.logger.Debug("", "event", "create", "resource", db.Meta.ID)
```
 The plugin host tags the
plugin's logger with `plugin=<name>`: the Go type name for an in-process
plugin (`ExamplePlugin`), the binary's file name for an external one. The
adapter that `RegisterResourceProvider` creates also tags the provider's
logger with `provider=<block type>`, inside the plugin process, so a provider
message reads

```
DEBU event=create plugin=ExamplePlugin provider=postgres resource=resource.postgres.main
```

The tags are written at the start of the message, after the event, by
`logger.WithTag`; `StdOutLogger` also moves the event to the front, so the
host's own lines read the same way.

An external plugin reaches the host through go-plugin, which logs how it
starts and talks to the plugin process, and passes on anything the process
writes to stderr. `GRPCPluginHost` gives go-plugin an adapter
([`plugins/hclog_adapter.go`](../plugins/hclog_adapter.go)) that writes all
of that through the same `plugin=<name>` tagged logger, as `event=go-plugin`. go-plugin's info and
debug messages are passed on at debug and its trace messages are dropped;
warnings and errors keep their level. go-plugin's `received EOF, stopping recv
loop` debug message is dropped as well: it reports the plugin's stdio stream
ending, which happens every time the plugin process stops, but carries an
`err=` field that reads like a failure. Everything an external plugin
produces therefore reaches the host app's logger in one format, rather than
partly through go-plugin's default logger straight to stderr:

```
DEBU event=go-plugin plugin=external starting plugin path=build/external args=[build/external]
DEBU event=create plugin=external provider=app resource=resource.app.web
```

Both kinds of plugin log the same framework messages, at debug: a
`plugin loaded` line (`event=load`) listing the block types when the host
loads the plugin, and a `calling provider` line before each provider call,
whose event is the provider call. The call line is
written by the adapter around the provider, which runs in the host for an
in-process plugin and inside the plugin process for an external one:

```
DEBU event=load plugin=ExamplePlugin plugin loaded block_types=postgres, redis
DEBU event=create plugin=ExamplePlugin provider=postgres calling provider resource=resource.postgres.main
DEBU event=load plugin=external plugin loaded block_types=app, ingress
DEBU event=create plugin=external provider=app calling provider resource=resource.app.web
```

## Configuration-only types

Not every block type needs a plugin. `PluginRegistry.RegisterType(name,
&MyType{})` registers a plain Go type (a pointer to a struct embedding
`types.ResourceBase`) under a block type name, with no plugin and no
provider:

```go
r := registry.NewPluginRegistry(log)
err := r.RegisterType("postgres", &PostgreSQL{})
```

`CreateResource` builds registered types with `reflect.New`, like builtins, so
blocks decode into the developer's own type and state reload returns that
type. The parser learns about them through the one-method
`parser.TypeRegistry` interface (`IsRegisteredType`), which `*PluginRegistry`
satisfies. The lifecycle and the destroy walk treat a registered type like a
builtin: it gets a success event and no provider is ever called for it. On
apply its status is left unchanged; on destroy it is removed from the state.

Type names are unique across the registry. `RegisterType`, `RegisterPlugin`,
`RegisterPluginWithPath` and `DiscoverAndLoadPlugins` all check each incoming
name against builtins, registered types and every loaded plugin's resource
types, and fail with a `*registry.TypeNameClashError` naming the type. A
clashing plugin is stopped and not added, and discovery always returns clash
errors, even when other plugins load.

[`example/configonly`](../example/configonly) parses a Kubernetes-like
configuration into registered types this way, with no plugin at all.
[`example/plugin`](../example/plugin) is the other half of the picture, four
block types provided by two plugins instead.

## Mocks

Both `plugins.ProviderAdapter` and `plugins.State` have generated
`testify`/`mockery` mocks under [`plugins/mocks/`](../plugins/mocks/)
(config: [`.mockery.yml`](../.mockery.yml)). `internal/parser.ProviderResolver`
(a narrow interface covering just `GetProviderForResource`) has its own
mock under [`internal/parser/mocks/`](../internal/parser/mocks/). Together
these let tests exercise the DAG-walk/lifecycle logic without a real plugin
process or a hand-written fake plugin.
