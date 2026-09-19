package registry

import "fmt"

// TypeNameClashError is returned when a type name is already provided by a
// builtin, a registered type or a plugin. It is returned whichever way the
// clashing name reached the registry: RegisterType, RegisterPlugin,
// RegisterPluginWithPath or DiscoverAndLoadPlugins.
type TypeNameClashError struct {
	// Name is the clashing type name
	Name string
	// Existing describes what already provides the name, i.e. "builtin",
	// "registered type" or "plugin"
	Existing string
}

func (e *TypeNameClashError) Error() string {
	return fmt.Sprintf("type %q is already provided by %s", e.Name, e.Existing)
}
