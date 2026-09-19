---
created_date: "2026-09-19"
document_status: draft
---

# Research: 20260919120639-config-only-types-and-examples

## Alternatives considered and rejected

- **A separate type registry passed through an `xcl.WithTypes` option.** Rejected by the spec constraint "Type registration goes wherever plugins are registered". It would also give `CreateResource` (`plugins/registry/plugin_registry.go:35`) and state reload (`state/file_state_store.go:80`) a second source to consult.
- **No-op provider "shim" for config-only types.** Register each type through an in-process plugin whose provider does nothing. Rejected in the spec: plugin types are created by `schema.CreateInstanceFromSchema` (`plugins/registry/plugin_registry.go:57`), which returns a `reflect.StructOf` look-alike rather than the user's type, and the lifecycle would still run Create/Read/Changed (`internal/parser/lifecycle.go:77-102`).
- **Adding `IsRegisteredType` to `ProviderResolver`** (`internal/parser/callbacks.go:23`). Rejected: the mockery mock (`internal/parser/mocks/mock_provider_resolver.go`) is used with a single `GetProviderForResource` expectation (`internal/parser/parse_test.go:717`). A second method would make every existing mock-based test panic with "no return value specified". A separate small interface, fed from the `PluginRegistry` the parser already holds, avoids that churn.
- **Deriving the type name in `Querier.FindResourcesByType` from `T`.** Would only work for registered types (reverse lookup by `reflect.Type`). Plugin types are generated look-alikes and can't be looked up that way, and the spec requires listing by type to work for both. Rejected in favour of passing the type name explicitly (breaking changes are allowed).
- **Schema check for registered types only.** Would keep plugin validation exactly as it is. Rejected by the user in favour of checking every resource type (see Open assumptions / user decisions).
- **Erroring on non-`.xcl` files passed explicitly.** Rejected by the user in the spec. `findXclFiles` (`internal/parser/util.go:64`) keeps skipping them silently.
- **Deleting `example/` in favour of `plugins/example`.** Rejected in the spec. The user wants both modes shown in `example/`.

## Chosen approach — evidence

- Builtins already show the "real Go type" path: `types.RegisteredTypes.CreateResource` (`types/register.go:21-37`) does `reflect.New(reflect.TypeOf(t).Elem())` and sets Meta name, type and properties. The same map type can hold user-registered types.
- `PluginRegistry.CreateResource` tries builtins first, then plugins (`plugins/registry/plugin_registry.go:35-44`). Registered types slot in between.
- State reload uses `registry.CreateResource` (`state/file_state_store.go:80`), so registered types reload as real types with no state-store change.
- Lifecycle skip point: `resourceLifecycle.run` returns early for `isBuiltinType` (`internal/parser/lifecycle.go:71-75`), firing a "create" success event and leaving Status unchanged (empty). Registered types take the same branch, which gives them "the same status builtins get".
- The destroy walk has its own hard-coded builtin list (`internal/parser/callbacks.go:190-203`). It needs the same registered-type skip. `destroyWalkCallback` isn't called by `Apply` today (no call site outside tests), so removing a block from configuration drops it from state without any provider call.
- Parser holds `pluginRegistry *registry.PluginRegistry` (`internal/parser/parser.go:129,155`) and builds the lifecycle in `walk` (`parser.go:969-975`). That's where the registered-type checker is injected.
- Plugin type lookup key is `RegisteredType{Type:"resource", SubType:<name>}` (`plugins/plugin.go:33-42`, `plugin_registry.go:61,106,224`). The clash check compares on `SubType` for `Type == "resource"`.
- Plugin entry points to guard: `RegisterPlugin` (`plugin_registry.go:117`), `RegisterPluginWithPath` (`:131`), `DiscoverAndLoadPlugins` (`:148`, which calls `RegisterPluginWithPath`). Discovery tolerates load failures unless all fail (`:182`), so a clash must be returned separately and always fail the call.
- `Querier.FindResourcesByType` bug: `types.GetMeta(new(T))` gives an empty `Meta.Type`, so `metaR.Type == metaT.Type` never matches (`querier.go:47-78`).
- `Querier` copies through `schema.UnmarshalUntyped` (`querier.go:36,66`). A direct `r.(*T)` assertion first returns registered types without any copy.
- Validate does **not** reject unknown attributes today. A probe (a temp test in the root package with `TestPlugin`, `network` block with `bogus = 1`) got `Validate` → nil and `Apply` → decode error. The decode path that catches it is `gohcl.decodeBodyToStruct` (`internal/xcl/gohcl/decode.go:55-130`): `ImpliedBodySchema`, then `Content` or `PartialContent` when a `remain` field exists, recursing through the remain struct with the leftovers and into nested blocks. The new check mirrors that without evaluating expressions. `getFieldTags` is unexported, so the check goes inside `internal/xcl/gohcl` (MPL, so it needs a modification notice and an `UPSTREAM.md` entry).
- In-process plugin pattern to copy for the plugin example: `PersonPlugin` with `plugins.PluginBase` and `plugins.RegisterResourceProvider` (`plugins/example/main.go:13-42`), and `ExampleProvider` embedding `plugins.DefaultChanged[*Person]` and filling a computed field in Create (`plugins/example/pkg/person/provider.go:15-50`).
- Computed-field tag pattern with json tags: `internal/test_fixtures/plugin/structs/network.go:15` (`xcl:"provider_id,optional,computed" json:"provider_id,omitempty"`).

