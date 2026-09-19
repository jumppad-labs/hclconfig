# State & Persistence

## `State` — the in-memory resource registry

[`state/state.go`](../state/state.go) is deliberately simple: a
mutex-protected flat list.

```go
type State struct {
    resources []any
    mu        sync.Mutex
}
```

There's no separate index or status table — a resource's operational
status lives on its own `types.Meta.Status` field (see
[Resource statuses](#resource-statuses)), so `State` itself just needs to
store and find resources.

Each saved resource also records its parents in `meta.parents`
([`types/resource.go`](../types/resource.go#L42)): the sorted IDs of the
resources it depends on, resolved when the create graph is built. They cover
explicit `depends_on`, references (including to variables and outputs),
module-wide references expanded to the module's resources, and the module the
resource sits in. The field is internal, cannot be set from configuration, and
is omitted when empty. It is what lets `Destroy` order resources from the
saved state alone; state saved before it existed has no parents, and is
destroyed with no ordering guarantee.

Every lookup (`FindResource`, `FindResourcesByType`, `FindModuleResources`)
is a **linear scan** comparing `types.GetMeta(r)` fields against a parsed
FQRN (fully-qualified resource name, `internal/resources/fqrn.go`) — there
is no index, which is fine at the scale this is used (one config's worth of
resources) but worth knowing if you're tempted to call these in a hot loop.

Key operations:

- **`AppendResource(r)`** ([`state.go:35`](../state/state.go#L35)) — computes
  the resource's FQRN from `Module`/`Name`/`Type` in its `Meta`, sets
  `Meta.ID` to that FQRN string, and errors with `ResourceExistsError` if a
  resource with the same FQRN is already present.
- **`FindResource(path)`** ([`state.go:81`](../state/state.go#L81)) — parses
  `path` as an FQRN and scans for a match.
- **`FindRelativeResource(path, parentModule)`** ([`state.go:89`](../state/state.go#L89)) —
  same, but prefixes `path`'s module with `parentModule` first; used when
  resolving a reference from inside a module to something in its own scope.
- **`FindModuleResources(module, includeSubModules)`** ([`state.go:132`](../state/state.go#L132)) —
  used by the parser to cascade `disabled` status onto everything inside a
  disabled module (`includeSubModules=true` matches by module-path prefix).
- **`Bytes()`** ([`state.go:178`](../state/state.go#L178)) — the
  serialization boundary: `json.MarshalIndent(s.resources, "", "  ")`. This
  is what any `StateStore.Save` implementation is expected to persist.

## Resource statuses

xcl records what happened to each resource in `Meta.Status`
([`types/status.go`](../types/status.go)). These are the only values it
sets:

| Status | Meaning | Next apply |
|---|---|---|
| `created` | the provider created the resource | read, then updated if changed |
| `updated` | the provider updated the resource | read, then updated if changed |
| `failed` | a provider call for the resource failed | rebuilt: destroyed, then created |
| `destroyed` | the provider destroyed the resource | — (never saved) |
| `destroy_failed` | destroying the resource failed | removed again if its block is gone, otherwise rebuilt: the destroy is tried again, then created |

`destroyed` is never saved: a destroyed resource is removed from the state
instead. A `destroy_failed` resource is retried by the next `Destroy`, or by
the next `Apply` as above.

## State saved after a failed apply

A failed apply still produces a state to save. `Parser.Apply` returns it
together with the error, and `Config.Apply` saves it before returning the
error, so the next apply picks up where this one stopped:

- resources the walk reached are saved with their new values and status;
- the failing resource is saved as `failed`, or `destroy_failed` if a
  rebuild's destroy failed;
- resources that existed before but were not reached keep their previous
  entry;
- new resources that were not reached are left out.

When a removed resource fails to be destroyed, the apply stops before
anything is created or changed, and the state saved is the previous state
minus what was destroyed, with the failures kept as `destroy_failed` (see
[State during a destroy](#state-during-a-destroy)). The next apply retries
the removal first.

Nothing is saved when the configuration doesn't parse or validate, declares
no blocks (`xcl.ErrEmptyConfiguration`), or the dependency graph can't be
built: no provider was called, so the previous state still stands. See
[Parser & Resource Lifecycle](parser-lifecycle.md#state-saved-after-a-failed-apply)
for how the state is built.

## State during a destroy

`Config.Destroy` and the removal phase of `Config.Apply` save the state
through the `StateStore` after every resource they destroy, not once at the
end ([`internal/parser/destroy.go`](../internal/parser/destroy.go#L22)):

- a destroyed resource is removed from the state;
- a resource whose destroy failed is kept, as the saved copy, with status
  `destroy_failed`;
- the resources it depends on are never reached, so they stay as they were.
  Unrelated resources are still destroyed.

The saved state is therefore correct at every step. If a destroy is
interrupted, or returns an error, running `Destroy` again picks up with what
is left. `Destroy` with no saved state, or an empty one, returns nil and
writes nothing.

## `StateStore` — the persistence contract

```go
// state/state_store.go
type StateStore interface {
    Load() (*State, error) // nil if no state exists (first run)
    Save(state *State) error
    Exists() bool
    Clear() error
}
```

`Parser.Apply` (and `Parser.Validate`) call `Exists()`/`Load()` at the
start of every run to get the "previous state", the state saved by the last
apply. `Config.Destroy` calls them too, to get the state to destroy.
`Parser.Apply` uses each resource's entry in it to decide between
create, read-then-update and rebuild (see
[Parser & Resource Lifecycle](parser-lifecycle.md)). `Config.Apply` calls
`Save()` after adopting the returned state, including after a failed apply
(see [State saved after a failed apply](#state-saved-after-a-failed-apply)).
Destroying, in `Config.Destroy` or an apply's removal phase, calls `Save()`
after every resource (see [State during a destroy](#state-during-a-destroy)).

`state/mocks/mock_state_store.go` is a generated mock of this interface
(same mockery setup as the plugin mocks — see [Plugin
Architecture](plugins.md)). Tests must stub `Exists()` even when it's
expected to return `false` — `Parser.Apply` and `Parser.Validate` call it
unconditionally (`parseAndValidate`,
[`internal/parser/parser.go:375`](../internal/parser/parser.go#L375)), so a
bare mock with no expectation set panics on the first call.

## `FileStateStore` — the on-disk implementation

[`state/file_state_store.go`](../state/file_state_store.go) is the only
`StateStore` implementation in this repo. Three things worth knowing:

**Loading requires a `*registry.PluginRegistry`.** State is persisted as a
flat JSON array of resources with no compiled-in type information on the
Go side, so `Load()` ([`file_state_store.go:33`](../state/file_state_store.go#L33))
does a two-phase decode:

1. Unmarshal the top-level array into `[]*json.RawMessage` — defers
   decoding each resource, preserving its raw JSON shape.
2. For each raw message, peek at `meta.type`/`meta.name` via an untyped
   `map[string]any` decode, call `registry.CreateResource(type, name)` to
   get a correctly-typed *empty* instance, then re-marshal/unmarshal the
   raw JSON into that instance.

**A type that is not registered fails the load.** When a saved entry's type
can't be created by the registry (e.g. a plugin that's no longer loaded, or
a type that hasn't been registered yet), `Load()` returns
[`state.UnknownTypesError`](../state/errors.go#L30) naming every such type,
sorted and unique, instead of dropping the entries — a state returned
without them would be saved without them, erasing resources that still
exist. This affects `Apply` and `Destroy` alike (`Destroy` wraps it as
`failed to load state: ...`), so register every type and plugin before
loading state. Entries that are malformed, or missing `meta`, `meta.type` or
`meta.name`, are still skipped (`continue`).

**`Save` is not atomic.** ([`file_state_store.go:136`](../state/file_state_store.go#L136))
It removes the existing file, then writes the new one — not a
write-to-temp-then-rename. A crash between the remove and the write would
lose the state file. Worth keeping in mind if this is ever hardened for
production use.

`NewFileStateStore(path, registry)` creates an empty state file
automatically if `path` doesn't exist yet (`createStateAtPath`,
[`file_state_store.go:158`](../state/file_state_store.go#L158)) — callers
don't need to special-case "first run."
