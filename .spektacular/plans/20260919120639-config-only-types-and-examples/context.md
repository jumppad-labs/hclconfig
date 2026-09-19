---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Context: 20260919120639-config-only-types-and-examples

## Current State Analysis

All work is in the single registered repo **xclconfig** (root `/home/nicj/code/github.com/jumppad-labs/xclconfig`).

- **Type sources:** `plugins/registry/plugin_registry.go:35-44` `CreateResource` knows builtins (`types.RegisteredTypes`, created with `reflect.New` at `types/register.go:21-37`) and plugin types (`schema.CreateInstanceFromSchema`, a `reflect.StructOf` look-alike, at `:46-84`). There is no way to register a plain type.
- **Provider requirement:** `internal/parser/lifecycle.go:71-80` skips providers only for the builtin list in `isBuiltinType` (`:349-356`). Any other type without a provider fails with `no provider found for resource type`. The destroy walk has its own copy of the list (`internal/parser/callbacks.go:190-194`).
- **No clash checks:** `RegisterPlugin` (`:117`), `RegisterPluginWithPath` (`:131`) and `DiscoverAndLoadPlugins` (`:148`) append hosts without checking names. The first matching host wins silently.
- **Querier bug:** `querier.go:47-79` compares against the empty `Meta.Type` of a zero `T`, so `FindResourcesByType` never matches. Its fallback also unmarshals into a nil `*T`.
- **Validation gap:** `internal/parser/validate.go` has structure (computed), reference and property stages, but no attribute-schema check. A probe confirmed that `Validate` accepts `bogus = 1` on a plugin block, and `Apply` then fails at decode (`internal/parser/callbacks.go:108`).
- **State:** `state/file_state_store.go:80` reloads resources through `registry.CreateResource`, and unknown types are skipped silently. Apply never calls the destroy walk, so blocks removed from configuration simply drop out of state.
- **Example:** `example/` is stale. Its `.hcl` files are ignored by `findXclFiles` (`internal/parser/util.go:64`), its types are never registered, and it uses `random_number()`, `count` and an undefined `person` type.
- **Baseline:** `go vet ./...` and `go test ./...` pass on `v2` at the time of planning.

## Per-Phase Technical Notes

### Phase 1.1: Register plain types on the plugin registry and guard type names

**Requirement → repo/files**: Register without a plugin, both clash directions, plugin vs plugin, and computed fields accepted all land in xclconfig `plugins/registry/`.

**File changes**
- `plugins/registry/plugin_registry.go:17-30` — add a `registeredTypes types.RegisteredTypes` field, initialised in `NewPluginRegistry`.
- `plugins/registry/plugin_registry.go` (new code near `:35`) — `RegisterType(name string, resource any) error`:
  - Validate that `resource` is a non-nil pointer to a struct and that `types.GetMeta(resource)` succeeds. Otherwise return `fmt.Errorf("type %q must be a pointer to a struct that embeds types.ResourceBase", name)`.
  - Call `checkTypeName(name)`, then store it.
  - Add `IsRegisteredType(name string) bool`.
- `plugins/registry/plugin_registry.go:35-44` `CreateResource` — the order becomes builtin → `registeredTypes.CreateResource` → `createResourceFromPlugins`.
- `plugins/registry/errors.go` (new) — `TypeNameClashError{Name, Existing string}` with `Error()`: `type "<name>" is already provided by <existing>`.
- `plugins/registry/plugin_registry.go` (new private helpers):
  - `checkTypeName(name string) error` checks `builtinTypes`, then `registeredTypes` ("registered type"), then every host's `GetTypes()` with `Type == "resource"` ("plugin"). It returns `*TypeNameClashError`.
  - `checkHostTypes(host plugins.PluginHost) error` runs `checkTypeName` for each resource sub-type of the new host, and also catches duplicates within the host itself. It returns `errors.Join` of every clash.
