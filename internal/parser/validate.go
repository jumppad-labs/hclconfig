package parser

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/types"
	"github.com/jumppad-labs/xcl/internal/cty"
)

// validate runs the validation stages over a parsed configuration and returns
// every problem found rather than stopping at the first.
//
// Validation runs to completion between parsing and walking. It reads the
// working set of parsed resources and reports what is wrong with them; it never
// resolves values, decodes bodies, or reaches a provider. An empty result means
// the configuration is valid.
//
// The stages run in order - structure, then references, then properties - and
// each gathers all of its own findings before the next is considered. Stopping
// between stages is deliberate: once references are known to be broken,
// checking properties on them would report confusing consequences of a problem
// that has already been reported.
func (p *Parser) validate() []error {
	problems := []error{}

	// Stage 1: structure. Resources that could not be understood well enough to
	// check anything else about them.
	problems = append(problems, p.validateStructure()...)
	if len(problems) > 0 {
		return problems
	}

	// Stage 2: references. Everything a resource refers to must be defined
	// somewhere in the configuration.
	problems = append(problems, p.validateReferences()...)
	if len(problems) > 0 {
		return problems
	}

	// Stage 3: properties. Every reference that resolves must name properties
	// its target's type actually has.
	problems = append(problems, p.validateProperties()...)

	return problems
}

// validateProperties reports every reference whose trailing property path names
// something its target's type does not have.
//
// It reports at most one problem per reference - the first segment that does
// not exist - because once a segment is unknown the type beyond it is unknown
// too, so later segments cannot be judged. Exhaustiveness is across references
// rather than within a single path.
func (p *Parser) validateProperties() []error {
	problems := []error{}

	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		for _, link := range meta.Links {
			resolved := p.resolveReference(link, meta.Module)
			if !resolved.found || resolved.attribute == "" {
				continue
			}

			missing := checkPropertyPath(reflect.TypeOf(resolved.target), resolved.attribute)
			if missing == "" {
				continue
			}

			problems = append(problems, errors.NewParserError(
				meta.File,
				meta.Line,
				meta.Column,
				propertyProblem(link, missing),
			))
		}
	}

	return problems
}

// validateReferences reports every reference in the configuration that names
// something defined nowhere. It reports all of them rather than the first, so
// an author is not made to fix one and rerun to discover the next.
//
// Problems are reported against the resource that holds the reference. The
// references themselves are collected without their source positions, so the
// position given is that of the resource block rather than of the reference
// within it - enough to place the problem on the right resource.
func (p *Parser) validateReferences() []error {
	problems := []error{}

	// Sort the resource keys so that problems are reported in a stable order.
	// The working set is a map, so iterating it directly would report the same
	// faults in a different order on every run.
	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		for _, link := range meta.Links {
			if p.resolveReference(link, meta.Module).found {
				continue
			}

			problems = append(problems, errors.NewParserError(
				meta.File,
				meta.Line,
				meta.Column,
				fmt.Sprintf("resource '%s' refers to '%s', which is not defined anywhere in the configuration", meta.ID, link),
			))
		}
	}

	return problems
}

// sortedResourceIDs returns the working set's keys in a stable order.
func (p *Parser) sortedResourceIDs() []string {
	ids := make([]string, 0, len(p.parsedResources.resources))
	for id := range p.parsedResources.resources {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	return ids
}

// validateStructure checks that each parsed resource is sound enough to be
// checked further. Problems found during parsing have already been reported by
// the time validation runs. This stage reports computed fields: a computed field
// that is not optional, which no configuration could satisfy, and every computed
// field set in configuration, since only the provider may set one. It also checks
// each resource body against the schema of its type, reporting attributes and
// blocks the type does not have, before any body is decoded.
func (p *Parser) validateStructure() []error {
	problems := []error{}

	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		resourceType := dereference(reflect.TypeOf(resource))

		for _, computed := range computedFields(resourceType) {
			if isOptional(computed.field) {
				continue
			}

			problems = append(problems, errors.NewParserError(
				meta.File,
				meta.Line,
				meta.Column,
				fmt.Sprintf("resource '%s' field '%s' is computed and must be optional", id, computed.path),
			))
		}

		body, ok := p.parsedResources.bodies[id]
		if !ok {
			continue
		}

		problems = append(problems, configuredComputedFields(id, body, resourceType, "")...)
		problems = append(problems, schemaProblems(id, meta, body, resource)...)
	}

	return problems
}