## Files examined

- `config.go:16-131` — Config holds `pluginRegistry`. Apply/Validate build a parser with it. There's no separate type option.
- `options.go` — `WithPluginRegistry`, `WithStateStore`, `WithVariables`.
- `querier.go:14-79` — FindResource/FindResourcesByType. FindResourcesByType's type comparison is broken. Both copy via `schema.UnmarshalUntyped`.
- `plugins/registry/plugin_registry.go:1-231` — builtin and plugin creation, GetProvider / GetProviderForResource, RegisterPlugin / RegisterPluginWithPath / DiscoverAndLoadPlugins. No clash checks.
- `types/register.go` — `RegisteredTypes` map and `CreateResource` via `reflect.New`.
- `types/resource.go` — Meta (json tags) and ResourceBase.
- `types/resource_helpers.go:10-50` — GetMeta finds a promoted `ResourceBase` through nested embedding (PostgreSQL → DBCommon → ResourceBase works).
- `internal/resources/default.go` — builtins: variable, output, module, root.
- `internal/parser/lifecycle.go:64-103,349-356` — run(), the builtin skip, and `isBuiltinType`.
- `internal/parser/callbacks.go:23-25,30-172,176-256` — ProviderResolver, walkCallback (disabled handling, decode, lifecycle.apply), destroyWalkCallback's builtin list.
- `internal/parser/parser.go:72-80,136-165,194-356,427-470,949-993` — options, NewParser (resolver defaults to registry), Apply/Validate/parseAndValidate, parseResource → `pluginRegistry.CreateResource`, walk builds resourceLifecycle.
- `internal/parser/validate.go:1-190` — stages: structure (computed checks), references, properties. There's no attribute-schema check.
- `internal/xcl/gohcl/decode.go:55-130`, `schema.go:15-40` — how decode derives schema and handles remain/blocks.
- `state/state.go:107-129` — `State.FindResourcesByType(t string)` works by name.
- `state/file_state_store.go:32-107` — load via registry.CreateResource. Unknown types are silently skipped.
- `plugins/plugin.go:30-125` — RegisteredType, PluginBase.RegisterType, RegisterResourceProvider.
- `plugins/direct_plugin_host.go:10-35` — in-process host. `GetTypes` delegates to the plugin.
- `plugins/example/main.go`, `plugins/example/pkg/person/provider.go` — in-process plugin and provider pattern.
- `example/main.go`, `example/types.go`, `example/config.hcl`, `example/modules/db/db.hcl` — stale example. `.hcl` files are ignored. It uses the undefined `random_number()`, `count`, an unregistered `person` type, and a `module` with a relative source.
- `config_validate_test.go:27-57` — `setupConfig` pattern (TestPlugin, mock state store, `logger.NewTestLogger`).
- `config_test.go:33,347-456` — most of it is commented out, including the stale `c.FindResourcesByType` test.
- `plugins/registry/plugin_discovery_test.go`, `testutils_discovery_test.go` — discovery tests build the `plugins/example` binary with `go build`. Reuse this for the external/discovered clash tests.
- `internal/parser/parse_test.go:716-717` — mock resolver usage.

