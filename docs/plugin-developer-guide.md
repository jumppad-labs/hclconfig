# Plugin Developer Guide

A provider owns the lifecycle of one resource type. xcl decides *when* to
call it; the provider decides *what* each call means for the real resource.
This guide is about writing the provider side correctly.

The reference implementation is the example provider in
[`plugins/example/pkg/person/provider.go`](../plugins/example/pkg/person/provider.go),
with its resource type in
[`resource.go`](../plugins/example/pkg/person/resource.go) next to it. For how
providers are hosted and registered, see [Plugin Architecture](plugins.md).

## The contract

A provider implements `plugins.ResourceProvider[T]`
([`plugins/provider.go`](../plugins/provider.go)), where `T` is a pointer to
your resource struct:

```go
type ResourceProvider[T any] interface {
    Init(state State, functions ProviderFunctions, logger Logger) error
    Create(ctx context.Context, resource T) (T, error)
    Read(ctx context.Context, old T, new T) (T, error)
    Changed(ctx context.Context, old T, new T) (bool, error)
    Update(ctx context.Context, resource T) (T, error)
    Destroy(ctx context.Context, resource T, force bool) error
    Functions() ProviderFunctions
}
```

`Init` is called once when the provider is registered. Use it to keep the
state, functions and logger it is given and to set up any clients.
`Functions` returns the functions the provider exposes to other providers.
The rest of this guide is about the lifecycle methods.

## The two copies of a resource

Every call on a resource that already exists works with two copies of it:

- **`old`**: the resource as saved to state by the last apply. It holds
  everything the provider returned last time, including identity (IDs
  assigned at create time) and observed values.
