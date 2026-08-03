# Module System

HCL `module` blocks let a config include another directory of `.xcl` files
as a scoped, reusable unit:

```hcl
module "consul_1" {
  source = "../single"
  variables = {
    cpu_resources = resource.container.base.resources.cpu
  }
}
```

## What's actually implemented: local paths only

[`Parser.parseModule`](../internal/parser/parser.go#L565) resolves `source`
as a **local filesystem path relative to the file that declared it** —
nothing else:

```go
sourceDir := filepath.Join(filepath.Dir(file), sourceVal.AsString())
```

It then discovers `.xcl` files in `sourceDir` (`findXclFiles`) and
recursively parses each one via `p.parseResourcesInFile(childFile,
moduleInstanceName)`, scoping every resource found there under
`moduleInstanceName` (the parent-qualified instance name, e.g.
`consul_3.consul_1` for a module nested inside another). Nested modules
work purely through this recursion — a module's source directory can
itself contain further `module` blocks.

There is no version resolution, no registry download, no caching. The
`Version` field on `resources.Module` ([`internal/resources/module.go`](../internal/resources/module.go))
is decoded from HCL but never read anywhere else in the module-resolution
path.

## Two-phase parsing

Module parsing is split across two passes (documented in
[`parser.go:558`](../internal/parser/parser.go#L558)):

- **Phase 1** (`parseModule`, at parse time): create a shell `Module`
  resource, resolve `source` to a directory, and recursively parse child
  files into resources scoped under this module's instance name. This
  happens before any dependency graph or evaluation context exists.
- **Phase 2** (during the DAG walk, in `walkCallback`): once the module's
  own dependencies are resolved and it has a full HCL evaluation context,
  its `Variables` expression (a raw `hcl.Expression`, not decoded via
  struct tags — see the comment at [`callbacks.go:120`](../internal/parser/callbacks.go#L120)
  explaining why gocty can't represent a heterogeneous object literal
  directly) is evaluated and stashed on `Module.SubContext`, where each
  child resource's own context-building step picks it up.

Cross-module references during interpolation (`module.consul_1.output.x`)
are resolved via a `module` namespace built in
[`internal/parser/context.go`](../internal/parser/context.go).

## Scaffolding that exists but isn't wired in

`ParserOptions` ([`internal/parser/parser.go:44`](../internal/parser/parser.go#L44))
declares several module-registry-related fields:

```go
ModuleCache         string
DefaultRegistry     string
RegistryCredentials map[string]string
ModuleRegistry      *modules.ModuleRegistry
```

`DefaultOptions()` even sets a default `ModuleCache` directory
(`$HOME/.xcl/cache`). **None of these are read anywhere in
`parseModule`** — they're declared and defaulted, never consumed.

The referenced type, [`internal/modules.ModuleRegistry`](../internal/modules/registry.go),
is a real, functioning HTTP client for a Terraform-registry-style remote
module API (`.well-known/registry.json` capability discovery, then
`/{org}/{module}/versions` and `/{org}/{name}/{version}` endpoints) — it
just isn't called from the module-resolution code path yet. Similarly,
`internal/getter/getter.go` exists with no importers anywhere in the
codebase — likely intended for fetching remote (git/HTTP) module sources
in the future.

**In short: if you're looking at `ModuleRegistry`/`ModuleCache` expecting
remote module support to work, it doesn't yet** — this is explicitly
scoped as future work; local relative-path sources are the only supported
mechanism today.
