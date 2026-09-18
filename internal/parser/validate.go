package parser

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/types"
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
// the time validation runs, so this stage currently adds nothing of its own and
// exists to anchor the stage ordering.
func (p *Parser) validateStructure() []error {
	return nil
}
