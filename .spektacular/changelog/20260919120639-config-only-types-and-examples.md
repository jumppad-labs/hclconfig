---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Configuration-only types and working examples

## What was built

Developers embedding XCL can now register a plain Go type as a block type, with no plugin and no provider:

```go
r := registry.NewPluginRegistry(log)
err := r.RegisterType("postgres", &PostgreSQL{})
```

Blocks of a registered type are decoded into the developer's own Go type. They take part in references (reading variables, other resources and module outputs, and being read by other blocks) and in dependency ordering, work inside modules and when disabled, and are saved to state and reloaded as that same type. They never trigger a provider call, whether on a first apply, a re-apply or after a block is removed. The lifecycle and the destroy walk treat them exactly like the builtin `variable`, `output` and `module` blocks.

Type names are now guarded. Every way a name reaches the plugin registry (`RegisterType`, `RegisterPlugin`, `RegisterPluginWithPath` and plugin discovery) is checked against builtin names, registered types and every loaded plugin's types. A clash fails with a `*registry.TypeNameClashError` that names the type. A clashing plugin is stopped and not added, and discovery always fails on a clash, even when other plugins load.

Two gaps were fixed along the way:

- **Listing by type.** `Querier[T].FindResourcesByType` never returned anything. It now takes the type name (`FindResourcesByType("postgres")`, a breaking signature change) and returns exactly the resources of that type, for registered and plugin types alike. Registered types come back as the value held in state. Plugin types are copied, as before.
- **Unknown attributes.** `Validate` now rejects an attribute or block that a resource's type doesn't have, for every resource type, including plugins. Before, the mistake only surfaced partway through `Apply`, when the body was decoded. A new `gohcl.CheckBody` in the in-repo HCL fork does the check, following the decoder's schema rules without evaluating anything.

The stale example, which silently found no resources, was replaced with one shared configuration (`example/config`) and one set of Go types (`example/resources`), used by two programs. `example/configonly` registers the types with no plugin. `example/plugin` applies the same configuration through an in-process plugin, whose provider fills in a computed `connection_string` that stays empty in configuration-only mode. Both run as part of `go test ./...`, which checks the exact resource set, the query results and the difference between the two modes. The README example section and the plugin architecture docs describe the new API.

## Why it matters

Sometimes a block type only holds configuration and nothing needs to be created, read or destroyed. Until now the only way to add a type was to write a plugin and a provider, and callers got back a generated look-alike of their type instead of the type itself. Registered types make XCL usable as a plain configuration parser with references, modules and state, while plugins stay available for types that do need a lifecycle. The rebuilt examples give developers a working starting point for either use, and because they are tested they can't silently go stale again.

## Deviations from the plan

- **The schema check reports only unknown attributes and blocks.** The plan had `gohcl.CheckBody` report everything decode would. The first version did, and an existing test then got a duplicate "missing required argument" error for a non-optional computed field. The user chose to narrow the check to unknown attributes and blocks. Missing required attributes and blocks, and duplicate blocks, are still reported by decode at apply time, so disabled blocks (which are never decoded) that leave out required values still validate.
- **Example shape.** The example `PostgreSQL` type has no nested `DBCommon` and no `id` field. `App` has a plain `connection_string` field that reads the postgres computed field, so the difference between the two modes also shows on a block that references it. The module declares its own variable instead of receiving values from the root.
- **The old example was patched in between.** While Phase 2.1 changed `FindResourcesByType`, the old `example/main.go` was updated to the new signature so the build stayed green until Phase 3.1 deleted it.
- **The CHANGELOG entry was added late.** The plan had Phase 3.2 add entries to `CHANGELOG.md`. A single prose record in the file's existing style was added during the final changelog step instead.
- **Formatting.** `gofmt` fixed an import-order problem that was already in `plugins/registry/plugin_registry.go`.

## Open questions resolved during implementation

- Registered types survive a `FileStateStore` save and reload through `encoding/json` with every field intact, including nested pointer blocks and `depends_on`.
- A plugin's generated types copy back into the shared Go types with every field filled in, including the computed `connection_string`, when the Go types carry `json` tags that match their `xcl` names.
- No existing fixture was rejected by the new unknown-attribute check.

## Follow-up

- A querier helper that returns a typed collection directly (for example every resource of a Go type as `[]*T`, without passing a type name) is worth looking at separately.
- XCL-owned serialization based on `xcl` tag names, which would make the `json` tags on the example types unnecessary, is still pending work.
