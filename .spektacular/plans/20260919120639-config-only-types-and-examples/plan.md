---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Plan: 20260919120639-config-only-types-and-examples

<!-- Metadata -->
<!-- Created: 2026-09-19T12:39:29Z -->
<!-- Commit: 99f8887 -->
<!-- Branch: v2 -->
<!-- Repository: git@github.com:jumppad-labs/hclconfig.git -->

## Overview

Developers embedding XCL can register a plain Go type as a configuration block type on the plugin registry, and apply configuration that uses it without writing a plugin or provider. Those blocks are decoded into the developer's own type, linked, validated, saved to state and returned as that type, and they never trigger a provider call. Along the way, type-name clashes are rejected wherever a name enters the registry, listing resources by type is fixed, and validation catches attributes a type doesn't have. The stale example is rebuilt as two tested programs, configuration-only and full plugin, on one shared configuration, so developers get a working starting point for either use and it can't silently go stale again.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases, favour verbosity** — every new behaviour (registration, clashes, lifecycle skip, query, schema check, examples) gets its own named test functions, with separate success and failure tests.
- **Generate test state with a real apply, not a hand-written state file** — the re-apply and removal criteria need an earlier state, which must come from a first real apply with a file state store.
- **Never modify dependencies / HCL fork rules** — the new `gohcl.CheckBody` goes into `internal/xcl/gohcl` (MPL-2.0). Keep the HashiCorp header, add `// Modifications Copyright (c) Jumppad Labs`, and record the change in `internal/xcl/UPSTREAM.md`.
- **Code style: small focused interfaces, explicit error handling, descriptive names, `any` over `interface{}`** — drives the separate `TypeRegistry` interface instead of widening `ProviderResolver`, and the explicit clash errors.
- **Patterns & Architecture: dependency injection for testability** — the lifecycle receives its type checker through the parser rather than reaching for a global, so tests can supply a fake.
- **Development standards: structured logging** — applies to the discovery clash reporting, which logs through the existing `logger.Logger`.


## Architecture & Design Decisions

All work lands in the single registered repo, **xclconfig** (`/home/nicj/code/github.com/jumppad-labs/xclconfig`). Registered configuration types live in the `PluginRegistry` (`plugins/registry/plugin_registry.go`), next to the builtin types and plugin hosts it already holds, because the spec says registration goes wherever plugins are registered. The new `RegisterType(name string, resource any) error` stores a pointer-to-struct prototype in a `types.RegisteredTypes` map, the same map type the builtins use. `CreateResource` then builds registered types with `reflect.New`, exactly as it builds builtins, so blocks decode straight into the developer's own type. State reload (`state/file_state_store.go:80`) and module parsing already go through `CreateResource`, so they pick registered types up with no change. Plugin types keep being generated from their schema, which is out of scope.

In the lifecycle, a registered type is treated like a builtin rather than routed through a provider. The parser gets a small interface, `TypeRegistry` with `IsRegisteredType(name string) bool`, which `*registry.PluginRegistry` satisfies. `resourceLifecycle.run` and `destroyWalkCallback` skip provider calls when a type is builtin or registered. The two hard-coded builtin lists are folded into one helper, so the next caller can't miss either kind. Registered resources then get the same "create" success event and the same (unchanged) status as builtins. A separate interface is used rather than a new method on `ProviderResolver`, because the mockery mock of `ProviderResolver` would otherwise panic in every existing test that only expects `GetProviderForResource` (small focused interfaces, per conventions). A no-op provider shim was rejected because it would still produce generated look-alike types and still run Create/Read/Changed.

Type-name clashes are checked in one place: a private `checkTypeName(name, source string) error` on the registry compares a name against builtins, registered types and the `resource` sub-types of every plugin host already loaded. `RegisterType` calls it for its one name. `RegisterPlugin` and `RegisterPluginWithPath` call it for every resource type the new host provides, before the host is added. An external host is stopped when it clashes. `DiscoverAndLoadPlugins` always returns clash errors, even when other plugins load, because the spec requires discovery to fail on a clash, while ordinary load failures keep their current partial-success handling. Every clash message names the type and says what it clashes with.

