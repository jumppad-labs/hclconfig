package types

import (
	"fmt"
	"regexp"
	"strings"
)

// ResourceFQRN is the fully qualified resource name
type ResourceFQRN struct {
	// Name of the module
	Module string
	// Type of the resource
	Type string
	// Resource name
	Resource string
	// Attribute for the resource
	Attribute string
}

// ParseFQRN parses a "resource" fqrn and returns the individual components
// e.g:
//
// get the "resource" container called mine that is in the root "module"
// which also has "attributes"
// // resource.container.mine.property.value
//
// get the "resource" container called mine that is in the root "module"
// // resource.container.mine
//
// get the "output" called mine that is in the root "module"
// // output.mine
//
// get the container "resource" called mine in the "module" module2 that
// is in the "module" module1
// // module1.module2.resource.container.mine
//
// get the "output" called mine in the "module" module2 that is in the
// "module" module1
// // module1.module2.output.mine
//
// get the "module" resource called module2 in the "module" module1
// // module1.module2
//
// get the "module" resource called module1 in the root "module"
// // module1
func ParseFQRN(fqdn string) (*ResourceFQRN, error) {
	moduleName := ""
	typeName := ""
	resourceName := ""
	attribute := ""

	// first split on the resource, module, or output
	r := regexp.MustCompile(`^(module.(?P<modules>.*)\.)?(?:(?P<resource>(resource|output|variable))\.(?P<attributes>(.*)))|(?P<onlymodules>.*)`)
	match := r.FindStringSubmatch(fqdn)
	results := map[string]string{}
	for i, name := range match {
		results[r.SubexpNames()[i]] = name
	}

	if len(results) < 2 {
		return nil, fmt.Errorf("ParseFQRN expects the fqdn to be formatted as variable.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
	}

	switch results["resource"] {
	case "resource":
		resourceParts := strings.Split(results["attributes"], ".")
		if len(resourceParts) < 2 {
			return nil, fmt.Errorf("ParseFQRN expects the fqdn to be formatted as variable.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
		}

		typeName = resourceParts[0]
		resourceName = resourceParts[1]
		attribute = strings.Join(resourceParts[2:], ".")
		moduleName = results["modules"]

	case "output":
		outputParts := strings.Split(results["attributes"], ".")
		if len(outputParts) != 1 {
			return nil, fmt.Errorf("ParseFQRN expects the fqdn to be formatted as variable.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
		}

		typeName = TypeOutput
		resourceName = outputParts[0]
		moduleName = results["modules"]

	case "variable":
		varParts := strings.Split(results["attributes"], ".")
		if len(varParts) != 1 {
			return nil, fmt.Errorf("ParseFQRN expects the fqdn to be formatted as variable.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
		}

		typeName = TypeVariable
		resourceName = varParts[0]
		moduleName = results["modules"]

	default:
		if results["onlymodules"] == "" || !strings.HasPrefix(results["onlymodules"], "module.") {
			return nil, fmt.Errorf("ParseFQRN expects the fqdn to be formatted as variable.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
		}

		//module1.module2
		modules := strings.Split(results["onlymodules"], ".")

		if len(modules) == 2 {
			resourceName = modules[1]
		} else {
			moduleName = strings.Join(modules[1:len(modules)-1], ".")
			resourceName = modules[len(modules)-1]
		}

		typeName = TypeModule
	}

	return &ResourceFQRN{
		Module:    moduleName,
		Type:      typeName,
		Resource:  resourceName,
		Attribute: attribute,
	}, nil
}

// AppendParentModule creates a new FQRN by adding the parent module
// to the reference.
func (f *ResourceFQRN) AppendParentModule(parent string) ResourceFQRN {
	newFQRN := ResourceFQRN{}

	newFQRN.Module = f.Module
	if parent != "" {
		newFQRN.Module = fmt.Sprintf("%s.%s", parent, f.Module)
		newFQRN.Module = strings.TrimSuffix(newFQRN.Module, ".")
	}

	newFQRN.Resource = f.Resource
	newFQRN.Type = f.Type
	newFQRN.Attribute = f.Attribute

	return newFQRN
}

// FQDNFromResource returns the ResourceFQDN for the given Resource
func FQDNFromResource(r Resource) *ResourceFQRN {
	return &ResourceFQRN{
		Module:   r.Metadata().Module,
		Resource: r.Metadata().Name,
		Type:     r.Metadata().Type,
	}
}

func (f ResourceFQRN) String() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	attrPart := ""
	if f.Attribute != "" {
		attrPart = fmt.Sprintf(".%s", f.Attribute)
	}

	if f.Type == TypeOutput {
		return fmt.Sprintf("%s%s.%s", modulePart, f.Type, f.Resource)
	}

	if f.Type == TypeModule {
		if f.Module == "" {
			return fmt.Sprintf("module.%s", f.Resource)
		}

		return fmt.Sprintf("%s%s", modulePart, f.Resource)
	}

	return fmt.Sprintf("%sresource.%s.%s%s", modulePart, f.Type, f.Resource, attrPart)
}
