# Feature: 20260714080036-restore-module-support

## Overview

xclconfig configurations can currently only be authored as a single flat set of resources. This feature restores the ability to package and reuse configuration as modules — self-contained, named units of configuration that can be instantiated multiple times with different variables — so users can share and reuse common infrastructure patterns (for example, a standard container setup) across multiple parts of a configuration without duplicating it.

## Requirements

- [x] **Define reusable configuration modules**
  Users can define a named module that references a source directory of configuration, so that a set of resources can be authored once and reused.

- [x] **Instantiate a module multiple times**
  Users can instantiate the same module source multiple times under different names within a single configuration, with each instance producing its own independent set of resources.

- [x] **Scope resources per module instance**
  Resources created by a module instance are uniquely identifiable and addressable per instance, so that two instances of the same module source never collide or overwrite each other's resources.

- [x] **Pass variables into a module instance**
  Users can supply variable values to a module instance, and those values are used when evaluating the module's configuration, overriding the module's own variable defaults.

- [x] **Preserve variable default fallback**
  When a module instance does not supply a value for one of the module's variables, the module's own default value is used.

- [x] **Resolve local module sources**
  Users can reference a module's configuration using a local relative path, and the system locates and parses that configuration as part of the module instance.

- [x] **Disable a module instance**
  Users can mark a module instance as disabled, and doing so is reflected on the resources it produces without removing those resources from the configuration's state.

- [x] **Allow a child resource to override module-level disabled state**
  A resource within a module can independently determine its own disabled state (for example, based on a variable), even when the module instance itself is not disabled.

- [x] **Interpolate values across module boundaries**
  Configuration inside a module can reference the module's own variables and resources, and values produced by a module instance (such as outputs) can be referenced from outside the module.

- [x] **Expose module outputs**
  Users can define outputs within a module, and those outputs are queryable per module instance from the resulting configuration state.

- [x] **Resolve dependencies correctly regardless of authoring order**
  Resources and variables inside and across modules are evaluated in an order that respects their dependencies, regardless of the order in which they are declared in configuration files.

- [x] **Support nested modules**
  A module can itself contain one or more module instances, and resources within a nested module instance are correctly scoped, evaluated, and addressable, matching prior supported behavior.

## Constraints

- Module resources must be decoded through the existing DAG-ordered walk phase, not eagerly or inline during file parsing — the parser's two-phase parse-then-decode model must not be bypassed or reintroduced as single-pass decoding for modules.
- All currently-passing (non-module) parser tests must continue to pass unmodified.
- The on-disk/serialized state format must remain compatible — the State structure's JSON serialization must not change shape as a result of this work.
- No new external dependencies may be introduced; the solution must be built using libraries already present in go.mod.
- Local relative-path module sources only — remote or URL/git-based module sources are explicitly out of scope for this work.

## Acceptance Criteria

- [x] **Module block is parsed without error**
  Given a configuration containing a module definition that points at a local source directory, parsing that configuration completes successfully with no errors.

- [x] **Resources from a module instance appear in state**
  After parsing a configuration containing a module instance, the resources declared in that module's source configuration are present and retrievable from the resulting state, addressed by a path that includes the module instance's name.

- [x] **Multiple instances of the same module produce independent resources**
  Given a configuration with two or more module instances using the same source but different instance names, resources for each instance are independently retrievable, and changing or inspecting one instance's resources does not affect another instance's resources.

- [x] **Repeated instantiation does not leak state between instances**
  Given a configuration with multiple instances of the same module source, values or fields specific to one instance (for example, an interpolated value) do not appear on another instance's resources.

- [x] **Supplied variable value is reflected in the module's resources**
  Given a module instance that supplies a value for one of the module's variables, a resource inside that module instance that uses the variable reflects the supplied value, not the module's own default.

- [x] **Unsupplied variable falls back to the module's default**
  Given a module instance that does not supply a value for one of the module's variables, a resource inside that module instance that uses the variable reflects the module's own default value.

- [x] **Disabling a module instance is reflected on its resources**
  Given a module instance marked as disabled, its resources are still retrievable from state, and each reflects a disabled outcome.

- [x] **A resource inside a module can independently override disabled state**
  Given a module instance that is not disabled, and a resource inside it whose own disabled state depends on a variable, that resource's disabled outcome matches what the variable dictates, independent of the module's own disabled state.

- [x] **A value from one module resource can be referenced by another resource in the same module**
  Given a resource inside a module that references another resource or variable declared in the same module, the referencing resource's evaluated value reflects the referenced value correctly.

- [x] **A module output is retrievable per instance**
  Given a module that declares an output, and one or more instances of that module, each instance's output value is independently retrievable from the resulting state.

- [x] **Parsing succeeds regardless of declaration order**
  Given a configuration where a resource, variable, or module instance depending on another is declared before or after what it depends on in the source file, parsing still completes successfully and produces correctly evaluated values.

- [x] **Resources inside a nested module are correctly scoped and retrievable**
  Given a module instance that itself contains another module instance, resources declared inside the nested module instance are retrievable from state, addressed by a path that reflects both the outer and inner module instance names.

## Technical Approach

- Prefer building on the existing dependency-resolution and resource-addressing infrastructure (the dependency graph, resource path/identity scheme, and dependency-link extraction already used for non-module resources) rather than introducing a parallel mechanism specifically for modules.
- This is framed as a restoration of previously-working functionality, redesigned to fit the current parse-then-decode architecture, rather than a net-new feature — prior behavior (as previously supported) is a useful reference point for the design.
- Beyond this, no further technical direction has been decided; the detailed design is left for the plan workflow to propose.

## Success Metrics

- All currently-failing module-related parser tests pass.
- The full parser test suite passes with no regressions introduced.

## Non-Goals

- Module version/update management (resolving, pinning, or upgrading a module's source content by version) is out of scope.
- Changes to provider/plugin lifecycle execution behavior (how Create/Update lifecycle calls interact with module resources during actual execution) are out of scope — this work covers parsing and state population only.
