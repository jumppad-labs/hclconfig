---
created_date: "2026-09-19"
document_status: final
closed_date: "2026-09-19"
---

# Feature: 20260919120639-config-only-types-and-examples

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Developers embedding XCL can currently only use their own configuration types by writing a plugin with a provider that manages each resource's lifecycle, even when they just want to read typed configuration. This feature lets a developer register a plain configuration type directly. Blocks of that type are parsed, validated, linked to other blocks and returned as the developer's own type, with no plugin and no lifecycle calls. The bundled example is rewritten to show both routes on the same configuration: first plain typed configuration parsing, then the same setup with a full plugin. Developers get a working starting point for either use, and the example is checked by tests so it can't silently go stale again.

<!--
  REQUIREMENTS
  Specific, testable behaviours the feature must deliver.
  Format: bold title on the checkbox line, detail indented below.
  Rules:
    - Use active voice: "Users can...", "The system must..."
    - Each requirement should be independently verifiable
    - Focus on WHAT, not HOW — avoid prescribing implementation
    - Keep each item atomic — one behaviour per line
-->
## Requirements

- [ ] **Register a configuration type without a plugin**
  Developers can register a plain Go type under a block type name without writing a plugin or a provider.

- [ ] **Registered types are parsed from configuration**
  The system must parse resource blocks whose type name matches a registered type, decoding their attributes and nested blocks into that type.

- [ ] **Registered types take part in references**
  Blocks of a registered type can reference variables, other resources and module outputs, and other blocks can reference their fields. Dependency ordering applies to them as it does to any other resource.

- [ ] **No lifecycle calls for registered types**
  The system must not attempt to create, read, update, detect changes on or destroy resources of a registered type, and applying configuration that contains them must succeed without any provider.

- [ ] **Registered types are returned as the developer's own type**
  After applying, resources of a registered type are held and returned as instances of the Go type the developer registered, not a generated look-alike.

- [ ] **Registered types are included in state and queries**
  Resources of a registered type appear in the resulting state and can be found by querying on their path, the same way as plugin resources.

- [ ] **Registered types work with the other existing features**
  Features that work for plugin resources, such as disabled blocks, modules and validating without applying, work the same way for registered types.

- [ ] **Type name clashes are rejected when a type is registered**
  Registering a type must fail with an error that names the clash if the name is already registered as a type, is provided by a registered plugin, or is a builtin block name.

- [ ] **Type name clashes are rejected when a plugin is registered**
  Registering or discovering a plugin must fail with an error that names the clash if the plugin provides a type name that is already registered as a type or provided by another plugin.

- [ ] **Computed fields on registered types are accepted**
  Developers can register a type that has fields marked as computed. The system must neither warn about nor reject such a type.

- [ ] **The example shows configuration-only use**
  The repository includes a runnable example that parses a configuration using only registered types, then prints and queries the resulting resources.

- [ ] **The example shows full plugin use**
  The repository includes a runnable example that uses the same configuration and the same Go types with an in-process plugin, and shows lifecycle behaviour the configuration-only mode does not have, such as a field filled in by the provider.

- [ ] **The examples are checked by tests**
  Automated tests run both examples and fail if either finds no resources or produces the wrong resources.

- [ ] **Querying by type returns every resource of that type**
  Developers can list every resource of a given type through the query API, for registered types and plugin types alike. Listing by type does not return any resources today, and the examples rely on it.

<!--
  CONSTRAINTS
  Hard boundaries the solution must operate within. These are non-negotiable.
  Format: one bullet point per constraint.
  Examples:
    - Must integrate with the existing authentication system
    - Cannot introduce breaking changes to the public API
    - Must support the current minimum supported runtime versions
  Leave blank if there are no constraints.
-->
## Constraints

- **Type registration goes wherever plugins are registered.** It must not be a separate configuration option. Source: user, "It should be wherever plugins are registered."
- **Both example modes use the same configuration and the same Go types.** The plugin example must not have its own config or type definitions. Source: user, "the same example but with full plugins."
- **Files without the `.xcl` extension continue to be ignored,** including a file passed explicitly by path. No error or warning is added. Source: user, "keep the convention where we ignore non .xcl files."
- **Tests follow the repository's testing rules:** testify `require`, no table-driven tests, and positive and negative cases in separate test functions. Source: repository conventions (CLAUDE.md).
- **Existing plugin and builtin behaviour must not change** beyond rejecting type name clashes and fixing listing by type. Source: user, confirmed during spec review.
- **Breaking changes to the public API are acceptable.** The library has not been released, so existing registry and query behaviour can change without a compatibility layer. Source: user, "don't worry about things breaking, we have not released this library", confirmed for this spec.

<!--
  ACCEPTANCE CRITERIA
  The specific, binary conditions that define "done".
  Format: bold title on the checkbox line, verifiable detail indented below.
  Each criterion must be:
    - Independently verifiable (pass/fail, not subjective)
    - Traceable back to a requirement above
    - Testable by someone who didn't write the code
-->
## Acceptance Criteria

- [ ] **A type registers without a plugin**
  Given a plain Go type and an otherwise empty registry, with no plugins registered, registering the type under a name succeeds without an error.

- [ ] **Registered blocks decode into their type**
  Given a configuration with a block of a registered type that sets attributes and a nested block, applying it succeeds, and the resulting resource holds every configured value, nested block included.

- [ ] **References resolve in both directions**
  Given a configuration where a registered-type block reads a variable and a field of another resource, and a third block reads a field of the registered-type block, applying it succeeds and each referencing field holds the referenced value.

- [ ] **Registered blocks can read module outputs**
  Given a registered-type block that reads an output of a module, applying succeeds and the field holds the module's output value.

- [ ] **Dependents are processed after registered blocks**
  Given a block that depends on a registered-type block, the dependent block is processed after the registered-type block it depends on.

