package types

import (
	"fmt"
	"strings"
)

type ResourceFQDN struct {
	// Name of the module
	Module string
	// Type of the resource
	Type string
	// Resource name
	Resource string
	// Attribute for the resource
	Attribute string
}

// ParseFQDN parses a resource fqdn and returns the individual components
// e.g:
// module.module1.resource.container.mine
// module.module1.module2.resource.container.mine
// module.module1.module2
// module.module1.module2.output.mine
func ParseFQDN(fqdn string) (*ResourceFQDN, error) {
	noResource := false
	moduleName := ""
	typeName := ""
	resourceName := ""
	attribute := ""

	// first split on the resource
	parts := strings.Split(fqdn, "resource.")
	if len(parts) < 2 {
		noResource = true
	}

	if !noResource {
		// then split into type and name
		resourceParts := strings.Split(parts[1], ".")
		if len(resourceParts) < 2 {
			return nil, fmt.Errorf("ParseFQDN expects the fqdn to be formatted as resource.type.name or module.name.resource.type.name. The fqdn: %s, does not contain a resource type", fqdn)
		}

		typeName = resourceParts[0]
		resourceName = resourceParts[1]
		attribute = strings.Join(resourceParts[2:], ".")
	}

	// now attempt to parse the module
	moduleParts := strings.Split(parts[0], "module.")
	if len(moduleParts) > 1 {

		// if we have a module does it reference an output
		outputParts := strings.Split(moduleParts[1], "output.")
		if len(outputParts) > 1 {
			moduleName = strings.TrimSuffix(outputParts[0], ".")
			resourceName = outputParts[1]
			typeName = TypeOutput
			attribute = "value"
		} else {
			// return only the module name
			moduleName = strings.TrimSuffix(moduleParts[1], ".")
		}
	}

	if moduleName == "" && noResource {
		return nil, fmt.Errorf("ParseFQDN expects the fqdn to be formatted as resource.type.name or module.name.resource.type.name. The fqdn: %s, does not contain a module or resource identifier", fqdn)
	}

	return &ResourceFQDN{
		Module:    moduleName,
		Type:      typeName,
		Resource:  resourceName,
		Attribute: attribute,
	}, nil
}

// FQDNFromResource returns the ResourceFQDN for the given Resource
func FQDNFromResource(r Resource) *ResourceFQDN {
	return &ResourceFQDN{
		Module:   r.Metadata().Module,
		Resource: r.Metadata().Name,
		Type:     r.Metadata().Type,
	}
}

func (f ResourceFQDN) String() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	if f.Type == TypeOutput {
		return fmt.Sprintf("%s%s.%s", modulePart, f.Type, f.Resource)
	}

	return fmt.Sprintf("%sresource.%s.%s", modulePart, f.Type, f.Resource)
}