## External references

- None beyond the repo. `internal/xcl` is the in-repo HCL v2.21.0 fork, and its behaviour was read from source.

## Prior plans / specs consulted

- Spec `20260919120639-config-only-types-and-examples.md` — the source for this plan.
- Knowledge `decisions/own-hcl-fork-with-xcl-tags.md` — XCL owns the HCL fork and `xcl` tags carry `computed`. That's why the schema check belongs in `internal/xcl/gohcl`.
- Knowledge `conventions/never-modify-dependencies.md` — MPL rules for editing `internal/xcl` (header, modification line, `UPSTREAM.md`).
- Knowledge `conventions/test-state-from-real-apply.md` — the re-apply and removal tests must produce the first state with a real apply.
- Prior plans `20260918075104-validation-phase-before-walk` and `20260918165700-provider-lifecycle-read` are listed in the store. The current code was read directly instead, as the source of truth for the validate stages and the lifecycle.

## Open assumptions

- Registered-type instances round-trip through `encoding/json` for state save and load using their Go field names or json tags. Assumed to work because the same type is used on both sides. If state reload loses values, STOP and ask.
- The plugin example's generated look-alike converts back to the shared Go type through `schema.UnmarshalUntyped`, provided the shared types carry json tags that match their xcl names. If the query returns empty fields, STOP and ask. It ties in with the pending serialization work.
- The new schema check doesn't break existing fixtures, because anything it rejects would already have failed at decode. Fixtures that are only validated and never applied could still surface new errors. If they do, STOP and report which ones.
- Computed-field validation (the "must be optional" rule and "configuration must not set it") applies unchanged to registered types. "Accepted silently" is read as no registration error and no runtime warning. See assumptions.md.

## Drafting assumptions

### Chosen direction: types in PluginRegistry, lifecycle skip like builtins (architecture)
- **Decision**: Option A. `PluginRegistry.RegisterType` stores prototypes in a `types.RegisteredTypes` map, and `CreateResource` builds them with `reflect.New`. The parser treats registered types like builtins through a `TypeRegistry` interface (`IsRegisteredType`) in `lifecycle.run` and `destroyWalkCallback`. Clash checks go through one private `checkTypeName`. Querier takes the type name. `gohcl.CheckBody` adds a validation-time schema check for all types.
- **Key design decisions**: a separate interface rather than widening `ProviderResolver` (mock churn). Clash errors from discovery are always returned, even on partial success. Querier returns the stored pointer for registered types and copies for plugin types. Examples share one `run(out, dir)` function between `main` and the tests.
- **Rejected**: B, a separate type registry passed through `xcl.WithTypes` (violates the "register wherever plugins are registered" constraint; Low-Medium effort). C, a no-op provider shim (generated look-alikes and provider calls still run; Low effort but fails the requirements). A itself is Medium effort.

### Registered-type checker is a separate interface, not a ProviderResolver method (discovery)
- **Decision**: The lifecycle and destroy walk learn about registered types through a small separate interface that `*registry.PluginRegistry` satisfies.
- **Rationale**: Adding a method to `ProviderResolver` would break every mockery-mock-based test that only expects `GetProviderForResource`.
- **Rejected**: Extending `ProviderResolver` (mock churn). Passing the registry's type map directly (couples the parser to registry internals).

### FindResourcesByType takes the type name (discovery)
- **Decision**: `Querier[T].FindResourcesByType(typeName string)`.
- **Rationale**: The type name can't be derived from `T` for plugin look-alikes. Breaking API changes are allowed.
- **Rejected**: Reverse lookup of `T` in the registry (registered types only).

### Computed-field validation unchanged for registered types (discovery)
- **Decision**: Registered types go through the same computed-field validation as plugin types: a computed field must be optional, and configuration must not set it. There's no error at registration and no runtime warning.
- **Rationale**: The same configuration and types must work in both example modes. If configuration could set a computed field in config-only mode, the plugin mode would reject that configuration.
- **Rejected**: Skipping computed validation for registered types (the shared config would diverge between modes).

