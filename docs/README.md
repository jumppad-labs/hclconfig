# xclconfig Architecture Wiki

Internal architecture documentation for xclconfig — an HCL-based configuration
and provisioning engine (conceptually similar to Terraform) with a pluggable
provider model. This is aimed at contributors working on the engine itself,
not at end users writing `.xcl` config.

## Start here

- [Overview](overview.md) — the big picture: how a `Config.Apply()` call
  flows from HCL files to running providers, and how the packages fit
  together.
- [Plugin Architecture](plugins.md) — how providers are authored, hosted
  (in-process vs. out-of-process/gRPC), and registered.
- [Parser & Resource Lifecycle](parser-lifecycle.md) — how HCL is parsed into
  a dependency graph and walked, and how `Create`/`Refresh`/`Changed`/`Update`/
  `Destroy` get invoked on providers in the right order.
- [State & Persistence](state.md) — what `State` holds, how it's diffed, and
  how `FileStateStore` serializes it to disk.
- [Module System](modules.md) — how `module` blocks are resolved and what is
  and isn't implemented yet.

## Directory map

| Path | Purpose |
|---|---|
| `config.go`, `options.go`, `diff.go` | Public facade: `Config`, functional options, `Diff`/`buildDiff` |
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
| `example/` | Runnable sample program |

See individual pages for details on how these pieces connect.
