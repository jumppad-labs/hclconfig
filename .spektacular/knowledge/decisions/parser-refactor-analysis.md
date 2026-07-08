# Parser Refactoring Analysis - Complete Implementation Guide

**Date**: 2025-12-27
**Status**: Refactoring 95% Complete - Missing Type Definitions and Wiring
**Branch**: v2

## Executive Summary

The parser is undergoing a major architectural refactoring to separate concerns between Config, Parser, and State. The refactor is functionally complete but has compilation errors due to missing type definitions. This document provides a comprehensive analysis of the refactor and detailed implementation steps to complete it.

## Table of Contents

1. [Current Architecture](#current-architecture)
2. [The Missing `parsed` Type](#the-missing-parsed-type)
3. [Data Flow Architecture](#data-flow-architecture)
4. [Two-Phase Processing Model](#two-phase-processing-model)
5. [Why Two Separate Data Structures](#why-two-separate-data-structures)
6. [Implementation Status](#implementation-status)
7. [Required Fixes](#required-fixes)
8. [Design Decisions](#design-decisions)

## Current Architecture

### Component Responsibilities

**State** (`state/state.go`)
- Thread-safe resource registry
- Query interface (FindResource, FindResourcesByType, etc.)
- JSON serialization for persistence
- No HCL bodies (clean separation)

**Parser** (`internal/parser/parser.go`)
- Orchestrates parsing workflow
- Manages temporary parsing state
- Builds and executes DAG
- Returns clean State object

**parsed** (MISSING - internal to Parser)
- Temporary holding structure during parsing
- Stores both resources and their HCL bodies
- Required for two-phase decoding

## The Missing `parsed` Type

### Definition

The `parsed` type is referenced but never defined. It should be:

```go
// parsed holds resources and their HCL bodies during the parsing phase
// This is an internal data structure used only within Parser
type parsed struct {
    resources map[string]any              // FQRN -> resource instance
    bodies    map[string]*hclsyntax.Body  // FQRN -> raw HCL body for decoding
}
```

### Where It's Used

**File**: `internal/parser/parser.go`

1. **Initialization** (line 184-187):
```go
p.parsedResources = &parsed{
    resources: map[string]any{},
    bodies:    map[string]*hclsyntax.Body{},
}
```

2. **Resource Storage** (lines 501-502):
```go
p.parsedResources.bodies[rtMeta.ID] = b.Body
p.parsedResources.resources[rtMeta.ID] = rt
```

3. **Transfer to State** (line 212):
```go
for _, resource := range p.parsedResources.resources {
    currentState.AppendResource(resource)
}
```

4. **Callback Creation** (line 617 - COMMENTED OUT):
```go
// w.Callback = walkCallback(p.parsedResources, currentState, previousState, p.pluginRegistry, &p.options, functions)
```

**File**: `internal/parser/callbacks.go`

5. **Body Retrieval** (line 53):
```go
bdy, ok := currentState.bodies[rMeta.ID]
```

6. **Context Building** (line 59):
```go
ctx, err := buildContextForResource(currentState, r, options, functions)
```

**File**: `internal/parser/context.go`

7. **Resource Lookup** (line 42):
```go
resource, ok := res.resources[fqdn.StringWithoutAttribute()]
```

### Why It Exists

The `parsed` type exists to enable **two-phase processing**:

1. **Phase 1 - Parsing**: Create resource instances, store HCL bodies
2. **Phase 2 - DAG Walk**: Decode bodies with full interpolation context

This separation is necessary because:
- Resources can reference each other via interpolations
- Decoding must happen in dependency order (DAG-sorted)
- Bodies must be preserved between phases

## Data Flow Architecture

### Complete Processing Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: FILE PARSING                                           │
│ Location: parser.go:236-323 (parseResourcesInFile)             │
├─────────────────────────────────────────────────────────────────┤
│ For each .xcl file:                                             │
│   For each resource block:                                      │
│     1. Parse HCL block structure                                │
│     2. Extract resource type, name, labels                      │
│     3. Create resource instance:                                │
│        - Builtin: Variable, Output, Module                      │
│        - Custom: From PluginRegistry                            │
│     4. Extract Links (dependencies) from HCL expressions        │
│     5. Store in parsedResources:                                │
│        parsedResources.resources[FQRN] = resourceInstance       │
│        parsedResources.bodies[FQRN] = hclBody                   │
│                                                                  │
│ At this point:                                                   │
│ - Resources exist but are NOT decoded (fields are zero values)  │
│ - Bodies contain raw HCL for later decoding                     │
│ - Links contain dependency information                          │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 2: TRANSFER TO STATE                                      │
│ Location: parser.go:212-215                                     │
├─────────────────────────────────────────────────────────────────┤
│ Transfer parsed resources to State object:                      │
│                                                                  │
│   for _, resource := range p.parsedResources.resources {        │
│       currentState.AppendResource(resource)                     │
│   }                                                              │
│                                                                  │
│ Note: Bodies are NOT transferred (State doesn't need them)      │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 3: DAG CONSTRUCTION                                       │
│ Location: dag.go:44-98 (buildCreateDAG)                        │
├─────────────────────────────────────────────────────────────────┤
│ 1. Create directed acyclic graph                                │
│ 2. Add all resources as vertices                                │
│ 3. Add edges based on:                                          │
│    - Links (interpolation dependencies)                         │
│    - Explicit depends_on attributes                             │
│ 4. Validate no cycles exist                                     │
│ 5. Return topologically sorted graph                            │
│                                                                  │
│ Result: Resources ordered for safe dependency resolution        │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 4: DAG WALK & DECODE                                      │
│ Location: parser.go:615-633, callbacks.go:22-163               │
├─────────────────────────────────────────────────────────────────┤
│ Walker processes each resource in dependency order:             │
│                                                                  │
│ walkCallback(vertex):                                           │
│   1. Get resource metadata                                      │
│   2. Skip if disabled or root                                   │
│   3. Lookup HCL body: parsedResources.bodies[FQRN]             │
│   4. Build interpolation context:                               │
│      - Extract variables from dependencies                      │
│      - Extract resource references from dependencies            │
│      - Apply variable precedence (defaults < files < env)       │
│   5. Decode body into resource: gohcl.DecodeBody(body, ctx, r)  │
│   6. Process disabled flag                                      │
│   7. Set defaults                                               │
│   8. Call provider lifecycle (Create/Update)                    │
│   9. Convert cty values to Go types                             │
│                                                                  │
│ Result: Fully decoded and processed resources                   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 5: STATE RETURN                                           │
│ Location: parser.go:636                                         │
├─────────────────────────────────────────────────────────────────┤
│ Return State object to caller:                                  │
│ - Contains fully decoded resources                              │
│ - No HCL bodies (clean separation)                              │
│ - Ready for querying and serialization                          │
│ - Can be saved via StateStore                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Two-Phase Processing Model

### Why Not Decode During Parsing?

**Problem**: Resources can reference each other via interpolations.

Example HCL:
```hcl
variable "port" {
  default = 8080
}

resource "container" "web" {
  port = variable.port
}

output "url" {
  value = "http://localhost:${resource.container.web.port}"
}
```

**Decoding Order Matters**:
1. Must decode `variable.port` first
2. Then decode `container.web` (references variable.port)
3. Finally decode `output.url` (references container.web.port)

**DAG Ensures Correct Order**:
- Builds dependency graph from Links
- Topologically sorts vertices
- Guarantees dependencies are decoded before dependents

### Phase 1: Parse (Create Shells)

```go
// In parseResourcesInFile()
for each resource block {
    // Create empty resource instance
    rt := pluginRegistry.CreateResource(type, name)

    // Extract dependencies (but don't resolve yet)
    links := extractLinks(block.Body)
    types.SetLinks(rt, links)

    // Store for later decoding
    parsedResources.resources[FQRN] = rt
    parsedResources.bodies[FQRN] = block.Body
}
```

**Result**: Resources exist but fields are zero values.

### Phase 2: Decode (Fill Data)

```go
// In walkCallback() - called for each vertex in DAG order
func walkCallback(v dag.Vertex) {
    resource := v

    // Get the stored HCL body
    body := parsedResources.bodies[resource.ID]

    // Build context with already-decoded dependencies
    ctx := buildContextForResource(parsedResources, resource, ...)

    // Now decode - interpolations resolve correctly
    gohcl.DecodeBody(body, ctx, resource)
}
```

**Result**: Resources fully decoded in correct dependency order.

## Why Two Separate Data Structures

### `parsed` (Internal to Parser)

**Purpose**: Working memory during parsing

**Structure**:
```go
type parsed struct {
    resources map[string]any              // Fast lookup by FQRN
    bodies    map[string]*hclsyntax.Body  // Bodies for decoding
}
```

**Characteristics**:
- Temporary (exists only during Parse call)
- Contains HCL bodies (can't be serialized)
- Map structure for O(1) lookup during DAG walk
- Internal to Parser, never exposed

**Why Not Use State?**
- State doesn't store HCL bodies
- State uses slice (not map) for different access patterns
- State is meant to be serializable

### `State` (Returned from Parser)

**Purpose**: Clean resource registry for queries and persistence

**Structure**:
```go
type State struct {
    resources []any      // All resources
    mu        sync.Mutex // Thread-safe
}
```

**Characteristics**:
- Persistent (returned to caller, can be saved)
- No HCL bodies (clean separation)
- Slice structure for iteration and queries
- Thread-safe for concurrent access
- JSON serializable

**Why Not Store Bodies?**
- Bodies are only needed during parsing
- State is serialized to disk (bodies can't serialize)
- Clean separation of concerns

### Comparison

| Feature | `parsed` | `State` |
|---------|----------|---------|
| **Lifetime** | Temporary (during Parse) | Persistent (returned) |
| **Contains Bodies** | Yes (for decoding) | No (clean data) |
| **Structure** | Map (fast lookup) | Slice (iteration) |
| **Thread Safety** | No (single-threaded parse) | Yes (mutex) |
| **Serializable** | No (has HCL bodies) | Yes (JSON) |
| **Visibility** | Private to Parser | Public API |
| **Purpose** | Parsing workflow | Resource queries |

## Implementation Status

### ✅ Completed Components

1. **State Management** (`state/state.go`)
   - NewState() constructor
   - AppendResource(), RemoveResource()
   - FindResource(), FindResourcesByType(), FindModuleResources()
   - Thread-safe with mutex
   - JSON serialization
   - Comprehensive tests

2. **DAG Building** (`internal/parser/dag.go`)
   - DoYouLikeDags() entry point
   - buildCreateDAG() - topological sort
   - buildDestroyDAG() - reverse dependencies
   - Cycle detection
   - Root node handling

3. **Context Building** (`internal/parser/context.go`)
   - buildContextForResource() - creates HCL eval context
   - Dependency resolution from Links
   - Variable precedence (defaults < files < env < direct)
   - Resource reference resolution
   - Module variable support (commented out, planned)

4. **Callbacks** (`internal/parser/callbacks.go`)
   - walkCallback() - full implementation
   - destroyWalkCallback() - destroy flow
   - Disabled resource handling
   - Default value setting
   - Provider lifecycle calls (commented out, planned)
   - Error handling with ParserError

5. **File Parsing** (`internal/parser/parser.go:236-323`)
   - findXclFiles() - recursive file discovery
   - parseResourcesInFile() - HCL parsing
   - Resource type detection (Variable, Output, Module, custom)
   - Label validation
   - Link extraction
   - Body storage

### ❌ Missing Components

1. **Type Definition** (`internal/parser/parser.go`)
   - `parsed` struct not defined
   - Causes compilation error

2. **Parser Struct Field** (`internal/parser/parser.go:105-111`)
   - `parsedResources *parsed` field missing
   - Causes compilation error

3. **Callback Wiring** (`internal/parser/parser.go:617`)
   - walkCallback commented out
   - Placeholder returns nil
   - Prevents actual decoding

4. **Module Support** (`internal/parser/parser.go:296-300`)
   - Panics with "modules not yet implemented"
   - Module variable passing commented out
   - Module resource processing disabled

5. **Functions Definition** (`internal/parser/parser.go:221`)
   - getFunctions() TODO
   - Custom function registration incomplete

### 🚧 Partially Complete

1. **Plugin Integration**
   - PluginRegistry integration exists
   - Provider lifecycle calls commented out in walkCallback
   - Event firing incomplete

2. **State Persistence**
   - StateStore interface defined
   - FileStateStore implemented
   - Loading/saving partially wired

## Required Fixes

### Fix 1: Define `parsed` Type

**Location**: `internal/parser/parser.go` (add after line 26)

```go
// parsed holds resources and their HCL bodies during the parsing phase.
// This is an internal working structure used only within the Parser.
// After parsing completes, resources are transferred to State (without bodies).
type parsed struct {
    resources map[string]any              // FQRN -> resource instance
    bodies    map[string]*hclsyntax.Body  // FQRN -> HCL body for later decoding
}
```

**Why Here**:
- Near other parser types (ResourceTypeNotExistError, ParserOptions)
- Before Parser struct definition
- Private to parser package

### Fix 2: Add Parser Field

**Location**: `internal/parser/parser.go:105-111`

**Current**:
```go
type Parser struct {
    options         ParserOptions
    customFunctions map[string]function.Function
    stateStore      state.StateStore
    pluginRegistry  *registry.PluginRegistry
    rawBodies       map[string]*hclsyntax.Body
}
```

**Updated**:
```go
type Parser struct {
    options         ParserOptions
    customFunctions map[string]function.Function
    stateStore      state.StateStore
    pluginRegistry  *registry.PluginRegistry
    parsedResources *parsed  // Working storage during parsing
}
```

**Changes**:
- Add `parsedResources *parsed` field
- Remove `rawBodies` field (duplicates parsed.bodies, unused)

### Fix 3: Wire Up Callback

**Location**: `internal/parser/parser.go:616-621`

**Current**:
```go
// Define the walker callback that will be called for every node in the graph
w := dag.Walker{}
// TODO: Implement walkCallback to process resources
// w.Callback = walkCallback(p.parsedResources.bodies, currentState, previousState, p.pluginRegistry, &p.options, functions)
w.Callback = func(v dag.Vertex) (diags dag.Diagnostics) {
    // Placeholder - actual implementation needed
    return nil
}
```

**Updated**:
```go
// Define the walker callback that will be called for every node in the graph
w := dag.Walker{}
w.Callback = walkCallback(p.parsedResources, previousState, p.pluginRegistry, &p.options, functions)
```

**Note**: Signature needs adjustment - `walkCallback` expects `*parsed` not just bodies.

### Fix 4: Update walkCallback Signature

**Location**: `internal/parser/callbacks.go:22`

**Current**:
```go
func walkCallback(currentState *parsed, previousState *parsed, registry *registry.PluginRegistry, options *ParserOptions, functions map[string]function.Function) func(v dag.Vertex) (diags dag.Diagnostics) {
```

**Analysis**: Signature is correct, but parameter name is confusing:
- `currentState *parsed` - should be `current *parsed` or `parsedData *parsed`
- It's NOT the State object, it's the parsed working data

**Better Naming**:
```go
func walkCallback(parsedData *parsed, previousState *parsed, registry *registry.PluginRegistry, options *ParserOptions, functions map[string]function.Function) func(v dag.Vertex) (diags dag.Diagnostics) {
```

### Fix 5: Implement getFunctions

**Location**: `internal/parser/parser.go:221`

**Current**:
```go
// Get HCL functions
functions := map[string]function.Function{}
// TODO: Implement getFunctions to get both custom and builtin functions
```

**Implementation**:
```go
// Get HCL functions (custom + builtins)
functions := p.getFunctions()
```

Add method:
```go
// getFunctions returns all HCL functions (custom + builtins)
func (p *Parser) getFunctions() map[string]function.Function {
    funcs := map[string]function.Function{}

    // Add custom functions from options
    for name, fn := range p.customFunctions {
        funcs[name] = fn
    }

    // Add builtin functions (if any)
    // TODO: Add builtin functions when implemented

    return funcs
}
```

### Fix 6: Handle Module Panic

**Location**: `internal/parser/parser.go:296-300`

**Current**:
```go
case resources.TypeModule:
    // Module support is not yet implemented in v2
    // Skip module parsing for now
    panic("modules not yet implemented")
```

**Better Approach**:
```go
case resources.TypeModule:
    // Module support is not yet implemented in v2
    ce := errors.NewConfigError()
    pe := errors.NewParserError(
        file,
        errors.ParserErrorLevelError,
        "module resources are not yet supported in v2",
    )
    ce.AppendError(pe)
    return []error{ce}
```

Or simply skip:
```go
case resources.TypeModule:
    // Module support coming in future release
    continue  // Skip module blocks for now
```

## Design Decisions

### Q: Why not decode during parsing?

**A**: Interpolations can reference resources that haven't been processed yet.

**Example**:
```hcl
output "url" {
  value = "http://${resource.container.web.ip}:${variable.port}"
}

resource "container" "web" {
  port = variable.port
}

variable "port" {
  default = 8080
}
```

File order doesn't guarantee dependency order. DAG ensures correct processing sequence.

### Q: Why store bodies separately?

**A**: State needs to be JSON-serializable for persistence. HCL bodies contain Go pointers and can't be serialized.

**Design**:
- `parsed.bodies` - temporary, for decoding
- State - clean, serializable

### Q: Why not just use State everywhere?

**A**: State and parsed serve different purposes:
- **State**: Clean API, queries, persistence (slice structure)
- **parsed**: Working memory, fast lookup (map structure)

State doesn't have bodies because it's meant to be a clean resource registry, not a parsing workspace.

### Q: Why map vs slice?

**A**: Different access patterns:
- **parsed**: Random access by FQRN during DAG walk (map is O(1))
- **State**: Iteration and queries (slice is better for range loops)

### Q: Why two phases instead of streaming?

**A**: Dependency resolution requires all resources to exist first:
- Phase 1: Discover all resources and their dependencies (Links)
- Phase 2: Build DAG from complete dependency graph
- Phase 3: Decode in topological order

Can't build DAG until all dependencies are known.

### Q: What about circular dependencies?

**A**: DAG validation (`dag.Validate()`) detects cycles and returns error before walk begins.

### Q: Why is previousState also `*parsed`?

**A**: For state comparison during updates:
- Load previous state from StateStore
- Reconstruct as `*parsed` with resources (but no bodies needed)
- Compare old vs new resources in walkCallback
- Route to Create() vs Update() lifecycle methods

### Q: Why is walkCallback a closure?

**A**: Provides access to parsing context:
- `parsedData` - current parse with bodies
- `previousState` - old state for comparison
- `registry` - plugin providers
- `options` - parser configuration
- `functions` - HCL functions

Closure captures all needed context without global state.

## Implementation Checklist

### Immediate (Blocking Compilation)

- [ ] Add `parsed` type definition
- [ ] Add `parsedResources` field to Parser struct
- [ ] Remove unused `rawBodies` field
- [ ] Uncomment walkCallback wiring
- [ ] Implement getFunctions() method
- [ ] Fix module parsing panic

### Near-Term (Complete Functionality)

- [ ] Uncomment provider lifecycle calls in walkCallback
- [ ] Implement state comparison logic
- [ ] Wire up previousState loading
- [ ] Enable interpolation dependency detection
- [ ] Add comprehensive error handling

### Future (Module Support)

- [ ] Implement module parsing
- [ ] Implement module variable passing
- [ ] Implement module resource scoping
- [ ] Add module dependency resolution

## Testing Strategy

### Unit Tests Needed

1. **parsed type**:
   - Resource storage and retrieval
   - Body storage and retrieval
   - Multiple resources with same type

2. **walkCallback**:
   - Decoding with interpolations
   - Dependency resolution
   - Disabled resource handling
   - Error cases

3. **Context building**:
   - Variable precedence
   - Resource references
   - Missing dependencies

### Integration Tests Needed

1. **Full parse flow**:
   - Multiple files
   - Cross-file references
   - Complex dependency graphs

2. **State persistence**:
   - Save after parse
   - Load and update
   - State comparison

## References

- **Main Plan**: `thoughts/shared/plans/2025-10-21-14-config-state-separation.md`
- **DAG Refactor**: `thoughts/shared/plans/2025-10-16-10-config-dag-refactoring.md`
- **TODO Tracking**: `TODO.md`

## Conclusion

The parser refactoring is architecturally sound and nearly complete. The missing `parsed` type is a simple oversight that blocks compilation. Once the type is defined and fields are added, the existing implementation should work correctly.

The two-phase processing model (parse then decode) is the correct approach for HCL with interpolations. The separation between `parsed` (working memory) and `State` (clean API) follows good design principles.

**Estimated effort to complete**: 2-3 hours for immediate fixes, 1-2 days for full functionality.

## Implementation Progress Update (2025-12-27)

### Completed Fixes

1. ✅ **Added `parsed` type definition** - [parser.go:35-41](internal/parser/parser.go#L35-L41)
   - Defined struct with `resources` and `bodies` maps
   
2. ✅ **Updated Parser struct** - [parser.go:118](internal/parser/parser.go#L118)
   - Added `parsedResources *parsed` field
   - Removed unused `rawBodies` field

3. ✅ **Implemented `getFunctions()` method** - [parser.go:669-682](internal/parser/parser.go#L669-L682)

4. ✅ **Wired up walkCallback** - [parser.go:229](internal/parser/parser.go#L229), [parser.go:629](internal/parser/parser.go#L629)

5. ✅ **Fixed module parsing panic** - [parser.go:306](internal/parser/parser.go#L306)
   - Replaced panic with graceful skip

6. ✅ **Updated walkCallback parameter naming** - [callbacks.go:22](internal/parser/callbacks.go#L22)
   - Renamed `currentState` → `parsedData`
   - Renamed `previousState` → `previousParsed`

7. ✅ **Restored callbacks.go** - Renamed from `.bak`

8. ✅ **Fixed test infrastructure**
   - Updated mock to expect `Exists()` call
   - Fixed test file paths (`./internal/test_fixtures` → `../test_fixtures`)
   - Renamed all test fixtures from `.hcl` to `.xcl`

9. ✅ **CRITICAL FIX: Set resource IDs** - [parser.go:475-477](internal/parser/parser.go#L475-L477)
   ```go
   // Set the ID from the FQRN
   fqrn := resources.FQRNFromResource(rt)
   rtMeta.ID = fqrn.String()
   ```
   - **Root Cause**: Resources were being added to `parsedResources.resources` map with empty string keys
   - **Impact**: All resources were overwriting each other, only last one remained
   - **Solution**: Generate ID from FQRN after setting Type, Name, and Module

### Current Status

**Parser Compilation**: ✅ Compiles successfully
**Resource Parsing**: ✅ All resources parsed and stored correctly  
**Resource Decoding**: ⚠️ Works with `executePlugins=true`, not with `false`
**Tests**: ⚠️ Type assertion failures

### Discovered Issues

#### Issue 1: Resource Type Assertion Failures

**Problem**: Resources returned from State cannot be type-asserted to their original types (e.g., `*structs.Container`)

**Test Error**:
```
Error: resource is not of type *structs.Container
```

**Hypothesis**: 
- Resources are created by plugins and stored in State as `any`
- When `executePlugins=false`, resources are not decoded (DAG walk doesn't run)
- Resources remain as empty shells without their fields populated
- Type assertion may be failing because the concrete type doesn't match the expected type

**Possible Root Causes**:
1. Resources created by `PluginRegistry.CreateResource()` may return a different type than expected
2. Without decoding (which only happens during DAG walk), resources are incomplete
3. State serialization/deserialization may affect types

#### Issue 2: Decoding Only Happens with executePlugins=true

**Problem**: The two-phase model requires DAG walk to decode resources, but DAG walk only runs when `executePlugins=true`

**Current Flow**:
```
Parse(executePlugins=false):
  1. Parse files → create resource shells
  2. Add to parsedResources
  3. Transfer to State
  4. Skip DAG walk (no decoding)
  5. Return State with un-decoded resources

Parse(executePlugins=true):
  1. Parse files → create resource shells
  2. Add to parsedResources  
  3. Transfer to State
  4. Build DAG
  5. Walk DAG → decode bodies into resources
  6. Return State with decoded resources
```

**Question**: Should decoding always happen, regardless of `executePlugins` flag?

**Options**:
- A: Always decode, `executePlugins` only controls provider lifecycle calls
- B: Keep current behavior, update tests to use `executePlugins=true`
- C: Add separate `decode` parameter

### Architectural Concerns

#### Concern 1: Plugin-Created Types in State

State stores resources created by plugins, which are loaded dynamically. This creates challenges:

1. **Type Safety**: State stores `any`, but consumers expect specific types
2. **Serialization**: Plugin types may not serialize to JSON cleanly
3. **Type Assertion**: Tests fail when trying to assert plugin types

**Question**: Should State store raw HCL structures instead of plugin-created types?

#### Concern 2: Separation of Parse vs Execute

The original design intent was to separate:
- **Parsing**: Convert HCL → resource structures
- **Execution**: Call plugin lifecycle methods

But currently:
- Parsing creates empty shells
- Decoding (filling the shells) happens during execution
- This couples parsing and execution

**Question**: Should decoding be part of parsing, not execution?

### Next Steps

1. **Investigate plugin resource creation**
   - Check what type `PluginRegistry.CreateResource()` actually returns
   - Verify if it matches the expected type from test plugins
   
2. **Determine decoding strategy**
   - Should decoding always happen?
   - Where should the `executePlugins` flag apply?

3. **Review State design**
   - Should State store decoded resources or raw data?
   - How should State handle plugin-created types?

4. **Update tests**
   - Determine correct expectations for `executePlugins=false`
   - Update tests to match chosen architecture

### Files Modified

- `internal/parser/parser.go` - Added `parsed` type, set resource IDs
- `internal/parser/util.go` - Already correctly handles `.xcl` files only
- `internal/parser/callbacks.go` - Renamed from `.bak`, updated parameter names
- `internal/parser/parse_test.go` - Fixed paths, updated to `.xcl` extensions
- `internal/parser/parser_plugin_test.go` - Fixed to use PluginRegistry API
- `internal/test_fixtures/**/*.hcl` - Renamed to `.xcl`

### Test Results

```bash
go test ./internal/parser/... -v -run TestParseFileProcessesResources
```

**Result**: Fails with type assertion error
**Resources Parsed**: 9/9 (all resources successfully parsed)
**Resources in State**: 9 (correct count)
**Type Assertion**: ❌ Fails - resource is not of expected type

