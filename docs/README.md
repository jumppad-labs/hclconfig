# xclconfig Architecture Wiki

Internal architecture documentation for xclconfig — an HCL-based configuration
and provisioning engine (conceptually similar to Terraform) with a pluggable
provider model. This is aimed at contributors working on the engine itself,
not at end users writing `.xcl` config.

## Start here

- [Overview](overview.md) — the big picture: how a `Config.Apply()` call
  flows from HCL files to running providers, how `Config.Destroy()` tears
  everything down from the saved state, and how the packages fit together.
- [Plugin Architecture](plugins.md) — how providers are authored, hosted
  (in-process vs. out-of-process/gRPC), and registered.
- [Parser & Resource Lifecycle](parser-lifecycle.md) — how HCL is parsed into
  a dependency graph and walked, and how `Create`/`Read`/`Changed`/`Update`/
  `Destroy` get invoked on providers, chosen from the state saved by the last
  apply, and how removed resources and `Config.Destroy` are destroyed
  children first.
- [Plugin Developer Guide](plugin-developer-guide.md) — the provider contract
  (`Create`/`Read`/`Changed`/`Update`/`Destroy`), what `old` and `new` are,
  what each method may and may not touch, computed fields, and what happens
  when a call fails.
- [State & Persistence](state.md) — what `State` holds, the resource
  statuses, what is saved after a failed apply and during a destroy, and how
  `FileStateStore` serializes it to disk.
- [Module System](modules.md) — how `module` blocks are resolved and what is
  and isn't implemented yet.

## Directory map

| Path | Purpose |
|---|---|
| `config.go`, `options.go` | Public facade: `Config` and functional options |
| `types/` | Shared resource metadata: `types.Meta`, `types.ResourceBase`, reflection helpers |
| `plugins/` | Provider contract (`ProviderAdapter`) and hosting (in-process / gRPC) |
| `plugins/registry/` | `PluginRegistry` — aggregates plugin hosts, resolves types to adapters |
| `internal/parser/` | HCL parsing, DAG construction, DAG walk, lifecycle calls, event instrumentation |
| `internal/resources/` | Built-in resource types: `module`, `output`, `variable`, `root` |
| `internal/schema/` | Reflection-based JSON schema generation/instantiation (Go struct ⇄ schema ⇄ dynamic struct) |
| `internal/modules/` | HTTP client for a Terraform-registry-style remote module API (not yet wired in) |
| `internal/functions/` | Custom HCL functions available to config authors |
| `state/` | `State`, `StateStore` interface, `FileStateStore` |
| `errors/` | Structured error types (`ParserError`, `ConfigError`) |
| `logger/` | Pluggable `Logger` interface + implementations |
| `example/` | Runnable, tested examples: `configonly` (registered types, no plugin) and `plugin` (an in-process and an external plugin), sharing `config/`, `resources/` and the `eventlog/` event handler |

See individual pages for details on how these pieces connect.
