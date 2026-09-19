package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// resourceBaseKeys are the JSON keys of types.ResourceBase. They hold xcl's own
// metadata about a resource, not provider state, so they never count as a change.
var resourceBaseKeys = []string{"meta", "depends_on", "disabled"}

// DefaultChanged provides default change detection for a ResourceProvider.
// Embed it in a provider to get a Changed method; defining Changed on the
// provider overrides it.
//
//	type MyProvider struct {
//		plugins.DefaultChanged[*MyResource]
//	}
type DefaultChanged[T any] struct{}

// Changed reports whether old and new differ. It compares the JSON form of both
// resources, ignoring xcl's resource metadata (meta, depends_on and disabled).
func (DefaultChanged[T]) Changed(ctx context.Context, old T, new T) (bool, error) {
	oldValue, err := comparableJSON(old)
	if err != nil {
		return false, fmt.Errorf("unable to compare old resource: %w", err)
	}

	newValue, err := comparableJSON(new)
	if err != nil {
		return false, fmt.Errorf("unable to compare new resource: %w", err)
	}

	return !reflect.DeepEqual(oldValue, newValue), nil
}

// comparableJSON returns the JSON form of a resource with xcl's metadata removed.
// A nil resource returns nil.
func comparableJSON(resource any) (any, error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	if fields, ok := value.(map[string]any); ok {
		for _, key := range resourceBaseKeys {
			delete(fields, key)
		}
	}

	return value, nil
}