- [ ] **Apply succeeds with no provider**
  Applying a configuration that contains only builtin blocks and registered-type blocks succeeds with no plugins registered. No "no provider found" error is returned, and each registered-type resource reports the same status that builtin resources get after an apply.

- [ ] **Re-applying unchanged configuration succeeds without a provider**
  Applying the same configuration a second time, with the state from the first apply, succeeds with no plugins registered and no provider error.

- [ ] **Removing a registered block succeeds without a provider**
  Re-applying after a registered-type block has been removed from the configuration succeeds with no plugins registered and no provider error.

- [ ] **Resources come back as the registered type**
  After applying, looking a registered-type resource up in the state returns a value whose Go type is exactly the registered type, and a type assertion to it succeeds without any conversion.

- [ ] **Registered resources are in state and queryable**
  After applying, the state contains every registered-type resource from the configuration. Finding one by its path returns it, and listing by its type returns all of them.

- [ ] **Disabled blocks are skipped**
  Given a registered-type block marked disabled, applying succeeds and the resource is reported as disabled, the same as a disabled plugin resource.

- [ ] **Registered types work inside modules**
  Given a module that contains a registered-type block and is used by the root configuration, applying succeeds and the module's resource is in state under the module's path.

- [ ] **Valid configuration with registered types validates**
  Validating a correct configuration with registered-type blocks succeeds.

- [ ] **Invalid registered-type block fails validation**
  Validating a configuration where a registered-type block references something undefined, or sets an attribute its type does not have, fails with an error naming the block.

- [ ] **Duplicate type registration is rejected**
  Registering a second type under a name that is already registered returns an error that includes the name, and the first registration is unaffected.

- [ ] **Registering a builtin name is rejected**
  Registering a type named `variable`, `output`, `module` or `root` returns an error that includes the name.

- [ ] **Registering a type a plugin provides is rejected**
  With a plugin registered that provides type `X`, registering a plain type named `X` returns an error that includes `X`.

- [ ] **Registering a plugin that clashes with a type is rejected**
  With a plain type registered as `X`, registering or discovering a plugin that provides `X` returns an error that includes `X`.

- [ ] **Two plugins providing the same type name are rejected**
  With a plugin registered that provides type `X`, registering or discovering a second plugin that also provides `X` returns an error that includes `X`.

- [ ] **A type with computed fields registers silently**
  Registering a type with a field marked computed succeeds. Applying a configuration that uses it succeeds with no warning logged, and the computed field stays at its zero value.

- [ ] **The configuration-only example runs**
  Running the configuration-only example exits successfully, and its output lists every resource declared in its configuration along with the result of at least one query.

- [ ] **The plugin example runs**
  Running the plugin example on the same configuration exits successfully. Its output lists the same resources, and the field the provider fills in has a value, where in the configuration-only output it is empty.

- [ ] **Example tests catch a broken example**
  The test suite includes tests for both examples, and they fail if an example returns no resources or a different set of resources than its configuration declares.

- [ ] **Querying by type lists all matching resources**
  Given a configuration with several resources of one registered type and one plugin type, listing by each type returns exactly the resources of that type and no others.

<!--
  TECHNICAL APPROACH
  High-level technical direction to guide the planning agent. Include:
    - Key architectural decisions already made
    - Preferred patterns or technologies if known
    - Integration points with existing systems
    - Known risks or areas of uncertainty
  Format: one bullet point per direction/steer.
  Leave blank if you want the planner to propose the approach.
-->
## Technical Approach

- **Create registered types as real Go types, the way builtins are.** Each block gets a new instance of the registered type rather than a look-alike generated from a schema, so no conversion is needed to get the developer's type back.
- **Treat registered types like builtins in the lifecycle.** Rather than giving them a no-op provider, the lifecycle recognises a registered type and skips provider calls for it. A no-op provider "shim" was considered and rejected, because it would still produce generated look-alike types and still run the provider calls.
- **Check type names in one place.** Whichever way a name arrives (a registered type, an in-process plugin, an external or discovered plugin), it is checked against builtins, registered types and every plugin's types, so both clash directions give the same error.
- **One small program per example mode.** The plugin example's provider fills in a computed field, so the output shows the difference between the modes directly.
- **Rename the example's configuration files to `.xcl`.**
- **Known risk: querying depends on JSON copying.** The query API copies each resource into the caller's type through JSON, which depends on `json` tags. Registered types return the real type and avoid it, but plugin types still depend on it. This ties in with the separate, pending work on XCL's own serialization.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- A developer can define a Go type, register it and apply a configuration that uses it without writing a plugin or a provider. The configuration-only example is the proof and contains no plugin or provider code.
- Both examples run in the normal test suite, so any change that breaks either one fails `go test ./...` and the example can not go stale unnoticed again.
- The full test suite and `go vet` pass after the change, with no regression in existing plugin or builtin behaviour.

<!--
  NON-GOALS
  Explicitly state what this spec does NOT cover. This is as important as
  the requirements — it prevents scope creep and sets clear expectations.
  Format: one bullet point per exclusion.
  Examples:
    - "Mobile support is out of scope (tracked in #456)"
    - "Internationalisation will be addressed in a follow-up spec"
  Leave blank if there are no explicit exclusions to call out.
-->
## Non-Goals

- Plugin types, in-process or external, keep being created as schema-generated look-alikes. Returning real Go types for plugin types is not part of this work.
- How resources are copied into the caller's type, written to state and sent to plugins stays as it is. XCL's own serialization based on `xcl` tags is separate follow-up work.
- How plugins are discovered, loaded and started is unchanged, apart from rejecting type names that clash.
- The rewritten example does not show an external, out-of-process plugin; `plugins/example` already covers that.
