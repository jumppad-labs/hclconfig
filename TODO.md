# HCLConfig Resource State Tracking - TODO

## Overview
This document tracks the implementation of resource state tracking for the HCLConfig parser. The goal is to track resource lifecycle across multiple ParseDirectory runs and integrate with provider lifecycle methods.

## Completed Work ✅

### Phase 1: Cleanup (Completed)
- [x] **Removed Processable interface** - `types/resource.go` - Eliminated Process() method pattern
- [x] **Removed checksum functionality** - `types/resource.go`, `utils.go`, `parser.go` - Removed Checksum type and generation
- [x] **Removed Diff functionality** - `config.go` - Removed ResourceDiff struct and Diff() method  
- [x] **Removed callback functionality** - `parser.go`, `dag.go`, tests - Eliminated user callbacks from parser
- [x] **Simplified process() method** - `parser.go:1697` - Single DAG walk instead of two-pass processing
- [x] **Fixed compilation issues** - Updated tests and examples to remove callback references

## Remaining Implementation Tasks 🚧

### Phase 2: State Store Foundation
- [ ] **Design StateStore interface**
  - Define interface that leverages Config's JSON serialization (`config.go:266-271`)
  - Support multiple state store implementations
  - Methods: Load(), Save(), Exists(), Clear()

- [ ] **Implement FileStateStore**
  - Create file-based state store as default implementation
  - JSON file format for persistence
  - Handle file locking and atomic writes

- [ ] **Integrate state store into Parser**
  - Add StateStore field to ParserOptions
  - Initialize default FileStateStore if none provided

### Phase 3: Resource State Management
- [ ] **Add state tracking to resources**
  - Extend resource metadata or create separate state structure
  - Track resource fingerprints for change detection
  - Store provider-specific state data

- [ ] **Implement state comparison logic**
  - Compare old vs new resources to determine changes
  - Handle resource addition, modification, and removal
  - Generate state transition events

- [ ] **Define resource lifecycle states**
  - New: Resource exists in config but not in state
  - Existing: Resource exists in both config and state
  - Changed: Resource exists in both but differs
  - Removed: Resource exists in state but not in config

### Phase 4: Provider Lifecycle Integration
- [ ] **Update walkCallback function** - `dag.go:144`
  - Replace TODO comment with actual provider lifecycle calls
  - Route operations based on resource state
  - Handle provider operation errors

- [ ] **Implement operation sequencing**
  - For existing resources: Call Changed() first
  - Only call Refresh() if Changed() returns true
  - Maintain correct dependency order

- [ ] **Add provider method routing**
  - New resources → Create()
  - Existing unchanged → skip processing
  - Existing changed → Changed() → Refresh() (conditional)
  - Removed resources → Destroy()

### Phase 5: ParseDirectory Enhancement
- [ ] **Enhance ParseDirectory flow** - `parser.go`
  - Load previous state before parsing
  - Compare old state with new configuration
  - Execute provider operations based on state differences
  - Save new state after successful operations

- [ ] **Add error handling and rollback**
  - Handle provider operation failures gracefully
  - Implement partial rollback on errors
  - Preserve state consistency

### Phase 6: Testing & Cleanup
- [ ] **Fix broken tests** - `parse_test.go`
  - Update callback-dependent tests to work with new lifecycle
  - Remove placeholder `calls` variables
  - Restore proper test assertions

- [ ] **Add comprehensive state tracking tests**
  - Test state persistence and loading
  - Test state comparison logic
  - Test provider lifecycle integration
  - Test error handling and edge cases

- [ ] **Update documentation and examples**
  - Update `example/main.go` to demonstrate state tracking
  - Add state store configuration examples
  - Document provider lifecycle requirements

### Phase 7: Implementing Diff for resources
- [ ] **Implement Diff functionality**
  - The Changed plugin lifecycle method is responsible for determining if a resource has changed. 
  - Previously we would generate a checksum for the raw config and then we would compare that to the 
  checksum of the previous config. If the two were different, we would call the Refresh method.
  - However, this was not particularly effective, as it only showed that the file had changed, it did not
  show what had changed. It also did not highlight any changes that were set when the value of a dependency
  changed, such as a variable or a data source.
  - I think the best way to check this is going to be to add a helper method that compares the two stucts and returns
  the changes that have been made.
  - We have to think about values in the resource that are computed, i.e. they are set as part of the 
  resource creation, but are not set in the config. If we are doing a compare between the previous and the current
  config these attribues will not yet be set on the new config.
  - One thing we could do is have a special annotation for computed values, such as `computed: true`, and then
  we could ignore these values when doing the compare.


## Architecture Notes 📝

### Key Design Principles
1. **Leverage existing Config JSON serialization** - Don't reinvent serialization
2. **Provider operation order**: Changed() → Refresh() (conditional) 
3. **No backward compatibility concerns** - Clean slate implementation
4. **Support multiple state store implementations** - Interface-based design
5. **Maintain DAG dependency resolution** - State operations follow dependency order

### Code References
- **Provider interface**: `plugins/provider.go` - ResourceProvider interface
- **Resource metadata**: `types/resource.go` - Meta struct
- **DAG processing**: `dag.go:144` - walkCallback function
- **Parser entry point**: `parser.go` - ParseDirectory methods
- **Config serialization**: `config.go:266-271` - ToJSON/FromJSON methods

### Provider States (Inferred from ResourceProvider interface)
1. **New** - Resource doesn't exist, needs Create()
2. **Existing/Unchanged** - Resource exists and unchanged, skip operations  
3. **Existing/Changed** - Resource exists but changed, call Changed() then conditionally Refresh()
4. **Removed** - Resource no longer in config, needs Destroy()
5. **Unknown** - Error state, needs investigation

## Next Steps 🎯

1. **Start with StateStore interface design** - Foundation for everything else
2. **Implement FileStateStore** - Concrete implementation for testing
3. **Add basic state tracking** - Simple state comparison logic
4. **Integrate with walkCallback** - Connect state to provider operations
5. **Test incrementally** - Validate each phase before moving to next

## Notes
- Tests that fail due to callback removal will be fixed in Phase 6
- Plugin discovery and registration system is already implemented
- Resource dependency resolution via DAG is already working
- Config JSON serialization is already implemented and tested