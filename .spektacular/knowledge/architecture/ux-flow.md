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

Preview changes without executing plugins:

```go
diff, err := config.Validate("./config/main.xcl")
if err != nil {
    log.Fatal(err)
}

// Inspect what would change
for _, create := range diff.Creates {
    fmt.Printf("Would create: %s\n", create.ID)
}
```

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
│  Diff            - Represents changes between states        │
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