// schemaProblems checks body against the schema implied by the type of
// resource, without evaluating anything, and reports every problem decoding
// would find with its shape against the resource
func schemaProblems(id string, meta *types.Meta, body *hclsyntax.Body, resource any) []error {
	problems := []error{}

	for _, d := range gohcl.CheckBody(body, resource) {
		if d.Severity != hcl.DiagError {
			continue
		}

		file, line, column := meta.File, meta.Line, meta.Column
		if d.Subject != nil {
			line = d.Subject.Start.Line
			column = d.Subject.Start.Column
		}

		problems = append(problems, errors.NewParserError(
			file,
			line,
			column,
			fmt.Sprintf("resource '%s' %s: %s", id, d.Summary, d.Detail),
		))
	}

	return problems
}

// configuredComputedFields reports every computed field of t set in body,
// including inside nested blocks and object values. path is the hcl path of
// body within the resource, ending in a dot when it is not empty.
func configuredComputedFields(id string, body *hclsyntax.Body, t reflect.Type, path string) []error {
	problems := []error{}

	fields := map[string]structField{}
	for _, f := range structFields(t) {
		fields[f.name] = f
	}

	// report attributes in the order they are written
	attributes := make([]*hclsyntax.Attribute, 0, len(body.Attributes))
	for _, attribute := range body.Attributes {
		attributes = append(attributes, attribute)
	}

	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].NameRange.Start.Byte < attributes[j].NameRange.Start.Byte
	})

	for _, attribute := range attributes {
		f, ok := fields[attribute.Name]
		if !ok {
			continue
		}

		if isComputed(f.field) {
			problems = append(problems, computedFieldProblem(id, path+attribute.Name, attribute.NameRange))
			continue
		}

		problems = append(problems, configuredComputedValues(id, attribute.Expr, f.field.Type, path+attribute.Name)...)
	}

	counts := map[string]int{}
	for _, block := range body.Blocks {
		f, ok := fields[block.Type]
		if !ok {
			continue
		}

		index := counts[block.Type]
		counts[block.Type]++

		blockPath := path + block.Type
		if kind := f.field.Type.Kind(); kind == reflect.Slice || kind == reflect.Array {
			blockPath = fmt.Sprintf("%s[%d]", blockPath, index)
		}

		if isComputed(f.field) {
			problems = append(problems, computedFieldProblem(id, blockPath, block.TypeRange))
			continue
		}

		element := blockElement(f.field.Type)
		if element == nil {
			continue
		}

		problems = append(problems, configuredComputedFields(id, block.Body, element, blockPath+".")...)
	}

	return problems
}

// configuredComputedValues reports every computed field set inside an object or
// tuple expression assigned to a field of type t
func configuredComputedValues(id string, expr hclsyntax.Expression, t reflect.Type, path string) []error {
	problems := []error{}

	t = dereference(t)

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		tuple, ok := expr.(*hclsyntax.TupleConsExpr)
		if !ok {
			return problems
		}

		for i, item := range tuple.Exprs {
			problems = append(problems, configuredComputedValues(id, item, t.Elem(), fmt.Sprintf("%s[%d]", path, i))...)
		}

	case reflect.Map:
		object, ok := expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			return problems
		}

		for _, item := range object.Items {
			key := objectKey(item.KeyExpr)
			problems = append(problems, configuredComputedValues(id, item.ValueExpr, t.Elem(), fmt.Sprintf("%s[%s]", path, key))...)
		}

	case reflect.Struct:
		if blockElement(t) == nil {
			return problems
		}

		object, ok := expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			return problems
		}

		fields := map[string]structField{}
		for _, f := range structFields(t) {
			fields[f.name] = f
		}

		for _, item := range object.Items {
			name := objectKey(item.KeyExpr)

			f, ok := fields[name]
			if !ok {
				continue
			}

			if isComputed(f.field) {
				problems = append(problems, computedFieldProblem(id, path+"."+name, item.KeyExpr.Range()))
				continue
			}

			problems = append(problems, configuredComputedValues(id, item.ValueExpr, f.field.Type, path+"."+name)...)
		}
	}

	return problems
}

// objectKey returns the name of an object key, written either as a bare
// keyword or as a string
func objectKey(expr hclsyntax.Expression) string {
	if keyword := hcl.ExprAsKeyword(expr); keyword != "" {
		return keyword
	}

	value, diags := expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return ""
	}

	return value.AsString()
}

// computedFieldProblem reports a computed field set in configuration
func computedFieldProblem(id string, path string, at hcl.Range) error {
	return errors.NewParserError(
		at.Filename,
		at.Start.Line,
		at.Start.Column,
		fmt.Sprintf("resource '%s' sets computed field '%s', computed fields are set by the provider and cannot be configured", id, path),
	)
}
