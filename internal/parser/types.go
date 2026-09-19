package parser

import (
	"reflect"

	"github.com/jumppad-labs/xcl/internal/xcl/tags"
)

// propertyNames reports the properties a type exposes to configuration, keyed
// by the name configuration uses for each.
//
// This is the single authoritative answer to what a type's properties are
// called. The rule it follows is not this codebase's own: the fork of go-cty
// this project pins reads the `hcl` struct tag where the upstream library reads
// `cty`, and that rule is what the runtime actually accepts. It is mirrored
// here rather than re-derived, and it lives in exactly one place, because a
// second divergent copy would reintroduce the disagreement about what a
// configuration means that the validation stage exists to remove.
//
// The rules, in full:
//
//   - the configuration name is the part of the `hcl` tag before the first
//     comma, so `xcl:"subnet,optional"` is addressed as `subnet`;
//   - an empty first segment contributes no name of its own, so
//     `xcl:",remain"` names nothing;
//   - an anonymous embedded field flattens its own properties into the
//     containing type, recursively;
//   - an embedded field may do both at once - `xcl:"rm,remain"` registers the
//     name `rm` *and* flattens - so the two rules are applied independently
//     rather than as alternatives;
//   - a field carrying no `hcl` tag is invisible to configuration and is
//     absent from the result even though it is a real Go field. Referring to
//     one is therefore correctly reported as naming something that does not
//     exist.
func propertyNames(t reflect.Type) map[string]reflect.Type {
	names := map[string]reflect.Type{}

	t = dereference(t)
	if t.Kind() != reflect.Struct {
		return names
	}

	collectPropertyNames(t, names, map[reflect.Type]bool{})

	return names
}

// collectPropertyNames gathers a type's property names into names, following
// anonymous embedded fields into the parent. visited guards against a type that
// embeds itself, directly or through a chain.
func collectPropertyNames(t reflect.Type, names map[string]reflect.Type, visited map[reflect.Type]bool) {
	if visited[t] {
		return
	}
	visited[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// An anonymous field contributes its own properties to this type. This
		// happens whether or not the field also carries a name of its own, so
		// it is checked before, and independently of, the tag below.
		if field.Anonymous {
			embedded := dereference(field.Type)
			if embedded.Kind() == reflect.Struct {
				collectPropertyNames(embedded, names, visited)
			}
		}

		name := xclTagName(field)
		if name == "" {
			continue
		}

		names[name] = field.Type
	}
}

// xclTagName returns the name configuration uses for a field, which is the part
// of its `xcl` tag before the first comma. It returns an empty string when the
// field has no valid `xcl` tag, or when the tag contributes no name of its own.
func xclTagName(field reflect.StructField) string {
	tag, ok := parseXclTag(field)
	if !ok {
		return ""
	}

	return tag.Name
}

// parseXclTag returns the field's parsed `xcl` tag, ok is false when the field
// has no tag or the tag is not valid
func parseXclTag(field reflect.StructField) (tags.Field, bool) {
	tag, ok := field.Tag.Lookup(tags.Name)
	if !ok {
		return tags.Field{}, false
	}

	ft, err := tags.Parse(tag)
	if err != nil {
		return tags.Field{}, false
	}

	return ft, true
}

// dereference follows pointers to the type they point at.
func dereference(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t
}