- `plugins/registry/plugin_registry.go:117-129` `RegisterPlugin` — after `NewDirectPluginHost`, call `checkHostTypes(host)`. On error, `host.Stop()` and return it, without appending.
- `plugins/registry/plugin_registry.go:131-146` `RegisterPluginWithPath` — after `host.Start`, call `checkHostTypes(host)`. On error, `host.Stop()` and return `fmt.Errorf("plugin %s: %w", pluginPath, err)`.
- `plugins/registry/plugin_registry.go:148-188` `DiscoverAndLoadPlugins` — when `RegisterPluginWithPath` returns an error for which `errors.As(err, &*TypeNameClashError)` holds, collect it in `clashErrors` and log it with `logger.Error`. After the loop, if `len(clashErrors) > 0`, return `errors.Join(clashErrors...)`, regardless of how many other plugins succeeded. Other load errors keep the existing rule.
- Tests: `plugins/registry/plugin_registry_test.go` (new). Separate functions:
  - `TestRegisterTypeSucceedsWithoutPlugins`, `TestCreateResourceReturnsRegisteredGoType`, `TestRegisterTypeRejectsDuplicateName` (the first still creates), `TestRegisterTypeRejectsBuiltinName` (one function per builtin name, or a loop over the four names inside one negative test; no table struct), `TestRegisterTypeRejectsPluginProvidedName`, `TestRegisterPluginRejectsRegisteredTypeName` (plugin not added: `len(GetPluginHosts())` unchanged), `TestRegisterPluginRejectsNameFromAnotherPlugin`, `TestRegisterTypeAcceptsComputedField`, `TestRegisterTypeRejectsNonPointer`, `TestRegisterTypeRejectsTypeWithoutResourceBase`, `TestIsRegisteredTypeReportsRegisteredTypes`, `TestIsRegisteredTypeIgnoresPluginTypes`.
  - Use a small in-test plugin built on `plugins.PluginBase` with a no-op `DefaultChanged` provider, providing type `"thing"`. `internal/parser.TestPlugin` could be imported instead, but it lives in the non-test file `internal/parser/test_plugin.go` and importing it from `registry` would create a cycle (parser imports registry), so define a local one.
- Tests: `plugins/registry/plugin_discovery_test.go` — `TestDiscoverAndLoadPluginsRejectsPluginClashingWithRegisteredType`: register type `"person"`, build the example plugin with `newTestPluginSetup(...).buildExamplePlugin` (`testutils_discovery_test.go:45`), and discover it. Expect an error containing `person`, and no host added. Also `TestRegisterPluginWithPathRejectsRegisteredTypeName`.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2 parallel agents. One does the registry code and unit tests. The other does the discovery and external clash tests, after the error type exists. Integrate sequentially.

### Phase 1.2: Apply registered types without any provider

**Requirement → repo/files**: No lifecycle calls, the parse/decode/reference/ordering criteria, state and query presence, and existing features (disabled, modules) all land in xclconfig `internal/parser/`, with fixtures under `internal/test_fixtures/`.

**File changes**
- `internal/parser/callbacks.go:20-25` — add `type TypeRegistry interface { IsRegisteredType(name string) bool }` next to `ProviderResolver`.
- `internal/parser/parser.go:72-80` `ParserOptions` — add `TypeRegistry TypeRegistry` with a doc comment in the style of `ProviderResolver`. In `NewParser` (`:136-165`), default it to `p.pluginRegistry` when nil and the registry isn't nil. Store it in a new `typeRegistry` field on `Parser` (`:127-134`).
- `internal/parser/lifecycle.go:21-32` — add a `types TypeRegistry` field to `resourceLifecycle`.
  - Replace `isBuiltinType(meta.Type)` at `:72` with `l.handledWithoutProvider(meta.Type)`.
  - Replace `isBuiltinType` at `:349-356` with a free function `handledWithoutProvider(types TypeRegistry, t string) bool`, returning builtin, or `types != nil && types.IsRegisteredType(t)`.
- `internal/parser/parser.go:969-975` (in `walk`) — set `types: p.typeRegistry` when building `resourceLifecycle`.
- `internal/parser/callbacks.go:176` `destroyWalkCallback(registry ProviderResolver, types TypeRegistry, options *ParserOptions)` — replace the hard-coded list at `:190-194` with `handledWithoutProvider(types, rMeta.Type)`. Update its callers (tests only; grep `destroyWalkCallback(`).
- Fixture Go types: `internal/test_fixtures/registered/types.go` (new, package `registered`):
  - `Database` with fields `location`, `port`, a nested block `timeouts` (`*Timeouts`) and a computed `connection_string,optional,computed`.
  - `App`, which reads a variable, a database field and a module output.
  - `Consumer`, which reads an `App` field and has `depends_on`.
  - All carry `xcl` and `json` tags.
