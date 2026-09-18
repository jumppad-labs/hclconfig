package parser

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/jumppad-labs/xcl/internal/resources"
)

// bracketIndex matches a trailing collection selector on a path segment, either
// a position such as `network[0]` or a key such as `map["one"]`.
var bracketIndex = regexp.MustCompile(`^(?P<name>[^\[\]]*)\[(?P<selector>[^\[\]]*)\]$`)

// checkPropertyPath reports whether a dotted property path names real
// properties on a type, returning the first segment that does not exist or an
// empty string when the path is acceptable.
//
// The walk steps through the path one segment at a time, carrying the type it
// has reached so far. Two rules govern where it stops, and the difference
// between them is the whole point:
//
//   - A segment that *selects a member* of a collection - a map key, a position
//     in a list, or every member at once - says which member is meant, never
//     what type it is. The walk steps to the member's type and keeps checking,
//     which is what catches a misspelled property written after a splat.
//   - The walk stops and accepts the remainder only where the *type* itself
//     stops being knowable: a collection of scalars, a dynamically typed value,
//     or an interface. Nothing is rejected at such a boundary, because a false
//     rejection breaks working configuration while a missed typo is only the
//     status quo.
func checkPropertyPath(target reflect.Type, path string) string {
	if path == "" {
		return ""
	}

	current := dereference(target)

	// A reference to an output or a variable names the value it holds, not the
	// declaration that holds it - `output.thing.name` reads `name` from the
	// output's value. That value's shape is decided when the configuration is
	// evaluated, so nothing beneath such a reference is knowable now.
	if holdsDynamicValue(current) {
		return ""
	}

	for _, segment := range strings.Split(path, ".") {
		if current == nil {
			return ""
		}

		name, selector, hasSelector := splitSelector(segment)

		// A segment may name a property, select a member, or do both at once as
		// in `network[0]`. Resolve the name first when there is one.
		if name != "" {
			// A segment standing against a collection selects one of its
			// members rather than naming a property on it. That is true of a
			// splat, a position, and a map key alike - a key is an arbitrary
			// string, so it is only recognisable as a selection by the type it
			// stands against, never by how it is written.
			if selects(name) || isCollection(current) {
				next, knowable := memberType(current)
				if !knowable {
					return ""
				}

				current = next

				continue
			}

			if !knowable(current) {
				// The type has stopped being knowable, so nothing beneath it
				// can be judged. Accept the remainder rather than reject it.
				return ""
			}

			properties := propertyNames(current)
			if len(properties) == 0 {
				// Nothing is known about this type's properties, so nothing
				// beneath it can be judged.
				return ""
			}

			property, ok := properties[name]
			if !ok {
				return name
			}

			current = dereference(property)
		}

		if hasSelector {
			// `network[0]` and `map["key"]` both select a member of whatever
			// the name resolved to.
			_ = selector

			next, knowable := memberType(current)
			if !knowable {
				return ""
			}

			current = next
		}
	}

	return ""
}

// splitSelector separates a path segment into the property it names and the
// member selector it applies, if any. `network[0]` yields ("network", "0",
// true), `*` yields ("*", "", false), and `subnet` yields ("subnet", "", false).
func splitSelector(segment string) (string, string, bool) {
	match := bracketIndex.FindStringSubmatch(segment)
	if match == nil {
		return segment, "", false
	}

	return match[1], match[2], true
}

// selects reports whether a segment identifies a member of a collection rather
// than naming a property. A splat selects every member at once; a bare number
// selects a position, as written in the dotted form `volume.0.source`.
func selects(segment string) bool {
	if segment == "*" {
		return true
	}

	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}

	return segment != ""
}

// holdsDynamicValue reports whether a type is a declaration whose referenced
// content is the value it holds rather than its own fields. Outputs, variables
// and locals are all of this kind: the value they carry is established when the
// configuration is evaluated, so a path reaching through one cannot be checked.
func holdsDynamicValue(t reflect.Type) bool {
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}

	switch t {
	case reflect.TypeOf(resources.Output{}), reflect.TypeOf(resources.Variable{}):
		return true
	default:
		return false
	}
}

// knowable reports whether a type's properties can be established before the
// configuration is applied.
//
// A dynamically typed value carries whatever the configuration puts in it, and
// an interface carries anything at all, so neither can say what properties it
// has until it holds something. A reference that reaches one of these is
// accepted for the rest of its length: rejecting it would break working
// configuration, while letting a typo through beneath it is only the status quo.
func knowable(t reflect.Type) bool {
	if t == nil {
		return false
	}

	if t.Kind() == reflect.Interface {
		return false
	}

	// cty.Value holds a configuration value whose shape is decided when the
	// configuration is evaluated, so nothing beneath it is knowable now.
	if t.PkgPath() == "github.com/zclconf/go-cty/cty" && t.Name() == "Value" {
		return false
	}

	return true
}

// isCollection reports whether a type holds members that are selected from
// rather than named as properties.
func isCollection(t reflect.Type) bool {
	if t == nil {
		return false
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return true
	default:
		return false
	}
}

// memberType returns the type of a collection's members, and whether that type
// is knowable. A map, slice or array whose members are themselves structured
// yields that structure; one whose members are scalars, dynamic or interfaces
// yields no useful type, and checking must stop there rather than reject.
func memberType(t reflect.Type) (reflect.Type, bool) {
	if t == nil {
		return nil, false
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		element := dereference(t.Elem())
		if !knowable(element) {
			return nil, false
		}

		if element == nil || element.Kind() != reflect.Struct {
			// A collection of scalars, or of something with no properties of
			// its own. Which member is meant no longer matters, because
			// nothing beneath it can be named.
			return nil, false
		}

		if len(propertyNames(element)) == 0 {
			return nil, false
		}

		return element, true
	default:
		// Selecting a member of something that is not a collection. The type
		// has stopped being knowable rather than the reference being wrong -
		// a dynamically typed value reaches here - so accept the remainder.
		return nil, false
	}
}

// propertyProblem renders the message for a property that a type does not have.
func propertyProblem(reference, property string) string {
	return fmt.Sprintf("reference '%s' names property '%s', which does not exist", reference, property)
}
