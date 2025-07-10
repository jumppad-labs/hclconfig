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

### Phase 2: State Store Foundation ✅
- [x] **Design StateStore interface** - `state_store.go`
  - Defined interface with LoadJSON(), SaveJSON(), Exists(), Clear()
  - Works with raw JSON to avoid circular dependencies
  - Support multiple state store implementations

- [x] **Implement FileStateStore** - `file_state_store.go`
  - Created file-based state store as default implementation
  - JSON file format for persistence in `.hclconfig/state/state.json`
  - Handles file locking (mutex) and atomic writes (temp file + rename)

- [x] **Integrate state store into Parser** - `parser.go`
  - Added StateStore field to ParserOptions
  - Initialize default FileStateStore if none provided
  - Added stateStore field to Parser struct

- [x] **Created comprehensive tests** - `file_state_store_test.go`
  - Tests for all StateStore operations
  - Concurrent access testing
  - Edge case handling

### Phase 3: Resource State Management ✅
- [x] **Add state loading to parser** - `parser.go`
  - Modified process() method to accept previousState parameter
  - Added state loading and saving with defer statements in ParseFile and ParseDirectory
  - Ensured state persistence even when errors occur for resuming failed operations

- [x] **Implement dependency validation with context-aware errors** - `parser.go`
  - Created validateResourceDependencies function that checks for missing dependencies
  - Added context-aware error messages that differentiate between "resource was removed" vs "resource never existed"
  - Integrated dependency validation into ParseFile and ParseDirectory methods

- [x] **Clean up obsolete Parse validation infrastructure**
  - Removed ParseError resource type from test plugins
  - Deleted obsolete test functions and test fixtures
  - Fixed require.IsType parameter order issues in tests

- [x] **Add state tracking to resources** - `types/resource.go`, `parser.go`
  - Added Status field to Meta struct to track operational state ("pending", "created", "failed")
  - Implemented preserve-by-default approach using previous state as working base
  - Created mergeNewResources() method to merge new config into existing state
  - Resources from new config are processed by DAG, resources not in new config are preserved
  - Set new resources to "pending" status, preserve existing status for unchanged resources
  - Ensured state integrity during partial failures by always saving working state

- [x] **Implement state comparison logic** ✅
  - Created state resource map for quick lookups in walkCallback
  - Implemented comparison between state resources (current reality) and config resources (desired state)
  - Added proper lifecycle routing based on resource existence in state

- [x] **Define resource lifecycle states** ✅
  - New: Resource doesn't exist in state, needs Create()
  - Existing/Unchanged: Resource exists and unchanged, preserve existing status
  - Existing/Changed: Resource exists but changed, call Update() after Changed() check
  - Failed: Resource operation failed, set status to "failed"

### Phase 4: Provider Lifecycle Integration ✅
- [x] **Update walkCallback function** - `dag.go:335-345`
  - Replaced TODO comment with actual provider lifecycle calls via callProviderLifecycle()
  - Added proper error handling and resource status tracking
  - Integrated with PluginRegistry for provider lookup

- [x] **Implement operation sequencing** ✅
  - For existing resources: Refresh() → Changed(old, new) → Update() (if changed)
  - For new resources: Create()
  - Maintains correct dependency order through DAG processing

- [x] **Add provider method routing** ✅
  - New resources → Create() and set status to "created"
  - Existing unchanged → preserve existing status  
  - Existing changed → Update() and set status to "updated"
  - Error handling → set status to "failed"
  - Skips builtin types (Variable, Output, Local, Module, Root)

### Phase 5: ParseDirectory Enhancement ✅
- [x] **Enhance ParseDirectory flow** - `parser.go:400-420`
  - Loads previous state before parsing new configuration
  - Merges new config into working state using mergeNewResources()
  - Executes provider operations through DAG walkCallback with lifecycle integration
  - Saves working state after processing (even on errors for recovery)

- [x] **Add error handling and rollback** ✅
  - Added comprehensive error handling in callProviderLifecycle()
  - Sets resource status to "failed" on provider operation errors
  - Preserves state consistency by always saving working state
  - Allows recovery from partial failures on subsequent runs

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

### Phase 7: Provider Interface Updates ✅
- [x] **Update ResourceProvider Interface** - `plugins/provider.go`
  - Added Update(ctx context.Context, resource T) error method
  - Modified Changed signature to: Changed(ctx context.Context, old T, new T) (bool, error)
  - Updated all adapter layers and plugin implementations