- Fixture configs under `internal/test_fixtures/config/registered/`:
  - `basic/main.xcl` — variable, database with nested timeouts, app, consumer, and a module using `module/db.xcl`, which declares a database and an output.
  - `disabled/main.xcl` — a database with `disabled = true`.
  - `removed/before/main.xcl` and `removed/after/main.xcl` — two databases, then one.
  - `invalid_reference/main.xcl`, `invalid_attribute/main.xcl` (used in 2.2).
- Tests: `internal/parser/registered_types_test.go` (new). Build the parser with `registry.NewPluginRegistry` plus `RegisterType` calls and **no** `RegisterPlugin`, and a `state.NewFileStateStore` in `t.TempDir()`. Separate functions:
  - `TestApplyRegisteredTypesSucceedsWithoutProvider` (status equals the `variable` resource's status)
  - `TestApplyRegisteredTypeDecodesValuesAndNestedBlock`
  - `TestApplyRegisteredTypeReturnsRegisteredGoType` (`reflect.TypeOf` equals `reflect.TypeOf(&registered.Database{})`, and a type assertion is ok)
  - `TestApplyRegisteredTypeResolvesReferences`
  - `TestApplyRegisteredTypeReadsModuleOutput`
  - `TestApplyProcessesDependentAfterRegisteredType` — record `OnParserEvent` "create"/"success" order and assert the index of `consumer` is greater than that of `app`.
  - `TestReapplyRegisteredTypesSucceedsWithoutProvider` — first apply, save through the store as `Config.Apply` does, then a second apply with the same store.
  - `TestApplyAfterRemovingRegisteredBlockSucceedsWithoutProvider`
  - `TestApplyRegisteredTypeInModuleIsStoredUnderModulePath`
  - `TestApplyMarksDisabledRegisteredTypeDisabled`
  - `TestApplyRegisteredTypeWithComputedFieldLogsNoWarning` — use `recordingLogger` (`internal/parser/recording_logger_test.go:30`) as `ParserOptions.Logger`, and assert `warnings()` is empty and `ConnectionString == ""`.
  - `TestRegisteredResourcesAreInStateAndFoundByPath`
  - `TestDestroyWalkSkipsProviderForRegisteredType` — call `destroyWalkCallback` directly with a mock resolver that has **no** expectations, and assert no diagnostics and `StatusDestroyed`.
- Re-apply and removal state comes from a real apply, per the test-state convention.

**Complexity**: Medium
**Token estimate**: ~35k tokens
**Agent strategy**: Single agent, sequential. The lifecycle change is small, and the tests share fixtures.

### Phase 2.1: List resources by type and return registered types as themselves

**Requirement → repo/files**: Querying by type and returning the developer's own type land in xclconfig `querier.go`.

**File changes**
- `querier.go:24-45` `FindResource` — when `meta.ID == path`, first `if typed, ok := r.(*T); ok { return typed, nil }`, else the existing `schema.UnmarshalUntyped`.
- `querier.go:47-79` `FindResourcesByType(typeName string)`:
  - Drop the `GetMeta(new(T))` lookup and compare `metaR.Type == typeName`.
  - Apply the same direct-assertion-first logic, and fix the fallback to unmarshal into a fresh `new(T)` (today it passes `&nr` with a nil `*T`).
  - Return `state.ResourceNotFoundError{Resource: typeName}` when empty.
  - Update the doc comments to say that registered types are returned by reference.
- Callers: `example/main.go:57` is removed in 3.1. There are no other callers (`grep -rn "FindResourcesByType()"`).
- Tests: `querier_test.go` (new, package `xcl`). A fixture under `internal/test_fixtures/config/query/main.xcl` holds three `database` blocks (registered type, from `internal/test_fixtures/registered`) and two `network` blocks (the plugin `TestPlugin` type). The config uses `setupConfig`-style construction plus `pr.RegisterType("database", &registered.Database{})`. Separate functions:
  - `TestFindResourcesByTypeListsRegisteredType` (exactly 3, IDs match)
  - `TestFindResourcesByTypeListsPluginType` (exactly 2, using `NewQuerier[structs.Network]`)
  - `TestFindResourcesByTypeReturnsNotFoundForUnusedType`
  - `TestFindResourceReturnsStoredRegisteredValue` (pointer equality with `c.FindResource`)
  - `TestFindResourceCopiesPluginResource`

**Complexity**: Low
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution.

### Phase 2.2: Reject attributes a resource's type doesn't have at validation time

**Requirement → repo/files**: The "invalid registered block fails validation" criterion and the user's all-types decision land in xclconfig `internal/xcl/gohcl/` and `internal/parser/validate.go`.

**File changes**
- `internal/xcl/gohcl/check.go` (new, header `// Copyright (c) HashiCorp, Inc.` / `// SPDX-License-Identifier: MPL-2.0` / `// Modifications Copyright (c) Jumppad Labs`) — `CheckBody(body hcl.Body, val any) hcl.Diagnostics`:
  - Mirrors `decodeBodyToStruct` (`internal/xcl/gohcl/decode.go:55-130`), using `ImpliedBodySchema` and `Content` or `PartialContent`.
  - For `tags.Remain`: when the field type is a struct or pointer to struct, recurse with the leftovers. When it's `hcl.Body` or `hcl.Attributes`, stop, since leftovers are allowed there.
  - For each `tags.Blocks` entry: take the element type (deref pointer, slice elem), and recurse for each block in `content.Blocks.OfType(name)` when it's a struct.
  - Never evaluates expressions.
- `internal/xcl/gohcl/check_test.go` (new, MPL header) — separate tests: accepts a known attribute, rejects an unknown attribute, rejects an unknown attribute inside a nested block, accepts attributes through an embedded `remain` struct (the `ResourceBase` pattern), rejects an unknown attribute when a `remain` struct is present.
- `internal/xcl/UPSTREAM.md` — add a bullet: "`gohcl/check.go`: new `CheckBody`, which validates a body against a Go value's implied schema without decoding."
- `internal/parser/validate.go:143-184` `validateStructure` — after the computed checks, when `body` is present:
  - `diags := gohcl.CheckBody(body, resource)`.
  - For each error diagnostic, append `errors.NewParserError(meta.File, d.Subject.Start.Line, d.Subject.Start.Column, fmt.Sprintf("resource '%s' %s: %s", id, d.Summary, d.Detail))`, falling back to the meta position when `Subject` is nil.
  - Module resources: `Module.Variables` is an `hcl.Expression` attribute, so the schema accepts it. Confirm with the existing module fixtures.
- Tests: `internal/parser/validate_test.go` — separate functions:
  - `TestValidateRejectsUnknownAttributeOnRegisteredType` (the message contains `resource.database.` and the attribute name)
  - `TestValidateRejectsUnknownAttributeOnPluginType` (TestPlugin `network`)
  - `TestValidateRejectsUndefinedReferenceFromRegisteredType`
  - `TestValidateAcceptsValidRegisteredTypeConfiguration`
- Regression: run the whole suite. If any existing fixture that is validated only now fails, STOP and report (an open assumption in research.md).

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2 parallel agents. One writes `gohcl.CheckBody` with its unit tests, the other writes the parser validation tests against the interface. Integrate sequentially.

### Phase 3.1: Shared example configuration and the configuration-only example

**Requirement → repo/files**: The configuration-only example, the example tests and the `.xcl` rename land in xclconfig `example/`.

**File changes**
- Delete `example/main.go`, `example/types.go`, `example/config.hcl` and `example/modules/db/db.hcl`.
- `example/config/main.xcl` (new):
  - variables `db_username` and `db_password`
  - `resource "postgres" "main"` with a `timeouts {}` block
  - `resource "postgres" "replica"` with `depends_on = ["resource.postgres.main"]`
  - `resource "app" "web"`, which reads `resource.postgres.main.location`, `variable.db_username`, `resource.postgres.main.connection_string` and `module.analytics.output.location`
  - `module "analytics" { source = "./modules/db" }`
  - output `web_database`
  - No custom functions and no `count` (neither is supported by the rewrite).
- `example/config/modules/db/db.xcl` (new) — one `postgres` block and an `output "location"`.
- `example/resources/resources.go` (new, package `resources`) — `PostgreSQL`, `Timeouts` and `App` with `xcl` and `json` tags. `ConnectionString string` is tagged `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`. The package holds types only, with no registration helper, so each program shows its own registration step.
- `example/configonly/main.go` (new, package main):
  - `run(out io.Writer, dir string) ([]any, error)` creates a registry with `logger.NewStdOutLogger()`, calls `RegisterType("postgres", &resources.PostgreSQL{})` and `RegisterType("app", &resources.App{})`, runs `xcl.NewConfig(xcl.WithPluginRegistry(r))`, then `Apply(dir)`.
  - It prints every resource ID with its key fields, `NewQuerier[resources.PostgreSQL](c).FindResourcesByType("postgres")`, and `FindResource("resource.app.web")`.
  - It returns `c.GetResources()`.
  - `main` uses `os.Args[1]` if given, else `"../config"`, and exits 1 on error.
- `example/configonly/main_test.go` (new):
  - `TestConfigOnlyExampleFindsDeclaredResources` — the exact sorted ID list: variables, outputs, the module, `resource.postgres.main`, `resource.postgres.replica`, `resource.app.web`, and `module.analytics.resource.postgres.<name>`, plus the module's output.
  - `TestConfigOnlyExamplePrintsQueryResult`
  - `TestConfigOnlyExampleLeavesConnectionStringEmpty`
  - `TestConfigOnlyExampleImportsNoPluginCode` — `go/parser.ParseDir(".", nil, parser.ImportsOnly)`, and fail if any import has the prefix `github.com/jumppad-labs/xcl/plugins` other than `github.com/jumppad-labs/xcl/plugins/registry`.
  - Pass `../config` as the dir.
- Remove `os.RemoveAll(".xclconfig")` and the stale comments.

**Complexity**: Low
**Token estimate**: ~18k tokens
**Agent strategy**: Single agent, sequential execution.

### Phase 3.2: Plugin example on the same configuration and types

**Requirement → repo/files**: The full plugin example and the docs land in xclconfig `example/plugin/`, `docs/`, `README.md` and `CHANGELOG.md`.

**File changes**
- `example/plugin/main.go` (new, package main):
  - `ExamplePlugin` embeds `plugins.PluginBase`. `Init` calls `plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "postgres", &resources.PostgreSQL{}, &postgresProvider{})`, and the same for `app` with an `appProvider` that is a no-op.
  - `postgresProvider` embeds `plugins.DefaultChanged[*resources.PostgreSQL]`. Its Create and Update set `ConnectionString = fmt.Sprintf("postgres://%s@%s:%d/%s", ...)`. Read returns `new`, and Destroy returns nil. The pattern follows `plugins/example/pkg/person/provider.go:15-50`.
  - `run(out io.Writer, dir string) ([]any, error)` is the same as configonly but calls `r.RegisterPlugin(&ExamplePlugin{})` instead of `RegisterType`.
  - Queries go through `NewQuerier[resources.PostgreSQL]`, the JSON copy path.
- `example/plugin/main_test.go` (new):
  - `TestPluginExampleFindsDeclaredResources` — same exact ID list as 3.1.
  - `TestPluginExampleFillsConnectionString`
  - `TestPluginExampleDefinesNoTypesOrConfig` — `go/parser` checks there's no `type` declaration of a struct that embeds `types.ResourceBase`, and that the directory has no `*.xcl` files.
- Docs:
  - `docs/plugins.md` or `docs/overview.md` — a short "Configuration-only types" section showing `RegisterType`, the clash rule, and a pointer to `example/configonly` and `example/plugin`.
  - `README.md` — update the example section.
  - `CHANGELOG.md` — add entries for `RegisterType`, the clash errors, `FindResourcesByType(typeName)`, and unknown-attribute validation.

**Complexity**: Low
**Token estimate**: ~18k tokens
**Agent strategy**: Single agent, sequential execution.


## Testing Strategy

The testing approach from plan.md, broken down per phase:

- **Phase 1.1** — Unit tests in `plugins/registry` for registration, creation order, `IsRegisteredType`, every clash direction (type vs builtin, type vs type, type vs plugin, plugin vs type, plugin vs plugin), input validation, and computed-field acceptance. Integration tests for the external and discovered clash, built on the example plugin binary.
- **Phase 1.2** — Parser integration tests with real `.xcl` fixtures and **no plugins registered**: decode, exact Go type, references in both directions, module output, ordering via `OnParserEvent`, re-apply and removal (state from a real first apply through `FileStateStore`), module path, disabled, no warning for computed fields (`recordingLogger`), and state/path lookup. One destroy-walk unit test with an expectation-free mock resolver.
- **Phase 2.1** — Root-package tests through `xcl.Config` and `Querier`: list-by-type for a registered and a plugin type in one configuration, not-found, pointer identity for registered, copy for plugin.
- **Phase 2.2** — `gohcl.CheckBody` unit tests (known, unknown, nested, remain) and parser validate tests (unknown attribute on a registered type and on a plugin type, undefined reference, valid config). A full-suite regression run guards the existing fixtures.
- **Phase 3.1** — Example tests calling `run`: exact resource ID set, query output, empty connection string, and the no-plugin-import check (success metric 1).
- **Phase 3.2** — Example tests: same exact ID set, filled connection string, and no own types or config.
- **Success metrics** — metric 1 is a behavioural test (3.1 example tests plus the import check). Metric 2 is a behavioural test (the example tests run under `go test ./...`). Metric 3 is a behavioural test (the full suite and `go vet ./...` at the end of every phase).

All tests follow house conventions: testify `require`, no table-driven tests, separate positive and negative functions, and verbose readable setup.

## Project References

- **Repo:** xclconfig, root `/home/nicj/code/github.com/jumppad-labs/xclconfig`. This is the only registered repo, and every phase lands here.
- **Spec:** `20260919120639-config-only-types-and-examples.md` (read with `spektacular spec file read`).
- **Knowledge (binding):** `conventions/testing-and-mocking.md`, `conventions/test-state-from-real-apply.md`, `conventions/never-modify-dependencies.md` (HCL fork MPL rules), `conventions/code-style.md`, `conventions/patterns-and-architecture.md`, `decisions/own-hcl-fork-with-xcl-tags.md`.
- **Patterns to copy:** builtin creation `types/register.go:21-37`; in-process plugin `plugins/example/main.go:13-42`; provider with computed field `plugins/example/pkg/person/provider.go:15-50`; config test setup `config_validate_test.go:27-57`; recording logger `internal/parser/recording_logger_test.go`; discovery test plugin build `plugins/registry/testutils_discovery_test.go:45`.
- **Fork notes:** `internal/xcl/UPSTREAM.md`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Phase budgets: 1.1 ~25k (Medium), 1.2 ~35k (Medium), 2.1 ~12k (Low), 2.2 ~25k (Medium), 3.1 ~18k (Low), 3.2 ~18k (Low). Always pass the repo root `/home/nicj/code/github.com/jumppad-labs/xclconfig` to sub-agents.

## Migration Notes

- `Querier[T].FindResourcesByType()` becomes `FindResourcesByType(typeName string)`. This is a breaking change that the spec allows because the library is unreleased. The only in-repo caller is the old example, which is removed. Record it in `CHANGELOG.md`.
- `destroyWalkCallback` gains a `TypeRegistry` parameter. It has test callers only.
- Registering a plugin or type whose name clashes now fails. Previously the first registered host silently won.
- Validate now rejects unknown attributes and blocks on every resource type, so configurations that used to fail only at Apply now fail at Validate.
- No state format change. Existing state files keep loading, and registered types reload as real Go types once the program registers them.

## Performance Considerations

- The clash check iterates the loaded hosts' `GetTypes()` on each registration. The number of plugins is small and registration is one-off, so this is negligible. External hosts' `GetTypes` is served from data cached at start.
- The schema check adds one `Content`/`PartialContent` pass per resource body at validation. That's the same cost decode already pays, and it doesn't evaluate expressions.
- The querier's direct `*T` assertion removes a JSON round trip for registered types, and plugin types are unchanged.