Two query and validation gaps are fixed along the way. `Querier[T].FindResourcesByType` takes the type name explicitly (`FindResourcesByType(typeName string)`), because a zero `T` has no `Meta.Type` and plugin look-alikes can't be matched back to `T`. Both `Querier` methods return the stored pointer directly when it already is a `*T`, and fall back to today's JSON copy for plugin types. Validation gains an attribute-schema check for **every** resource type (the user's choice during discovery). A new `gohcl.CheckBody` in the in-repo HCL fork mirrors `decodeBodyToStruct` (schema, `remain` and nested blocks) without evaluating anything. `validateStructure` reports its diagnostics against the resource ID, so an unknown attribute now fails at `Validate` instead of at `Apply` decode time. The example is rebuilt as `example/config` (shared `.xcl` files), `example/resources` (shared Go types), `example/configonly` and `example/plugin`, one small program per mode. Each exposes a `run(out io.Writer, dir string)` function that `main` and the tests share. Rejected alternatives, with evidence, are in `research.md#alternatives-considered-and-rejected`.


## Component Breakdown

- **PluginRegistry (changed)** — remains the single place where resource types come from. It newly owns a map of registered configuration types, next to its builtins and plugin hosts. It gains `RegisterType` for plain Go types and `IsRegisteredType` for the parser. `CreateResource` resolves names in this order: builtin, then registered type (a real Go instance), then plugin (a generated look-alike). The state store and parser keep calling it unchanged.
- **Type-name clash check (new, private to PluginRegistry)** — a single check that compares a candidate name against builtins, registered types and every loaded plugin's resource types, and produces the one clash error format. `RegisterType`, `RegisterPlugin`, `RegisterPluginWithPath` and `DiscoverAndLoadPlugins` all use it, so both clash directions and plugin-versus-plugin clashes behave the same.
- **TypeRegistry interface (new, parser)** — a one-method interface (`IsRegisteredType`) through which the parser asks whether a type is a registered configuration type. The PluginRegistry satisfies it. It's kept separate from `ProviderResolver` so existing resolver mocks are unaffected.
- **Resource lifecycle and destroy walk (changed)** — decide whether a resource needs a provider. Both now consult one "handled without a provider" helper (builtin or registered). Registered resources get the builtin treatment: a success event, unchanged status, no Create/Read/Changed/Update/Destroy. Plugin resources behave exactly as before.
- **Validation structure stage (changed)** — besides its computed-field checks, it now runs a schema check of each resource body against the resource's type (every type, plugins included) and reports each unknown attribute or block against the resource that holds it.
- **`gohcl.CheckBody` (new, in-repo HCL fork)** — walks a body against a Go value's implied schema the way decode does (`remain` fields, nested blocks, repeated blocks) and returns the diagnostics without evaluating expressions. The validation stage calls it. It lives in the fork because it needs the fork's private tag parsing.
- **Querier (changed)** — typed lookup over the Config's current state. `FindResourcesByType` now takes the type name and matches it against each resource's `Meta.Type`. Both lookups return the stored value directly when it already is `*T` (registered types) and fall back to the JSON copy for plugin look-alikes.
- **Example configuration and types (rewritten, shared)** — one `.xcl` configuration (root and a local module) and one Go types package used by both example programs. The types include a computed `connection_string` field.
- **Config-only example program (new)** — registers the shared types with `RegisterType`, applies the shared configuration with no plugin, and prints the resources and a query result.
- **Plugin example program (new)** — an in-process plugin and provider for the same shared types, applied to the same configuration. Its provider fills `connection_string`, which stays empty in config-only mode.
- **Example tests (new)** — run each program's shared `run` function against the shared configuration and assert the exact set of resources, the query result, and the `connection_string` difference between modes.


## Data Structures & Interfaces

**`PluginRegistry` (changed, public)**. It gains a registered-types map and two methods. `RegisterType` takes a pointer to a struct that embeds `types.ResourceBase`. It rejects anything else, and any clashing name, and it leaves the registry unchanged when it fails.

```go
type PluginRegistry struct {
    builtinTypes    types.RegisteredTypes
    registeredTypes types.RegisteredTypes // new: plain configuration types
    pluginHosts     []plugins.PluginHost
    logger          logger.Logger
}

func (r *PluginRegistry) RegisterType(name string, resource any) error
func (r *PluginRegistry) IsRegisteredType(name string) bool
```

**`TypeNameClashError` (new, public, package `registry`)**. It's returned for every clash, whichever route the name arrived by, so callers and tests can use `errors.As` and the message always names the type.

```go
type TypeNameClashError struct {
    Name     string // the clashing type name
    Existing string // what already holds it: "builtin", "registered type", or the plugin that provides it
}
// Error(): `type "postgres" is already provided by registered type`
```

**`parser.TypeRegistry` (new interface)**. It's the parser's view of registered configuration types. `*registry.PluginRegistry` satisfies it. `ParserOptions` gains an optional `TypeRegistry` override that defaults to `PluginRegistry`, the same pattern `ProviderResolver` already uses.

```go
type TypeRegistry interface {
    IsRegisteredType(name string) bool
}
```

**`Querier[T]` (changed, public)**. `FindResourcesByType` takes the type name to match against `Meta.Type`. This is a breaking change, which is allowed because the library isn't released. Return types are unchanged.

```go
func (q *Querier[T]) FindResource(path string) (*T, error)                  // unchanged signature
func (q *Querier[T]) FindResourcesByType(typeName string) ([]*T, error)     // was FindResourcesByType()
```

**`gohcl.CheckBody` (new, in-repo HCL fork)**. Checks a body against the implied schema of `val`, recursing the way decode does, and returns diagnostics without evaluating any expression.

```go
func CheckBody(body hcl.Body, val any) hcl.Diagnostics
```

**Example types (new, package `example/resources`)**. The shared Go types that both example programs use. They embed `types.ResourceBase` and carry `xcl` and `json` tags. The json tags let the plugin example's JSON copy fill the same fields.

```go
type PostgreSQL struct {
    types.ResourceBase `xcl:",remain"`
    Location, DBName, Username, Password string; Port int
    Timeouts *Timeouts `xcl:"timeouts,block"`
    ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}
type App struct { // references postgres fields, a variable and a module output
    types.ResourceBase `xcl:",remain"`
    ...
}
```

There are no serialization-boundary changes. State, plugin wire data and querier copies still go through `encoding/json` (out of scope in the spec).


## Implementation Detail

**A third source of resource types.** The registry now resolves a type name from three sources, in a fixed order: builtin, registered, plugin. Registered types follow the existing builtin pattern exactly: a prototype pointer in a `RegisteredTypes` map, instantiated with `reflect.New`, and Meta name, type and properties set on creation. No new creation mechanism is introduced. Someone reading `CreateResource` sees three short lookups in sequence. Because every name is unique across the three sources (enforced by the clash check), the order never changes which source answers.

**One gate for names.** All four ways a name can enter the registry (`RegisterType`, `RegisterPlugin`, `RegisterPluginWithPath` and discovery, which goes through `RegisterPluginWithPath`) validate the incoming names through one private check before anything is stored. For plugins, the host is built first, because an in-process plugin only reports its types after `Init` and an external one only after it starts. The host is discarded (and, if external, stopped) when a name clashes, so a failed registration leaves the registry exactly as it was. A plugin that lists the same name twice within itself is also treated as a clash. Discovery collects clash errors separately from load errors and always returns them joined, while other load failures keep their existing "fail only if all fail" rule.

**"Handled without a provider" becomes one question.** The lifecycle and the destroy walk each have a hard-coded builtin check today. Both are replaced by a single helper that answers "builtin or registered", backed by the injected `TypeRegistry`. The branch body stays what builtins already run (a success event and no status change), so registered resources are indistinguishable from builtins in events and state. Plugin resources never reach the helper's true branch, so their path is byte-for-byte unchanged.

**Validation grows a schema check, following the decode shape.** The new fork function walks a body the way `decodeBodyToStruct` does (implied schema, `PartialContent` when a `remain` field exists and recursion into it with the leftovers, then recursion into each nested block's element type) but only collects diagnostics. It sits in the structure stage, next to the computed-field checks, and follows that stage's convention of reporting against the resource ID with the block's position. Every resource type is checked, including builtins (variable, output, module), whose bodies already decode against the same schema.

**Querier prefers the real value.** Both lookups first try a direct `*T` assertion on the stored resource and return it as is. Only a failed assertion falls back to the existing JSON copy. For registered types, a query therefore returns the same pointer the state holds. That matches "held and returned as the developer's own type", and it means mutating a query result mutates state, which the doc comments will say.

**Examples as testable programs.** Each example `main` is a thin wrapper around a `run(out io.Writer, dir string)` function that does all the work and returns the resources it found. Tests in the same package call `run` with a buffer and the shared config directory, then assert on the returned values and the printed output. The shared config sits in its own directory. `main` takes the config directory as an optional argument and defaults to the shared config next to the program, while tests pass the path explicitly. No example code shells out.

## Dependencies

- **`types` package (`RegisteredTypes`, `ResourceBase`, `GetMeta`)**: provides instantiation by `reflect.New` and metadata access, including through nested embedding. Used as is, with no change.
- **`plugins` package (`Plugin`, `PluginBase`, `RegisterResourceProvider`, `DirectPluginHost`, `GRPCPluginHost`, `DefaultChanged`)**: provides plugin hosts and their `GetTypes`, plus the building blocks for the plugin example. No change. The external host's `Stop` is used when a started plugin clashes.
- **`plugins/registry` (`PluginRegistry`, `PluginDiscovery`)**: changed. Gains type registration, the clash gate and `IsRegisteredType`.
- **`internal/parser` (lifecycle, callbacks, validate, parser options)**: changed. Gets the provider-less skip, the `TypeRegistry` injection and the schema-check stage.
- **`internal/xcl/gohcl` (in-repo HCL fork, MPL-2.0)**: changed. Gains `CheckBody`. It needs the MPL modification notice and an `UPSTREAM.md` entry.
- **`state` (`FileStateStore`)**: no change. It reloads registered types through `registry.CreateResource` and is used by the re-apply and removal tests.
- **`internal/schema` (`UnmarshalUntyped`)**: no change. It's still the querier's fallback copy for plugin types.
- **`plugins/example` build (`go build ./plugins/example`)**: no change. Discovery and external clash tests build it the way the existing discovery tests do.
- **testify (`require`, `mock`) and mockery**: existing test dependencies. No new mocks are needed, and there are no new third-party dependencies.
- **Spec `20260919120639-config-only-types-and-examples`**: the source of requirements. Final.
- **Prior work that must already be in place**: the provider lifecycle with Read/Changed/computed carry-over, and the validation phase before the walk. Both are already on the `v2` branch, so nothing has to land first.
- **Pending, not blocking**: XCL-owned JSON serialization using `xcl` tag names. This plan keeps `json` tags on the example types to bridge the gap until then.


## Testing Approach

**Test types.** Most coverage is unit tests at the registry level (registration, clashes, creation order) and in the fork (`CheckBody`). Integration tests with real `.xcl` fixtures on disk cover apply, re-apply, removal, validate, disabled blocks and modules at the parser level (where the recording logger and parser events are available), and querying through the public `xcl.Config` API. The two example programs get end-to-end tests that call each program's `run` function. Everything follows the house conventions: testify `require`, one behaviour per named test function, positive and negative cases in separate functions, no table-driven tests, and state for re-apply and removal produced by a real first apply with a file state store rather than a hand-written state file.

**Where coverage concentrates, and why.** The registry clash gate gets the densest coverage because four entry points share it and the spec calls out both clash directions. There's one test per direction per route: plain type vs builtin, plain vs plain, plain vs in-process plugin, in-process plugin vs plain, plugin vs plugin (in-process), and external or discovered plugin vs plain type (using the built example plugin). Each also asserts that the earlier registration still works. The lifecycle skip is covered by applying registered-only configurations with **no plugins registered**. The load-bearing assertion is that apply, re-apply and removal all succeed with no "no provider found" error, and that the status of each registered resource equals that of a builtin. Ordering is asserted by recording the order of parser "create" events: a dependent's event comes after the registered block it depends on.

**Load-bearing assertions, in plain language.**
- A registered type comes back from state as exactly the registered Go type. A type assertion succeeds, and `reflect.TypeOf` matches.
- Every configured value, nested block included, lands on that value, and references resolve both ways (variable, another resource's field, a module output, and a third block reading the registered block).
- Listing by type returns exactly the resources of that type, for a registered type and a plugin type in the same configuration.
- Validate rejects an undefined reference and an unknown attribute (for a registered type, and, per the user's decision, for a plugin type too), and names the block.
- A computed field on a registered type is accepted, and no warning is logged (checked through the recording test logger). The field stays empty after apply.
- A disabled registered block is reported disabled. A registered block inside a module lands in state under the module's path.
- Regression guard: the full existing suite (plugin lifecycle, builtins, discovery) passes unchanged, apart from the call sites updated for the `FindResourcesByType(typeName)` signature.

**Success metrics.**
- *"A developer can define, register and apply a type with no plugin or provider; the config-only example is the proof and contains no plugin or provider code."* **Behavioural test.** The config-only example test runs the program and asserts the declared resources are returned. A second test asserts that the config-only example package doesn't import `plugins` (through `go/parser` import inspection of its source files), so plugin or provider code can't creep in.
- *"Both examples run in the normal test suite, so a broken example fails the normal test run."* **Behavioural test.** The example tests are ordinary package tests under `example/`, picked up by the normal full test run. They fail when an example returns no resources or a set of resource IDs different from the one the configuration declares.
- *"The full test suite and vet checks pass, with no regression in plugin or builtin behaviour."* **Behavioural test.** Covered by running the complete existing suite and the vet checks at the end of every phase, and by the regression guard above.

**Deliberate gaps.** There are no new tests for `Destroy` on `Config` (still a TODO in the code and outside this spec). The destroy-walk skip gets one parser-level unit test that calls the destroy callback directly with a registered resource. No external-plugin tests are added beyond the discovery clash test, because `plugins/example` already covers external plugin behaviour.

## Milestones & Phases

### Milestone 1: Plain Go types can be used as configuration blocks without a plugin

**What changes**: A developer can register a plain Go type under a block type name on the plugin registry, then apply configuration that uses it with no plugin or provider at all. Blocks of that type are decoded into the developer's own type, take part in references and dependency ordering, work inside modules and when disabled, are saved to state and reloaded, and never trigger a provider call, whether on first apply, re-apply or removal. Type names are now guarded: registering a type or a plugin whose type name is already taken (by a builtin, a registered type or another plugin) fails with an error naming the type.

**Validation point**: Integration tests apply registered-only configurations with no plugins registered and succeed (first apply, re-apply from real state, removal). Resources come back as the exact registered type. Every clash direction is rejected with an error naming the type. The existing test suite and vet checks still pass.

#### - [x] Phase 1.1: Register plain types on the plugin registry and guard type names

**Repo:** xclconfig

The plugin registry learns to hold plain Go types under a block type name and to create new instances of them as the developer's own type. Every way a type name can enter the registry (a registered type, an in-process plugin, an external plugin, or discovery) goes through one clash check against builtins, registered types and every plugin's types. A failed registration leaves the registry unchanged.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-register-plain-types-on-the-plugin-registry-and-guard-type-names)

**Acceptance criteria**:
- [x] A plain Go type registers on an otherwise empty registry with no plugins, and creating a resource of that type gives back an instance of exactly that Go type with its name and type set.
- [x] Registering a second type under a name already registered fails with an error naming it, and the first type still creates correctly.
- [x] Registering a type named `variable`, `output`, `module` or `root` fails with an error naming it.
- [x] Registering a type that a registered plugin already provides fails with an error naming it.
- [x] Registering an in-process plugin, an external plugin, or discovering a plugin that provides an already-registered type fails with an error naming it, and the clashing plugin is not added.
- [x] Registering a second plugin that provides the same type as an earlier plugin fails with an error naming the type.
- [x] A type with a computed field registers without error.
- [x] Registering something that isn't a pointer to a struct embedding the resource base fails with a clear error.

#### - [x] Phase 1.2: Apply registered types without any provider

**Repo:** xclconfig

The lifecycle and the destroy walk treat registered types like builtins. They're decoded, linked and saved, but no provider is ever called for them. The parser learns about registered types through a small interface the registry satisfies. This phase also proves the whole feature through the apply path: references, ordering, modules, disabled blocks, re-apply and removal, all with no plugins registered.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-apply-registered-types-without-any-provider)

**Acceptance criteria**:
- [x] A configuration with only builtin and registered blocks applies with no plugins registered and no "no provider found" error, and each registered resource reports the same status builtins get.
- [x] Every configured value of a registered block, nested block included, is present on the applied resource, and the resource is exactly the registered Go type.
- [x] A registered block reads a variable, another resource's field and a module output, a third block reads a field of the registered block, and every referencing field holds the referenced value.
- [x] A block that depends on a registered block is processed after it.
- [x] Re-applying unchanged configuration against the state saved by the first apply succeeds with no provider error.
- [x] Re-applying after a registered block is removed from configuration succeeds with no provider error.
- [x] A disabled registered block is reported as disabled, the same as a disabled plugin resource.
- [x] A registered block inside a module lands in state under the module's path.
- [x] A registered type with a computed field applies with no warning logged, and the computed field stays empty.
- [x] Every existing plugin and builtin behaviour is unchanged.

### Milestone 2: Querying by type and validating attributes work for every resource type

**What changes**: Listing resources by type through the typed query API now actually returns them. The caller names the type, and gets exactly the resources of that type, for registered and plugin types alike. Registered types come back as the stored value itself, with no copy. Validating a configuration now catches an attribute or block that a resource's type doesn't have, for every resource type, so the mistake is reported before anything is applied, not partway through an apply.

**Validation point**: Tests list a registered type and a plugin type from one configuration and get exactly their own resources. Validate fails with an error naming the block for an unknown attribute, on both a registered and a plugin type, and still accepts every existing valid fixture. The full test suite and vet checks pass.

#### - [x] Phase 2.1: List resources by type and return registered types as themselves

**Repo:** xclconfig

The typed querier's list-by-type is fixed. The caller names the type, and gets exactly the resources of that type. Both query methods return a registered resource as the stored value itself, and fall back to the existing copy for plugin types.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-list-resources-by-type-and-return-registered-types-as-themselves)

**Acceptance criteria**:
- [x] With several resources of one registered type and one plugin type in a configuration, listing by each type returns exactly that type's resources and no others.
- [x] Finding a registered resource by its path returns the value held in state, as the registered Go type, without conversion.
- [x] Finding and listing plugin resources still return filled-in copies, as before.
- [x] Listing a type with no resources returns a not-found error.

#### - [x] Phase 2.2: Reject attributes a resource's type doesn't have at validation time

**Repo:** xclconfig

Validation gains a schema check of every resource body against its type, using a new helper in the in-repo HCL fork that follows the decoder's rules without evaluating anything. An unknown attribute or block is reported, naming the resource, before anything is applied, for registered and plugin types alike.

*Technical detail:* [context.md#phase-22](./context.md#phase-22-reject-attributes-a-resources-type-doesnt-have-at-validation-time)

**Acceptance criteria**:
- [x] Validating a configuration where a registered block sets an attribute its type doesn't have fails with an error naming the block.
- [x] Validating a configuration where a plugin block sets an attribute its type doesn't have fails the same way.
- [x] Validating a configuration where a registered block references something undefined fails with an error naming the block.
- [x] A correct configuration with registered blocks validates successfully.
- [x] Every existing valid configuration in the test suite still validates and applies.
- [x] The HCL fork change carries its licence notices and is recorded in the fork's upstream notes.

### Milestone 3: A working, tested example for both configuration-only and plugin use

**What changes**: The stale example is replaced by one shared configuration and one shared set of Go types, used by two small runnable programs. One parses the configuration with registered types only. The other uses an in-process plugin whose provider fills in a connection string. Both print their resources and a query result. Their tests run as part of the normal test suite, so the example can't silently stop finding resources again.

**Validation point**: Both example programs run successfully from the command line. Their tests assert the exact set of declared resources, the query result, and the connection-string difference between the two modes. The config-only example is checked to contain no plugin code. The full test suite and vet checks pass.

#### - [x] Phase 3.1: Shared example configuration and the configuration-only example

**Repo:** xclconfig

The stale example is replaced by a shared `.xcl` configuration (root plus a local module) and a shared Go types package, and a configuration-only program that registers the types, applies the configuration and prints the resources and a query result. Tests run the program's logic and check the exact resources it finds.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-shared-example-configuration-and-the-configuration-only-example)

**Acceptance criteria**:
- [x] Running the configuration-only example exits successfully and prints every resource the configuration declares plus at least one query result.
- [x] The example's tests fail if it finds no resources or a different set from the one the configuration declares.
- [x] The configuration-only example contains no plugin or provider code, and a test enforces that.
- [x] The old `.hcl` example files and unregistered types are gone.

#### - [x] Phase 3.2: Plugin example on the same configuration and types

**Repo:** xclconfig

A second program uses the same configuration and Go types through an in-process plugin whose provider fills the computed connection string. Its tests check the same resource set, and check that the connection string is filled here and empty in the configuration-only run. The docs point at both examples and describe type registration.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-plugin-example-on-the-same-configuration-and-types)

**Acceptance criteria**:
- [x] Running the plugin example on the shared configuration exits successfully and lists the same resources as the configuration-only example.
- [x] The connection string has a value in the plugin example's output and is empty in the configuration-only output.
- [x] The plugin example defines no configuration or resource types of its own.
- [x] The documentation describes registering a plain type and points at both examples.
- [x] The full test suite and vet checks pass with both examples included.

## Open Questions

- **Does the new schema check reject any existing fixture that is only ever validated, never applied?** It depends on which fixtures hold attributes that decode would reject but that no test ever decodes. That only shows up by running the full suite with the check in place. **When hit:** STOP and report the fixtures affected. Don't silently loosen the check or edit the fixtures.
- **Does the plugin example's generated look-alike copy back into the shared Go type with every field filled, `connection_string` included?** It depends on the schema generator carrying the `json` tags through `reflect.StructOf`, and on `schema.UnmarshalUntyped` honouring them. That's only confirmed by running the plugin example end to end. **When hit:** if fields come back empty, STOP and ask. The fix belongs to the pending XCL serialization work, and this plan shouldn't absorb it.
- **Do registered types survive a state save and reload with every field intact?** It depends on `encoding/json` round-tripping the user's type (including nested pointer blocks) through `FileStateStore`. The re-apply test exercises it. **When hit:** if values are lost, STOP and ask rather than adding custom serialization.


## Out of Scope

- **Returning real Go types for plugin resources.** In-process and external plugin types are still created as schema-generated look-alikes and reach callers through the querier's copy (spec Non-Goals).
- **XCL-owned serialization.** State, plugin wire data and querier copies keep using `encoding/json`, and the example types carry `json` tags to bridge the gap. Serialization based on `xcl` tags with implicit omitempty is separate pending work, to be captured in its own spec.
- **Plugin discovery, loading and start-up.** Unchanged apart from rejecting clashing type names (spec Non-Goals).
- **An external, out-of-process plugin in `example/`.** `plugins/example` already covers that route (spec Non-Goals).
- **Erroring on non-`.xcl` files.** Files without the extension stay silently ignored, even when passed by path (spec Constraints).
- **Destroying removed resources during Apply, and `Config.Destroy`.** Apply doesn't run the destroy walk today and `Config.Destroy` is a TODO. This plan only makes the destroy walk skip providers for registered types, and doesn't wire destruction into Apply.
- **Custom functions (`random_number()`) and `count` in the example.** The rewritten example drops them because the current parser doesn't support them. Restoring them is not part of this work.
- **A typed collection helper (follow-up).** This plan only fixes `Querier[T].FindResourcesByType(typeName)` and returns registered types directly. A helper that returns a typed collection straight away (for example, every resource of a Go type as `[]*T`, without the caller passing a type name) is worth looking at separately. It should be captured as its own follow-up rather than added here.
- **Skipping computed-field validation for registered types.** Registered types get the same computed-field validation as plugin types. Computed fields are accepted at registration and never warned about, but configuration still may not set them.

## Changelog


### 2026-09-19 — Phase 1.1: Register plain types on the plugin registry and guard type names

**What was done**: `PluginRegistry` gained `RegisterType` and `IsRegisteredType`, and `CreateResource` now resolves builtin, then registered (a real Go instance through `reflect.New`), then plugin. Every route a type name can enter by (`RegisterType`, `RegisterPlugin`, `RegisterPluginWithPath` and discovery) now goes through one clash check, which returns a typed `TypeNameClashError`. A clashing plugin host is stopped and not added, and discovery always returns clash errors.

**Deviations**: None. `TypeNameClashError.Existing` also takes the value "the same plugin" for a plugin that lists a type twice. `gofmt` fixed an import-order problem that was already in `plugin_registry.go`.

**Files changed**:
- `plugins/registry/errors.go`
- `plugins/registry/plugin_registry.go`
- `plugins/registry/plugin_registry_test.go`
- `plugins/registry/plugin_discovery_test.go`

**Discoveries**: Loading two copies of the same external plugin used to succeed silently, with the first host winning. It is now rejected as a clash. `types.GetMeta` panics when given a pointer to a non-struct or a nil pointer, so callers must check the kind first. `destroyWalkCallback` has no callers at all today. About 20 files in the repo already fail `gofmt` because of import order after the cty embedding.

### 2026-09-19 — Phase 1.2: Apply registered types without any provider

**What was done**: The parser gained a one-method `TypeRegistry` interface and a `ParserOptions.TypeRegistry` option, which defaults to the plugin registry. A single `handledWithoutProvider` helper replaces the hard-coded builtin checks in the lifecycle and the destroy walk, so registered types are handled like builtins: a success event, no status change, and no provider call. Integration tests with real `.xcl` fixtures and no plugins cover decoding, the exact Go type, references, module outputs, ordering, re-apply, removal, disabled blocks, modules and computed fields.

**Deviations**: None.

**Files changed**:
- `internal/parser/callbacks.go`
- `internal/parser/lifecycle.go`
- `internal/parser/parser.go`
- `internal/parser/registered_types_test.go`
- `internal/test_fixtures/registered/types.go`
- `internal/test_fixtures/config/registered/` (`basic`, `disabled`, `removed/before`, `removed/after`, `invalid_reference`, `invalid_attribute`)

**Discoveries**: Registered types survive a `FileStateStore` save and reload through `encoding/json` with every field intact, including nested pointer blocks and `depends_on`, which answers one of the plan's open questions. Disabled resources get no parser event and an empty status. `ParserOptions.TypeRegistry` only falls back to `PluginRegistry` when the registry is non-nil, to avoid a nil pointer inside a non-nil interface.

### 2026-09-19 — Phase 2.1: List resources by type and return registered types as themselves

**What was done**: `Querier[T].FindResourcesByType` now takes the type name and matches it against each resource's `Meta.Type`, so listing by type works for registered and plugin types. Both query methods go through a shared `asType` helper. It returns the stored value directly when it already is a `*T` (registered types), and otherwise copies it through `schema.UnmarshalUntyped` into a new `T`, which fixes the old nil `*T` fallback.

**Deviations**: `example/main.go` was changed to call `FindResourcesByType("postgres")` so the build stays green until Phase 3.1 deletes it. `querier.go` no longer imports `fmt`.

**Files changed**:
- `querier.go`
- `querier_test.go`
- `internal/test_fixtures/config/query/main.xcl`
- `example/main.go`

**Discoveries**: Plugin resources are held in state as generated types, never as the plugin's Go type, so the copy path is still needed for them. The not-found error from `FindResourcesByType` now carries the type name.

### 2026-09-19 — Phase 2.2: Reject attributes a resource's type doesn't have at validation time

**What was done**: The in-repo HCL fork gained `gohcl.CheckBody`, which walks a body against a Go value's implied schema the way `DecodeBody` does (remain fields, nested and repeated blocks) without evaluating anything. It reports only attributes and blocks the type doesn't have. `validateStructure` runs it for every resource type and reports each problem against the resource ID at the attribute's position, so an unknown attribute now fails at `Validate` instead of partway through `Apply`.

**Deviations**: The plan had `CheckBody` report everything decode would. The first version did, and `TestValidateRejectsNonOptionalComputedField` then got a duplicate "Missing required argument" error, which the plan's open question said to stop on. The user chose option A: report only unknown attributes and blocks. Missing required attributes and blocks, and duplicate blocks, are still reported by decode at apply time, so disabled blocks that leave out required values keep validating. No existing test or fixture was changed.

**Files changed**:
- `internal/xcl/gohcl/check.go`
- `internal/xcl/gohcl/check_test.go`
- `internal/xcl/UPSTREAM.md`
- `internal/parser/validate.go`
- `internal/parser/validate_test.go`
- `internal/test_fixtures/config/unknown_attribute/network.xcl`
- `internal/test_fixtures/config/registered/disabled_missing_required/main.xcl`

**Discoveries**: The HCL fork's root package is imported as `github.com/jumppad-labs/xcl/internal/xcl` but its package name is `hcl`. `hcl.Body.Content` itself reports missing required attributes, so a check limited to unknown attributes has to clear `Required` on the implied schema. Validation that also reported missing values would break disabled blocks, because they are never decoded.

### 2026-09-19 — Phase 3.1: Shared example configuration and the configuration-only example

**What was done**: The stale `.hcl` example was replaced with a shared `.xcl` configuration in `example/config` (two variables, two postgres blocks, one with a nested `timeouts` block and one ordered with `depends_on`, an app that reads a variable, a postgres field, a computed field and a module output, a local `analytics` module and an output). The shared Go types live in `example/resources`. The `example/configonly` program registers the types with `RegisterType`, applies the configuration with no plugin, and prints every resource plus the results of querying by type and by path. Its logic is in a `run(out, dir)` function that `main` and the tests share.

**Deviations**: `PostgreSQL` has no `DBCommon` embedding and no `id` field. `App` has a plain (not computed) `connection_string` field that reads the postgres computed field, which makes the difference between the two modes show up on a second resource. The module declares its own `db_username` variable instead of receiving values from the root.

**Files changed**:
- `example/main.go`, `example/types.go`, `example/config.hcl`, `example/modules/db/db.hcl` (deleted)
- `example/config/main.xcl`
- `example/config/modules/db/db.xcl`
- `example/resources/resources.go`
- `example/configonly/main.go`
- `example/configonly/main_test.go`

**Discoveries**: `Config` without a state store writes nothing to disk for local modules, so the old `os.RemoveAll(".xclconfig")` isn't needed. `GetResources` returns resources in no fixed order, so tests sort IDs before comparing. A module's own variables and outputs show up as resources (for example `module.analytics.variable.db_username`), so the declared set has 10 IDs.

### 2026-09-19 — Phase 3.2: Plugin example on the same configuration and types

**What was done**: `example/plugin` applies the shared configuration and Go types through an in-process `ExamplePlugin`. Its postgres provider fills in the computed `connection_string` on Create and Update, and a no-op provider handles `app`. It shares the `run(out, dir)` shape with the config-only example. Its tests check the same 10 resource IDs, the filled connection strings (including where `app.web` reads one), that resources are held as generated types, and that the package declares no resource types and holds no `.xcl` files. The README example section, which described a removed API, was rewritten for `RegisterType`, the clash rule and the querier. `docs/plugins.md` gained a "Configuration-only types" section, and `docs/README.md` describes the new example layout.

**Deviations**: The `CHANGELOG.md` entry was added during the feature-changelog step rather than in this phase, as a single prose record in the file's existing style.

**Files changed**:
- `example/plugin/main.go`
- `example/plugin/main_test.go`
- `README.md`
- `docs/plugins.md`
- `docs/README.md`
- `CHANGELOG.md`

**Discoveries**: A plugin's generated types copy back into the shared Go types through `schema.UnmarshalUntyped` with every field filled in, including the computed `connection_string`, when the types carry `json` tags that match their `xcl` names. That answers the plan's second open question. A block that reads another block's computed field gets the provider's value in plugin mode and an empty string in config-only mode.