- [x] **Update Plugin Infrastructure** ✅
  - Updated ProviderAdapter interface with new methods
  - Modified TypedProviderAdapter implementation  
  - Updated PluginHost interfaces (Direct and GRPC)
  - Updated PluginEntityProvider and PluginBase
  - Regenerated protobuf definitions with Update service and modified Changed request

### Phase 8: Architecture Improvements ✅
- [x] **Rename ResourceRegistry to PluginRegistry** - `plugin_registry.go`
  - More accurate naming since it manages plugins, not resources
  - Updated all references throughout codebase

- [x] **Simplify Provider Access** ✅
  - Renamed GetPluginHostAndAdapter() to GetProvider()
  - Cleaner API that returns only the needed ProviderAdapter
  - Removed confusing "host" terminology from public interface

### Phase 9: Implementing Enhanced Diff for Resources
- [ ] **Implement Enhanced Diff Functionality**
  - The Changed() method now receives both old and new resources for comparison
  - Providers can implement intelligent diff logic based on their resource types
  - This replaces the previous checksum-based approach with semantic comparison
  - Providers can ignore computed values or handle dependency-driven changes
  - Each provider decides what constitutes a "change" worth updating

### Phase 10: Final Cleanup and Documentation
- [ ] **Final Cleanup**
  - Make error messages consistent, when only a single error is returned from a function, use the 
  compact format like below:

    ```go
    if err := someOperation(); err != nil {
        return fmt.Errorf("error message: %w", err)
    }
    ```

    If a tuple is returned, use the longer format:

    ```go
    value, err := someOperation(); 
    if err != nil {
        return fmt.Errorf("error message: %w", err)
    }
    ```

## Architecture Notes 📝

### Key Design Principles ✅
1. **Leverage existing Config JSON serialization** - Uses existing ToJSON/FromJSON for state persistence
2. **Provider operation order**: Refresh() → Changed(old, new) → Update() (if changed) | Create() (if new)
3. **State vs Config separation** - State = current reality, Config = desired state  
4. **Support multiple state store implementations** - Interface-based design maintained
5. **Maintain DAG dependency resolution** - State operations follow dependency order
6. **Clear provider interface** - PluginRegistry.GetProvider() for clean provider access

### Code References
- **Provider interface**: `plugins/provider.go` - ResourceProvider interface with Update() and Changed(old, new)
- **Resource metadata**: `types/resource.go` - Meta struct with Status field
- **Lifecycle implementation**: `dag.go:351-426` - callProviderLifecycle function
- **DAG processing**: `dag.go:146` - walkCallback function with PluginRegistry integration
- **Plugin management**: `plugin_registry.go` - PluginRegistry with GetProvider() method
- **Parser entry point**: `parser.go` - ParseDirectory methods with state management
- **Config serialization**: `config.go:266-271` - ToJSON/FromJSON methods
- **State persistence**: `file_state_store.go` - FileStateStore implementation

### Provider States (Implemented in callProviderLifecycle)
1. **New** - Resource doesn't exist in state, call Create() → status: "created"
2. **Existing/Unchanged** - Resource exists, Changed() returns false → preserve existing status
3. **Existing/Changed** - Resource exists, Changed(old, new) returns true → call Update() → status: "updated"  
4. **Failed** - Any provider operation fails → status: "failed"
5. **Removed** - Resource in state but not in config → preserved for future Destroy() implementation

## Completed Implementation 🎯

✅ **All major lifecycle components implemented:**
1. StateStore interface and FileStateStore implementation
2. PluginRegistry with provider lookup capabilities  
3. Enhanced provider interface with Update() and Changed(old, new)
4. Complete lifecycle integration in walkCallback
5. State comparison and resource status tracking
6. Updated protobuf definitions and all plugin layers
7. Improved naming and architecture

## Remaining Work 🚧

1. **Testing** - Update tests to work with new lifecycle
2. **Documentation** - Update examples and docs
3. **Provider Implementations** - Providers need to implement enhanced Changed() logic

## Notes
- Tests that fail due to callback removal will be fixed in Phase 6
- Plugin discovery and registration system is already implemented
- Resource dependency resolution via DAG is already working
- Config JSON serialization is already implemented and tested