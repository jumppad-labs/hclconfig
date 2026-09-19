# Working context: config-only-types-and-examples

## Problem and motivation

- `example/` is stale from the old hclconfig API. It compiles and runs but finds
  no resources, for two reasons:
  1. Its files are `config.hcl` and `modules/db/db.hcl`. The parser only reads
     `.xcl` files, even a file passed explicitly by path
     (`internal/parser/util.go:64`, `findXclFiles`), so nothing is parsed and no
     error is returned.
  2. `example/types.go` defines plain structs (`Config`, `PostgreSQL`,
     `DBCommon`, `Timeouts`) that are never registered. Since the plugin
     refactor, non-builtin resource types only come from plugins registered on
     the `PluginRegistry`, each with a provider.
- User wants the example rewritten to demonstrate two modes (user's words):
  "a really basic config only parse mode, no plugin just types. Then the same
  example but with full plugins."
- User on the API: "I like this register type, sometimes you don't need a
  plugin you just want a simple config block type is fine here".

## Current behaviour found in code (drives the design)

- Registered types need a provider. For a non-builtin type with no provider,
  the lifecycle fails with `no provider found for resource type ...`
  (`internal/parser/lifecycle.go:78`, also `callbacks.go:213`). Only builtins
  skip providers, via the hard-coded `isBuiltinType` list
  (`lifecycle.go:351`: variable, output, module, root).
- Plugin types are instantiated from the plugin's schema with
  `reflect.StructOf` (`plugins/registry/plugin_registry.go`,
  `createResourceFromPlugins`), so callers get an anonymous struct shaped like
  the type, not the user's concrete type (e.g. not `*PostgreSQL`). Necessary
  for out-of-process plugins; wrong for config-only use where callers expect
  `NewQuerier[PostgreSQL]` to return their own type.
- Builtins are created with `types.RegisteredTypes.CreateResource`, which does
  `reflect.New` of the registered type, sets Meta name/type.
- `ProviderResolver` interface (`callbacks.go:23`) has one method,
  `GetProviderForResource`; there is a mockery mock
  (`internal/parser/mocks`) used in `parse_test.go:716`.

## Direction agreed so far

- Add direct type registration without a plugin, e.g.
  `registry.RegisterType("postgres", &PostgreSQL{})` (or an `xcl.WithTypes`
  option). Instances created with `reflect.New` of the real type so querying
  returns `*PostgreSQL`.
- Lifecycle treats provider-less registered types like builtins: decoded,
  references resolved, included in state, no Create/Read/Update/Destroy calls.
- Proposed (not yet confirmed by user): registering a type that has a
  `computed` field is an error, since nothing would ever set it.
- Rewrite of the example:
  - shared types in `example/types` (proposed layout)
  - `example/configonly`: types only, no plugin
  - `example/plugin`: same config and types, with an in-process plugin whose
    provider fills a computed field (`connection_string`)
  - tests that run both so the example cannot rot unnoticed
  - rename `.hcl` files to `.xcl`
- Proposed (user has not answered): passing a non-`.xcl` file explicitly to
  Apply/Validate returns an error instead of silently skipping it. Directory
  scans still skip non-`.xcl` files.

## Alternatives considered

- A "plugin shim" that registers types with a no-op provider (user's initial
  suggestion). Rejected in favour of direct registration because the shim
  would still produce schema-generated anonymous structs rather than the
  user's own types, and a no-op provider would still run Create/Read/Changed.
- Deleting `example/` in favour of `plugins/example` (which already shows the
  external gRPC plugin route and has e2e tests). Rejected; user wants the
  example rewritten to show both modes.

## Related context from this session (not part of this spec)

- Struct tags are now `xcl:"name,kind,computed,key"`, parsed by
  `internal/xcl/tags`.
- go-cty is vendored at `internal/cty` (v1.15.0, gocty modified), dag at
  `internal/dag`. No replace directives in go.mod.
- Pending separately: XCL-owned JSON serialization using xcl tag names with
  implicit omitempty (user: "extend encoding/json to use the xcl name, we
  should also always assume omitempty"). Candidate for its own spec.

## Interview answers (user)

- Registration lives wherever plugins are registered (PluginRegistry).
- Computed fields on registered types: ignore, no warning.
- Name clashes are an error, checked both when registering a type and when
  registering a plugin.
- Provider-less types return the real Go type; plugins unchanged (Querier
  already converts). Plugin instantiation change is out of scope.
- Keep ignoring non-.xcl files, even when passed explicitly (the earlier
  "error on explicit file" proposal is rejected).
- Fixing Querier.FindResourcesByType (compares against empty Meta.Type of
  zero T) is IN scope (user confirmed requirements).
- All sections confirmed. Fresh-eyes review produced 14 findings; user approved
  all fixes (dedup, a "no change to existing plugin/builtin behaviour" constraint,
  extra acceptance criteria for module outputs, ordering, re-apply, removal,
  split validate and plugin-vs-plugin clash criteria).
- Spec committed to the store as
  20260919120639-config-only-types-and-examples.md. Next: plan workflow. User confirmed breaking public API changes are acceptable (unreleased library).

## Plan workflow (started 2026-09-19)

- Plan name: 20260919120639-config-only-types-and-examples. Working files in
  `.spektacular/work/20260919120639-config-only-types-and-examples/`.
- Single repo: xclconfig, root `/home/nicj/code/github.com/jumppad-labs/xclconfig`.
- User decision (discovery): Validate currently does NOT reject unknown
  attributes for any type (probe: Validate nil, Apply decode error). User chose
  to add a schema check for ALL resource types (plugins included), not only
  registered types. Check lives in internal/xcl/gohcl (MPL rules apply).
- Learned: Apply never calls destroyWalkCallback, so removed blocks are simply
  dropped from state; state reload uses registry.CreateResource.
- Architecture locked: RegisterType on PluginRegistry; parser TypeRegistry
  interface (IsRegisteredType) for lifecycle/destroy skip; checkTypeName shared
  clash check; Querier.FindResourcesByType(typeName); gohcl.CheckBody schema
  check in validateStructure; example/{config,resources,configonly,plugin}.
- Drafting done through phases (3 milestones / 6 phases). Next: open_questions.
- Assembled docs staged in .spektacular/tmp/{plan,context,research}_template.md. Next: verification.
- Verification passed (added Project References to context). Next: write steps.
- All three plan docs committed to store; work dir removed. Now in walkthrough (read docs via spektacular plan file read).
- Walkthrough complete (2026-09-19), user signed off. Added Out of Scope follow-up: a typed collection helper on the querier (e.g. all resources of a Go type as []*T without passing a type name). User accepted all drafting assumptions unchanged. Next: implement workflow.

## Implement workflow (started 2026-09-19)

- read_plan passed: structure OK, no drift (destroyWalkCallback has no callers at all, not even tests), full spec coverage. First-phase run (no Changelog section yet).
- Phase 1.1 analysis: GetMeta panics on non-struct / nil pointers, so RegisterType validates kind before GetMeta. plugins/example binary provides type "person" (use for discovery/external clash tests).
- Phase 1.1 code: plugins/registry/errors.go (TypeNameClashError, Existing = "builtin" | "registered type" | "plugin" | "the same plugin"); RegisterType/IsRegisteredType/checkTypeName/checkHostTypes in plugin_registry.go. Discovery collects clash errors separately and always returns them. gofmt fixed a pre-existing import order issue in plugin_registry.go.
- Phase 1.1 tests: plugins/registry/plugin_registry_test.go + 4 tests appended to plugin_discovery_test.go. Note: loading two copies of the same external plugin now fails with a clash (intended).
- Pre-existing: ~20 files fail gofmt (import order after cty embed); not ours, left alone. gofmt only files we touch.
- Phase 1.1 done & changelogged. User said run all phases without asking.
- Phase 1.2 code: TypeRegistry in callbacks.go, ParserOptions.TypeRegistry (defaults to PluginRegistry only when non-nil), handledWithoutProvider(typeRegistry, t) replaces isBuiltinType + destroy list; destroyWalkCallback(registry, typeRegistry, options).
- Phase 1.2 done. Registered types round-trip through FileStateStore fine (open question resolved). Gotcha: implement goto must go analyze→implement→test→verify→update_plan→update_changelog in order; check each goto's error.
- Phase 2.1 done (querier asType helper; example/main.go temp-patched).
- Phase 2.2: USER DECISION: schema check reports only unknown attributes/blocks (option A); attrs forced non-required in CheckBody; missing required / duplicate blocks left to decode. Triggered by TestValidateRejectsNonOptionalComputedField getting a duplicate 'Missing required argument'.
- Phase 2.2 done.
- Phase 3.1 done. Example declared ID set = 10 IDs (see example/configonly/main_test.go declaredResourceIDs).
- Phase 3.2 done; all phases ticked. CHANGELOG.md left to feature-changelog step.
- Feature changelog written (.spektacular/changelog + CHANGELOG.md entry). Tests rebuild tracked plugins/example/build/example binary; restored with git checkout.
- Spec reconciled: all 38 checkboxes satisfied. Implement workflow finished 2026-09-19.