### Example package layout (architecture)
- **Decision**: `example/config` (`main.xcl` and `modules/db/db.xcl`), `example/resources` (Go types, package `resources`), `example/configonly` and `example/plugin` (package main each, with a `run(out io.Writer, dir string)` function tested by `main_test.go`).
- **Rationale**: The spec requires both modes to share one configuration and one set of types, with one small program per mode. A `run` function lets tests assert on the result without spawning a process.
- **Rejected**: `example/types` (package name clashes with `xcl/types` and would force an import alias). Testing the examples by `go run` subprocess (slower, and harder to assert the exact resource set).

### Conventions selected (architecture)
- **Decision**: Kept testing-and-mocking, test-state-from-real-apply, never-modify-dependencies (HCL fork rules), code-style, patterns-and-architecture and development-standards (logging). Dropped database-and-external-services, dependencies and project-structure.
- **Rationale**: The dropped ones don't bear on this work: there's no database, no new third-party dependency, and no new top-level directory beyond the existing `example/`.
- **Rejected**: Listing all conventions (noise).

### Typed clash error and TypeRegistry option (data_structures)
- **Decision**: Clashes return `registry.TypeNameClashError{Name, Existing}`. `ParserOptions.TypeRegistry` is an optional override that defaults to `PluginRegistry`.
- **Rationale**: A typed error lets tests assert the clash without matching strings, and the message still names the type. The option mirrors the existing `ProviderResolver` override pattern.
- **Rejected**: Plain `fmt.Errorf` (tests would have to match strings). A mandatory option (breaks every existing `NewParser` caller for no gain).

### A plugin that lists the same type twice is a clash (implementation_detail)
- **Decision**: When a single plugin reports the same resource type name twice, the registration is rejected with the same clash error.
- **Rationale**: Such a plugin has an ambiguous provider for that type, and it costs nothing to check it with the same gate.
- **Rejected**: Ignoring it (the first host type silently wins, as today).

### Querier returns the stored pointer for registered types (implementation_detail)
- **Decision**: The querier returns the state's own `*T`, not a copy, when the stored value is already `*T`.
- **Rationale**: This is what the spec asks for ("returned as instances of the Go type ... without any conversion"), and it avoids the JSON-tag dependency.
- **Rejected**: Returning a shallow copy (still a conversion, and it surprises callers who expect identity).

### Example config path resolution (implementation_detail)
- **Decision**: Each example's `main` resolves `../config` relative to its own package directory (the working directory when run with `go run` from inside the package), and also accepts an optional directory argument.
- **Rationale**: Tests run with the package directory as the working directory, and a flag or argument covers running the example from the repo root.
- **Rejected**: `runtime.Caller`-based paths (fragile with `-trimpath`). Embedding the config (would hide the files the example is meant to show).

### Guard "no plugin code" in the config-only example with an import check (testing_approach)
- **Decision**: A test parses the config-only example's Go files and fails if any of them imports the `plugins` package.
- **Rationale**: This turns the success metric "contains no plugin or provider code" into an automated check.
- **Rejected**: A manual review note (it can drift unnoticed).

### Test placement (phases)
- **Decision**: Apply, lifecycle and ordering tests live in `internal/parser` (where `recordingLogger` and `OnParserEvent` are available). Query tests live in the root `xcl` package. Registry tests use a local test plugin, to avoid an import cycle with `internal/parser`.
- **Rationale**: Reuses the existing helpers and keeps each test next to the code it exercises.
- **Rejected**: Putting every test at the `xcl.Config` level (no access to the recording logger or events).

### Example config drops custom functions and count (phases)
- **Decision**: The rewritten config uses no `random_number()` and no `count`.
- **Rationale**: Neither is supported by the current parser, and the example has to run.
- **Rejected**: Keeping them (the example would fail).

## Rehydration cues

- `spektacular spec file read 20260919120639-config-only-types-and-examples.md`
- Re-read: `plugins/registry/plugin_registry.go`, `internal/parser/lifecycle.go:64-103`, `internal/parser/callbacks.go:176-210`, `querier.go`, `internal/parser/validate.go:140-190`, `internal/xcl/gohcl/decode.go:55-130`.
- `spektacular knowledge always-applied --tier repo --filter xclconfig`
- Baseline before the change: `go vet ./...` and `go test ./...` both pass (2026-09-19).
