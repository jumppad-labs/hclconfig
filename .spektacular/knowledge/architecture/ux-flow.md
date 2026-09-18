# XCL User Experience Flow

## Overview

This document describes how end users interact with the XCL package to parse and manage configuration.

## Entry Point

The main entry point for end users is the `Config` type in the root `xcl` package.

```go
import "github.com/jumppad-labs/xcl"
```

## Basic Usage Flow

### 1. Create a Config

```go
config := xcl.NewConfig(
    xcl.WithStateStore(stateStore),
    xcl.WithPlugin(myPlugin),
    xcl.WithVariables(map[string]any{"env": "production"}),
)
```

### 2. Apply Configuration

Parse and apply configuration files, executing plugin lifecycle methods:

```go
err := config.Apply("./config/main.xcl")
if err != nil {
    log.Fatal(err)
}
```

### 3. Query Resources

Use the type-safe Querier to find resources:

```go
q := xcl.NewQuerier[MyResourceType](config)

// Find a specific resource by path
resource, err := q.FindResource("resource.mytype.name")

// Find all resources of a type
resources, err := q.FindResourcesByType()
```

### 4. Validate Without Applying

Check whether a configuration is valid, without acting on it. Validation
creates, changes and removes nothing, decodes no resource bodies and reaches
no provider:

```go
err := config.Validate("./config/main.xcl")
if err != nil {
    log.Fatal(err)
}
```

A nil error means the configuration is valid. Otherwise the returned error is
an `*errors.ConfigError` collecting **every** problem found, each naming what
is wrong and the file and position it occurs at:

```go
var ce *errors.ConfigError
if errors.As(err, &ce) {
    for _, problem := range ce.Errors {
        fmt.Println(problem)
    }
}
```

Validation runs three stages in order, and each reports everything it finds
before the next is considered:

1. **Structure** — configuration too malformed to check any further.
2. **References** — anything a resource refers to must be defined somewhere
   in the configuration. Resolution spans files and reaches into modules.
3. **Properties** — a reference's trailing property path must name properties
   the referenced type actually has.

A later stage is skipped when an earlier one found problems, since checking
properties on a reference that resolves nowhere would only report consequences
of a problem already reported.

`Apply` runs the same validation first and refuses to proceed if it fails, so
nothing is created, changed or removed unless the configuration is valid.

### 5. Destroy Resources

Remove all resources in state:

```go
err := config.Destroy()
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     End User Code                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  config := xcl.NewConfig(...)                               │
│  config.Apply("config.xcl")                                 │
│  q := xcl.NewQuerier[T](config)                             │
│  resource, _ := q.FindResource("resource.type.name")        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    xcl Package (Public API)                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Config          - Main orchestrator                         │
│  Querier[T]      - Type-safe resource queries               │
│  ConfigOption    - Functional options for Config            │
│  ConfigError     - Collects every problem found by validation │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Internal Packages                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  internal/parser    - HCL parsing, DAG building             │
│  internal/schema    - Go/CTY type conversion                │
│  internal/resources - Built-in resource types               │
│  internal/functions - Built-in HCL functions                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Supporting Packages                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  state/            - State management and persistence        │
│  plugins/          - Plugin interfaces and registry         │
│  types/            - Resource base types and helpers        │
│  errors/           - Custom error types                      │
│  logger/           - Logging abstractions                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Config is the Only Entry Point

- Users never directly interact with Parser
- Parser is internal implementation detail
- Config orchestrates parsing, state management, and plugin execution

### 2. Querier Requires Config

- `Querier` works with `*Config`, not `*State`
- This maintains the abstraction - users don't need to know about State
- Querier provides type-safe access to parsed resources

### 3. State is Managed Internally

- State persistence is handled by `StateStore` interface
- Users configure storage via `WithStateStore()` option
- State transitions are managed by Config during Apply/Destroy

## Open Questions

1. **Should Querier accept an interface instead of concrete Config?**
   - Pro: More flexible, could work with State directly in tests
   - Con: Adds complexity, exposes internal details

2. **How should tests query resources without circular dependencies?**
   - Option A: External test package (`package parser_test`) imports both `internal/parser` and `xcl`
   - Option B: Querier accepts interface that both Config and State implement
   - Option C: Tests use Config wrapper around parser results

3. **What's the minimal interface for Querier?**
   ```go
   type ResourceProvider interface {
       GetResources() []any
   }
   ```
   Both `*Config` and `*State` already implement this.