- **`new`**: the resource decoded from the current configuration. It holds
  what the user wrote, plus the computed values xcl carried over from `old`
  (see [Computed fields](#computed-fields)). Observed and derived values are
  empty until `Read` fills them in.

Keep these straight and the rest follows.

## Kinds of field

| Kind | Example | Who sets it | Where it comes from on the next apply |
|---|---|---|---|
| Config | `image`, `path`, `port` | the user, in HCL | the configuration |
| Identity | container ID, cloud ARN | the provider, in `Create` | carried over from `old` (a computed field) |
| Observed | `running`, `ip_address` | the provider, in `Create`/`Read`/`Update` | `Read` writes it into `new` |
| Derived | a file's `checksum` | the provider, in `Create`/`Read`/`Update` | `Read` writes it into `new` |

Identity fields are why `Read` gets `old`: you can't always find the real
resource from configuration alone. Any field the user must not set, identity
in particular, should be marked computed.

## The lifecycle

On every apply, xcl walks the resources in dependency order. For each
resource it looks up the resource's entry in the state saved by the last
apply and picks one of three paths
([`internal/parser/lifecycle.go`](../internal/parser/lifecycle.go)).

### Not in the previous state: create

```
Create(new)                    -> status created
```

`Read` is not called. Whatever `Create` returns is saved, and becomes `old`
on the next apply.

### Saved as `created` or `updated`: read, then update if changed

```
carry computed values from old onto new
result, err = Read(old, new)
    ErrNotFound -> reset new to the configured copy, Create(new) -> status created
    other error -> status failed, the apply fails
changed = Changed(old, result)
    true  -> Update(result)    -> status updated
    false -> keep result and the previous status
```

When nothing changed, what `Read` returned is saved and the status stays as
it was (`created` or `updated`).

When `Read` reports `ErrNotFound`, the computed values carried over from
`old` are dropped along with the real resource: `Create` gets the resource as
configured.

### Saved as `failed` or `destroy_failed`: rebuild

```
Destroy(old)
    error -> keep old, status destroy_failed, no Create
Create(new)                    -> status created
```

A resource whose last provider call failed is destroyed using its saved copy
and then created again, whether or not its configuration changed. This also
applies to any status xcl does not recognise.

If `Destroy` fails, the saved copy is kept, because it holds the identity
needed to try again, and the resource is saved as `destroy_failed`. `Create`
is not called. The next apply tries the rebuild again.

### Builtin types

`variable`, `output` and `module` resources have no provider. xcl handles
them itself and makes no provider calls for them.

### Removed resources

A resource that is in the previous state but no longer in the configuration
is **not** destroyed today. There is no removed-resource destroy yet,
`Config.Destroy` is a stub, and the destroy walk in the parser is never run.
The only time xcl calls `Destroy` is during a rebuild.

## Methods

### `Create(ctx, resource) (T, error)`

Called when the resource is not in the previous state, when `Read` returned
`ErrNotFound`, and after a rebuild's `Destroy`.

- **Input**: the resource as configured. Computed fields are empty.
- **May change**: computed, observed and derived fields.
- **Returns**: the resource with identity, observed and derived fields set.
  This is what is saved and what `old` will be next time.

### `Read(ctx, old, new) (T, error)`

Called only for resources saved as `created` or `updated`, so `old` is never
nil. Look up the real resource and fill in `new` from it.

- **Input**: `old` is the saved copy; use its identity fields to find the
  real resource. `new` is the configured copy with the saved computed values
  already carried over.
- **May change**: identity, observed and derived fields on `new`.
- **Returns**: `new`, filled in. This goes to `Changed`, then to `Update` if
  something changed, or is saved as is if nothing did.

The rules:

- Never change a configured field. If you overwrite `image` with what is
  actually running, the user's change is lost.
- Never record values that change on their own (see the callout below).
- Never change the real resource. `Read` only looks: it must not create,
  change or remove anything.
- Return `plugins.ErrNotFound` when the real resource no longer exists. xcl
  creates it again.
- Any other error fails the resource and the apply. xcl does not guess.

> [!IMPORTANT]
> **Never record values that change on their own in `Read`.** Uptime,
> last-seen timestamps, counters and anything else that moves without the
> resource changing will differ from the saved copy on every apply. `Changed`
> then reports a change and xcl calls `Update` on every apply, forever.
> Record only values that change when the resource really changes.

### `Changed(ctx, old, new) (bool, error)`

Decides whether the resource needs an `Update`. `old` is the saved copy and
`new` is what `Read` returned, so it holds both configuration edits and
drift in the real resource. It must not change anything.

Most providers should not write this. Embed `plugins.DefaultChanged[T]`
([`plugins/changed.go`](../plugins/changed.go)):

```go
type ContainerProvider struct {
    plugins.DefaultChanged[*Container]
    // ...
}
```

`DefaultChanged` compares the JSON form of both copies, ignoring xcl's own
resource metadata (`meta`, `depends_on` and `disabled`). Because `Read` has
put reality into `new`, one comparison catches both kinds of change:

| | `old` (last applied) | `new` (config + Read) | result |
|---|---|---|---|
| nothing changed | `running=true` | `running=true` | no update |
| container stopped | `running=true` | `running=false` | update |
| config changed | `image=a` | `image=b` | update |
| file contents changed | `checksum=x` | `checksum=y` | update |

Define `Changed` on your provider to override it, only when a plain
comparison is wrong for your type, for example two values that differ as
text but mean the same thing. If you find yourself computing things in
`Changed`, move that work into `Read`.

An error from `Changed` fails the resource and the apply.

### `Update(ctx, resource) (T, error)`

Called when `Changed` returned true.

- **Input**: the resource as `Read` returned it.
- **May change**: computed, observed and derived fields.
- **Returns**: the resource with observed and derived fields set to the new
  reality (`running=true` after a restart), so that the next `Read` sees no
  difference. This is what is saved.

### `Destroy(ctx, resource, force) error`

Removes the real resource. Today this is only called during a rebuild, with
the saved copy of a resource saved as `failed` or `destroy_failed`.

- **Input**: the saved copy. After a failed `Create`, the saved copy may hold
  no identity at all, because `Create` never returned one.
- **Returns**: only an error. `force` asks for a quick destroy that doesn't
  wait for graceful shutdown.

Whether destroying a resource that no longer exists is an error is your
decision. Any error you return fails the destroy, and the resource is saved
as `destroy_failed`, which keeps it from being created again. Treating
"already gone" as success is usually right, since a rebuild only needs the
resource to be gone.

## Computed fields

A computed field is owned by the provider. Mark it with the `computed`
option in an `xcl` struct tag, and make it optional in its `hcl` tag:

```go
PersonID string `xcl:"person_id,optional,computed" json:"person_id,omitempty"`
```

The option lives in its own `xcl` tag because `gohcl` rejects options it
does not know in the `hcl` tag.

What xcl does with computed fields
([`internal/parser/computed.go`](../internal/parser/computed.go)):

- **Users can't set them.** Setting a computed field in configuration is a
  validation error naming the resource and the field path, for example
  `resource 'resource.container.web' sets computed field 'network.assigned_address'`.
  It is reported before any provider is called.
- **They must be optional.** A computed field without `xcl:",optional"` is a
  validation error, since no configuration could satisfy it.
- **Saved values are carried over.** Before `Read`, xcl copies the computed
  values from the saved copy onto the configured copy. This works at any
  depth: nested blocks, pointer blocks, and lists and maps of blocks.
- **They survive unchanged applies.** Because the values are carried over,
  a provider whose `Read` adds nothing still keeps them, and `Changed` sees
  the same values on both copies.
- **Dependents can reference them.** Other resources can use a computed
  value, such as `resource.container.web.container_id`, and see the value
  the provider set.

### Pairing list elements with `key`

For a list of blocks, xcl has to decide which saved element goes with which
configured element. Mark the fields that identify an element with
`xcl:"key"`. Elements are paired by the values of their key fields; when the
element type has no key fields, they are paired by position. Map elements are
paired by map key. An element present on only one side gets nothing carried
over.

An example, a container with network attachments whose address is assigned
by the network:

```go
type Container struct {
    types.ResourceBase `xcl:",remain"`

    Image string `xcl:"image" json:"image"`

    // set by the provider in Create
    ContainerID string `xcl:"container_id,optional,computed" json:"container_id,omitempty"`

    Networks []NetworkAttachment `xcl:"network,block" json:"networks,omitempty"`
}

type NetworkAttachment struct {
    // identifies the attachment, so saved and configured elements pair up
    // even when the user reorders the blocks
    Name string `xcl:"name,key" json:"name"`

    // set by the provider when the container joins the network
    AssignedAddress string `xcl:"assigned_address,optional,computed" json:"assigned_address,omitempty"`
}
```

Without the key, reordering the `network` blocks would carry each address
onto the wrong attachment.

## Don't change configured values

`Create`, `Read` and `Update` must only change computed, observed and
derived fields. After each of these calls, xcl compares what went in with
what came back
([`internal/parser/configured_check.go`](../internal/parser/configured_check.go)).
For every non-computed field the provider changed, it logs a warning with
the message `provider changed a configured value` and the resource and field
as attributes (the layout depends on the logger):

```
provider changed a configured value  resource=resource.container.web  field=image
```

The apply continues. Fields whose configuration references another resource
or a module are not reported, since their value comes from elsewhere.

The warning points at a real problem. The changed value is what gets saved,
so on the next apply the saved copy no longer matches the configuration,
`Changed` reports a change, and xcl calls `Update`. A provider that rewrites
a configured value causes an update on every apply.

## Errors

### `ErrNotFound`

`plugins.ErrNotFound` ([`plugins/errors.go`](../plugins/errors.go)) is the
only error with a special meaning, and only from `Read`. xcl checks for it
with `errors.Is`, so you can wrap it:

```go
return nil, fmt.Errorf("container %s: %w", old.ContainerID, plugins.ErrNotFound)
```

It keeps its meaning when the provider runs in a separate plugin process.
Error values don't survive gRPC, so `ReadResponse` carries a `not_found`
field ([`plugins/plugin.proto`](../plugins/plugin.proto)) and the host turns
it back into `ErrNotFound`.

### Any other error

An error from any provider call marks the resource `failed` (or
`destroy_failed` for a rebuild's `Destroy`) and fails the apply. On the next
apply a failed resource is rebuilt: destroyed with its saved copy, then
created.

### What happens to the rest of the apply

- Resources that depend on a failed resource are skipped.
- Resources that don't depend on it still complete.
- The state is saved anyway, so the next apply picks up where this one
  stopped:
  - resources that were reached are saved with their new values and status;
  - the failing resource is saved as `failed` (or `destroy_failed`);
  - resources that existed before but were not reached keep their previous
    entry;
  - new resources that were not reached are left out.

`Config.Apply` saves this state and then returns the error. A configuration
that doesn't parse or validate saves nothing, since no provider was called.

## Statuses

xcl records a resource's status in `Meta.Status`
([`types/status.go`](../types/status.go)). These are the only values it sets:

| Status | Meaning |
|---|---|
| `created` | the provider created the resource |
| `updated` | the provider updated the resource |
| `failed` | a provider call for the resource failed; it is rebuilt on the next apply |
| `destroyed` | the provider destroyed the resource |
| `destroy_failed` | destroying the resource failed; the destroy is tried again on the next apply |

`Meta` belongs to xcl. Don't set it, and don't confuse `Meta.Status` with
your own observed fields, like a container's `running`.

## Ordering

Resources are processed in dependency order. By the time your provider is
called for a resource, everything it references has already been through its
own lifecycle, and the references hold the values those providers returned.

## Events

Each provider call fires a `start` event and then a `success` or `error`
event, with the operation name `create`, `read`, `changed`, `update` or
`destroy`. See
[Parser & Resource Lifecycle](parser-lifecycle.md#instrumentation-parserevent).

## The example provider

[`plugins/example/pkg/person/provider.go`](../plugins/example/pkg/person/provider.go)
follows everything in this guide:

- It embeds `plugins.DefaultChanged[*Person]` and defines no `Changed` of its
  own.
- `Person.PersonID` is a computed field, set in `Create`.
- `Read` returns `plugins.ErrNotFound` when the saved email is
  `missing@example.com`. This is a sentinel for demonstration; a real
  provider would look the resource up by its identity.
- It never changes a configured field.
