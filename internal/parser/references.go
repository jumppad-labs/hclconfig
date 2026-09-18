package parser

import (
	"github.com/jumppad-labs/xcl/internal/resources"
)

// resolvedReference is the outcome of matching a reference string against
// everything the configuration defines.
type resolvedReference struct {
	// target is the resource the reference names, when it was found.
	target any

	// key is the working-set key the reference resolved to, which carries the
	// module scope the reference was resolved in.
	key string

	// attribute is the property path left over once the part naming the
	// resource has been taken off. It is empty when the reference names only a
	// resource. Stage 3 checks this against the target's type.
	attribute string

	// found reports whether the named resource exists anywhere in the
	// configuration.
	found bool
}

// resolveReference answers whether the thing a reference names exists, and
// hands back the property path left over.
//
// References carry a trailing property path while the working-set keys never
// do, so the attribute is stripped *before* the lookup rather than after -
// doing it the other way round is why the existing dependency lookup misses.
//
// A reference made from inside a module is written as though it were at the top
// level, so it is resolved against the referring resource's module scope first
// and against the global scope second. Resolving in that order is what lets a
// module refer to its own resources while still being able to name anything
// declared outside it.
func (p *Parser) resolveReference(reference string, fromModule string) resolvedReference {
	fqrn, err := resources.ParseFQRN(reference)
	if err != nil {
		// A reference that cannot be parsed names nothing that exists.
		return resolvedReference{found: false}
	}

	base := fqrn.StringWithoutAttribute()

	// A reference from inside a module names things in that module first.
	if fromModule != "" {
		scoped := "module." + fromModule + "." + base
		if target, ok := p.parsedResources.resources[scoped]; ok {
			return resolvedReference{
				target:    target,
				key:       scoped,
				attribute: fqrn.Attribute,
				found:     true,
			}
		}
	}

	if target, ok := p.parsedResources.resources[base]; ok {
		return resolvedReference{
			target:    target,
			key:       base,
			attribute: fqrn.Attribute,
			found:     true,
		}
	}

	return resolvedReference{attribute: fqrn.Attribute, found: false}
}
