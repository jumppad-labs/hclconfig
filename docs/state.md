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
| `destroyed` | the provider destroyed the resource | — |
| `destroy_failed` | destroying the resource failed | rebuilt: the destroy is tried again, then created |

`destroyed` is only set by the destroy walk, which nothing runs yet (see
[Parser & Resource Lifecycle](parser-lifecycle.md#destroy)).

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

Nothing is saved when the configuration doesn't parse or validate, or the
dependency graph can't be built: no provider was called, so the previous
state still stands. See
[Parser & Resource Lifecycle](parser-lifecycle.md#state-saved-after-a-failed-apply)
for how the state is built.

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
apply. `Parser.Apply` uses each resource's entry in it to decide between
create, read-then-update and rebuild (see
[Parser & Resource Lifecycle](parser-lifecycle.md)). `Config.Apply` calls
`Save()` after adopting the returned state, including after a failed apply
(see [State saved after a failed apply](#state-saved-after-a-failed-apply)).

`state/mocks/mock_state_store.go` is a generated mock of this interface
(same mockery setup as the plugin mocks — see [Plugin
Architecture](plugins.md)). Tests must stub `Exists()` even when it's
expected to return `false` — `Parser.Apply` and `Parser.Validate` call it
unconditionally (`parseAndValidate`,
[`internal/parser/parser.go:255`](../internal/parser/parser.go#L255)), so a
bare mock with no expectation set panics on the first call.

## `FileStateStore` — the on-disk implementation

[`state/file_state_store.go`](../state/file_state_store.go) is the only
`StateStore` implementation in this repo. Two things worth knowing:

**Loading requires a `*registry.PluginRegistry`.** State is persisted as a
flat JSON array of resources with no compiled-in type information on the
Go side, so `Load()` ([`file_state_store.go:32`](../state/file_state_store.go#L32))
does a two-phase decode:

1. Unmarshal the top-level array into `[]*json.RawMessage` — defers
   decoding each resource, preserving its raw JSON shape.
2. For each raw message, peek at `meta.type`/`meta.name` via an untyped
   `map[string]any` decode, call `registry.CreateResource(type, name)` to
   get a correctly-typed *empty* instance, then re-marshal/unmarshal the
   raw JSON into that instance.

Any entry that's malformed, missing `meta`, or references a type not
currently registered (e.g. a plugin that's no longer loaded) is silently
skipped (`continue`) rather than failing the whole load — a partial/stale
state file degrades gracefully instead of blocking every future run.

**`Save` is not atomic.** ([`file_state_store.go:124`](../state/file_state_store.go#L124))
It removes the existing file, then writes the new one — not a
write-to-temp-then-rename. A crash between the remove and the write would
lose the state file. Worth keeping in mind if this is ever hardened for
production use.

`NewFileStateStore(path, registry)` creates an empty state file
automatically if `path` doesn't exist yet (`createStateAtPath`,
[`file_state_store.go:146`](../state/file_state_store.go#L146)) — callers
don't need to special-case "first run."
