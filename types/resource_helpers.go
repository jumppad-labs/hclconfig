package types

import (
	"fmt"
	"reflect"
)

// findResourceBase uses reflection to find the embedded ResourceBase field in a resource.
// It handles nested embedding scenarios (e.g., PostgreSQL -> DBCommon -> ResourceBase).
func findResourceBase(resource any) (reflect.Value, error) {

	st := reflect.TypeOf(resource).Elem()
	_, found := st.FieldByName("ResourceBase")
	if !found {
		return reflect.Value{}, fmt.Errorf("ResourceBase field not found in resource type %s", st.Name())
	}

	v := reflect.ValueOf(resource).Elem()

	return v.FieldByName("ResourceBase"), nil
}

// GetMeta returns a pointer to the Meta field of the embedded ResourceBase
func GetMeta(resource any) (*Meta, error) {
	baseField, err := findResourceBase(resource)
	if err != nil {
		return nil, err
	}

	metaField := baseField.FieldByName("Meta")
	if !metaField.IsValid() {
		return nil, fmt.Errorf("Meta field not found in ResourceBase")
	}

	if !metaField.CanAddr() {
		return nil, fmt.Errorf("Meta field is not addressable")
	}

	// depending if the resource has been created using reflection, meta may be an anonymous struct
	// in this case we can not directy cast it, we need to construct a new
	metaInterface := metaField.Addr().Interface()

	meta, ok := metaInterface.(*Meta)
	if !ok {
		return nil, fmt.Errorf("Meta field is not of type *Meta, got %T", metaInterface)
	}

	return meta, nil
}

// GetDependencies returns the DependsOn slice from the embedded ResourceBase
func GetDependencies(resource any) ([]string, error) {
	baseField, err := findResourceBase(resource)
	if err != nil {
		return nil, err
	}

	dependsOnField := baseField.FieldByName("DependsOn")
	if !dependsOnField.IsValid() {
		return nil, fmt.Errorf("DependsOn field not found in ResourceBase")
	}

	if dependsOnField.IsNil() {
		return []string{}, nil
	}

	return dependsOnField.Interface().([]string), nil
}

func AppendUniqueDependency(resource any, dependency string) error {
	deps, err := GetDependencies(resource)
	if err != nil {
		return fmt.Errorf("failed to get dependencies: %w", err)
	}

	for _, d := range deps {
		if d == dependency {
			return nil // Dependency already exists
		}
	}

	deps = append(deps, dependency)
	return SetDependencies(resource, deps)
}

// SetResourceDependencies sets the entire DependsOn slice in the embedded ResourceBase
func SetDependencies(resource any, deps []string) error {
	baseField, err := findResourceBase(resource)
	if err != nil {
		return err
	}

	dependsOnField := baseField.FieldByName("DependsOn")
	if !dependsOnField.IsValid() {
		return fmt.Errorf("DependsOn field not found in ResourceBase")
	}

	if !dependsOnField.CanSet() {
		return fmt.Errorf("DependsOn field cannot be set")
	}

	dependsOnField.Set(reflect.ValueOf(deps))
	return nil
}

// GetResourceDisabled returns the Disabled field from the embedded ResourceBase
func GetDisabled(resource any) (bool, error) {
	baseField, err := findResourceBase(resource)
	if err != nil {
		return false, err
	}

	disabledField := baseField.FieldByName("Disabled")
	if !disabledField.IsValid() {
		return false, fmt.Errorf("Disabled field not found in ResourceBase")
	}

	return disabledField.Bool(), nil
}

// SetResourceDisabled sets the Disabled field in the embedded ResourceBase
func SetDisabled(resource any, disabled bool) error {
	baseField, err := findResourceBase(resource)
	if err != nil {
		return err
	}

	disabledField := baseField.FieldByName("Disabled")
	if !disabledField.IsValid() {
		return fmt.Errorf("Disabled field not found in ResourceBase")
	}

	if !disabledField.CanSet() {
		return fmt.Errorf("Disabled field cannot be set")
	}

	disabledField.SetBool(disabled)
	return nil
}
