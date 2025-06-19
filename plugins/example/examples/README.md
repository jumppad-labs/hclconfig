# Person Resource Test Fixtures

This directory contains HCL configuration files for testing the Person resource plugin.

## Files

### `simple_person.hcl`
- Contains a single person resource with basic fields
- Good for initial testing and validation
- Minimal configuration to verify plugin functionality

### `person.hcl`
- Contains multiple person resources demonstrating different field combinations
- Includes examples with all fields, minimal fields, and various data types
- Good for comprehensive testing

### `complex_person.hcl`
- Contains advanced test cases with edge cases
- International characters, special addresses, age ranges
- Good for testing data validation and edge case handling

## Usage

These files can be used to test:
1. HCL parsing and validation
2. Resource creation and lifecycle management
3. Field validation and type conversion
4. Plugin registration and provider functionality

## Expected Resource Type

All resources use the type identifier `"person"` and should be handled by the ExampleProvider in the parent directory.