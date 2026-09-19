---
created_date: "2026-09-19"
document_status: draft
project: xclconfig
spec: 20260919120639-config-only-types-and-examples
plan: 20260919120639-config-only-types-and-examples
---

# Configuration-only types and working examples

You can now register a plain Go type as a configuration block type and apply configuration that uses it without writing a plugin or a provider. Its blocks decode into your own Go type, can reference and be referenced by other blocks, work in modules and when disabled, and are saved to state, without ever calling a provider. Type names that clash with a builtin, a registered type or another plugin are now rejected with an error naming the type. Listing resources by type works, validation catches attributes a type doesn't have, and the repository has two tested, runnable examples: configuration only, and full plugin.

> Derived from project xclconfig (file), spec/plan 20260919120639-config-only-types-and-examples. See the project-level record for the full feature.

## What changed in this repo

- **Plugin registry** (`plugins/registry`):
  - `RegisterType(name, &MyType{})` registers a pointer to a struct that embeds `types.ResourceBase`. `IsRegisteredType(name)` reports whether a name was registered that way.
  - `CreateResource` now tries builtins, then registered types (a real instance of the Go type), then plugins.
  - A single clash check guards `RegisterType`, `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverAndLoadPlugins`, and returns the new `*TypeNameClashError`.
  - Loading two plugins that provide the same type, including two copies of one plugin, now fails instead of the first one silently winning.
- **Parser** (`internal/parser`):
  - A new `TypeRegistry` interface and the `ParserOptions.TypeRegistry` option, which defaults to the plugin registry.
  - The lifecycle and the destroy walk skip providers for registered types, the same way they do for builtins.
  - `Validate` reports unknown attributes and blocks on every resource, against the resource, at the attribute's position.
- **HCL fork** (`internal/xcl/gohcl`): a new `CheckBody` (MPL-2.0, recorded in `internal/xcl/UPSTREAM.md`), which checks a body for attributes and blocks its Go type doesn't have without decoding or evaluating anything.
- **Querier** (`querier.go`):
  - **Breaking:** `FindResourcesByType(typeName string)` now takes the type name, and returns exactly that type's resources.
  - Registered types are returned as the stored value, so changing one changes state. Plugin types are copied, as before.
- **Examples** (`example/`): the stale `.hcl` example was replaced by `config/` (shared `.xcl` configuration with a local module), `resources/` (shared Go types), `configonly/` and `plugin/`. Each program has tests that run under `go test ./...`.
- **Docs**: the README example section, `docs/plugins.md` ("Configuration-only types") and `docs/README.md`.

## Why

Not every block type needs a lifecycle. Adding a type used to require a plugin and a provider, and callers got back a generated look-alike of their type. This change lets the library be used as a configuration parser with references, modules and state, returning the developer's own types. It also leaves plugin and builtin behaviour unchanged, and makes sure name clashes and unknown attributes are reported early, with errors that name the problem.
