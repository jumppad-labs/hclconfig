# Restore module support in the v2 parser

## What was built

Configuration can once again be organized into `module` blocks: a named, reusable unit of configuration that packages a set of resources into something a user can instantiate multiple times with different variable values, and even nest inside other modules. Previously, `module` blocks were silently parsed and then skipped entirely — their resources never appeared in the parsed configuration or state. This restoration makes `module` blocks fully functional again, built to fit the parser's current two-phase parse-then-decode architecture rather than reintroducing the old single-pass design.

Concretely, this delivers:

- **Module discovery and instantiation.** A `module` block creates a real, trackable unit of configuration. Its `source` attribute is resolved as a directory relative to the file that declared it, and every `.xcl` file in that directory is parsed as part of that module instance, with resources addressable by a path that includes the module instance's name (e.g. `module.consul_1.resource.container.consul`).
- **Independent, non-colliding instances.** The same module source can be instantiated multiple times under different names, and each instance's resources are fully independent — no shared or leaked state between instances.
- **Variable passing with default fallback.** A module instance can supply values for the module's own variables (`variables = { ... }`), and those values are used inside that instance in place of the module's own defaults. An instance that supplies nothing still gets the module's default value.
- **Cross-boundary interpolation.** Resources inside a module can reference each other and the module's own variables. Values produced by a module instance — including its declared `output`s — can be referenced from outside the module (e.g. `module.consul_1.output.some_value`), and this works regardless of the order things are declared in the configuration file.
- **Nested modules.** A module can itself contain other module instances, with resources correctly scoped and addressable through arbitrary nesting depth, matching how this worked before the two-phase refactor.
- **Disabled propagation and override.** Marking a module instance as disabled marks every resource it produces as disabled too (they remain visible in state, just not further processed) — including through nested modules. A resource inside a module can still independently determine its own disabled state (e.g. based on one of the module's variables), regardless of whether the module itself is disabled.

Local relative-path module sources are supported; remote or URL/git-based module sources remain out of scope, matching the project's current constraints.

## Why it matters

Before this work, users could not share or reuse common infrastructure patterns (for example, a standard container setup) across a configuration without duplicating the same resources by hand. Module blocks existed in the language and were even accepted by the parser without error, but produced nothing — a confusing, silently-broken state. Restoring this closes that gap and brings back a capability the project previously supported, letting users package and reuse configuration the way `module` blocks were always meant to work.

## Deviations from the plan

The plan's technical design held up well overall — the two-phase parse/decode architecture, DAG-based dependency walk, and dot-joined FQRN module-scoping scheme required no structural changes, exactly as anticipated. A few things came out differently than planned during implementation:

- **`Module.Variables`'s retype target changed.** The plan called for retyping the field from `any` to `map[string]cty.Value` to fix a confirmed decode bug (an `any`-typed field cannot be decoded by `gohcl.DecodeBody` at all). During implementation, `map[string]cty.Value` turned out to have its own decode limitation: it forces every value in a `variables = { ... }` object literal to share one common cty type, which breaks as soon as a module instance passes a mix of types (e.g. a number and a boolean in the same block) — a real, common case in the test fixtures. The field was instead retyped to `hcl.Expression` (a documented special case gohcl supports for capturing a raw, un-decoded expression), evaluated manually once a full decode context is available, producing a `cty.Value` that can hold a heterogeneous object correctly.
- **Two additional, pre-existing bugs were found and fixed, both required for the target behavior to actually work, not incidental cleanup:**
  - The generic HCL expression walker that extracts dependency links had no case for a unary expression (e.g. `disabled = !variable.enabled`), so such a reference was silently never tracked as a dependency. This is a pre-existing gap unrelated to modules specifically, exposed for the first time by the module test fixtures.
  - The interpolation context builder resolved a resource's dependency links without first re-scoping them relative to that resource's own module — so a bare reference like `variable.cpu_resources` written from inside a module never matched the module-qualified resource it was meant to point to. Fixed to apply the same module-relative resolution the dependency-graph builder already used.
  - A `module` namespace was added to the interpolation context (alongside the pre-existing `resource` and `variable` namespaces) so that a reference like `module.consul_1.output.some_value` — used from *outside* the module — resolves correctly. This namespace did not exist anywhere in the codebase before this work.
- **Test fixture and test scope adjustment**, agreed during planning: one existing module test instance was fetching configuration from a remote source, which conflicts with the local-only constraint; it was swapped for an additional local-source instance, and its companion test was narrowed to only assert local-source, no-caching behavior rather than exercising remote fetching.

All six previously-failing module-related tests now pass, and the parser package's full test suite has no regressions to previously-passing, non-module behavior. A small number of pre-existing test failures elsewhere in the repository (in unrelated packages, plus one pre-existing mock-setup bug in a non-module parser test) were confirmed via a baseline-commit comparison to predate this work entirely and are outside this change's scope.
