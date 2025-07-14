package resources

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jumppad-labs/hclconfig/types"
)

// FQRN is the fully qualified resource name
type FQRN struct {
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
// get the "local" called mine that is in the root "module"
// // local.mine
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
func ParseFQRN(fqrn string) (*FQRN, error) {
	moduleName := ""
	typeName := ""
	resourceName := ""
	attribute := ""

	// first split on the resource, module, or output
	r := regexp.MustCompile(`^(module.(?P<modules>.*)\.)?(?:(?P<resource>(resource|output|local|variable|provider))\.(?P<attributes>(.*)))|(?P<onlymodules>.*)`)
	match := r.FindStringSubmatch(fqrn)
	results := map[string]string{}
	for i, name := range match {
		results[r.SubexpNames()[i]] = name
	}

	if len(results) < 2 {
		return nil, errors.New(formatErrorString(fqrn))
	}

	switch results["resource"] {
	case "resource":
		resourceParts := strings.Split(results["attributes"], ".")
		if len(resourceParts) < 2 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		typeName = resourceParts[0]
		resourceName = resourceParts[1]
		attribute = strings.Join(resourceParts[2:], ".")
		moduleName = results["modules"]

	case "local":
		fallthrough

	case "output":
		outputParts := strings.Split(results["attributes"], ".")

		typeName = results["resource"]
		resourceName = outputParts[0]
		moduleName = results["modules"]
		attribute = strings.Join(outputParts[1:], ".")

		// check if the fqdn is using parentheses based selectors []
		indexR := regexp.MustCompile(`(?P<name>.*)\[(?P<index>\d+)\]`)
		indexMatch := indexR.FindStringSubmatch(outputParts[0])
		indexResults := map[string]string{}
		for i, name := range indexMatch {
			indexResults[indexR.SubexpNames()[i]] = name
		}

		if i := indexResults["index"]; i != "" {
			attribute = fmt.Sprintf("%s.%s", i, attribute)
			attribute = strings.Trim(attribute, ".")
			resourceName = indexResults["name"]
		}

	case TypeVariable:
		varParts := strings.Split(results["attributes"], ".")
		if len(varParts) != 1 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		typeName = TypeVariable
		resourceName = varParts[0]
		moduleName = results["modules"]

	case TypeProvider:
		providerParts := strings.Split(results["attributes"], ".")
		if len(providerParts) < 1 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		typeName = TypeProvider
		resourceName = providerParts[0]
		moduleName = results["modules"]
		attribute = strings.Join(providerParts[1:], ".")

	default:
		if results["onlymodules"] == "" || !strings.HasPrefix(results["onlymodules"], "module.") {
			return nil, errors.New(formatErrorString(fqrn))
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

	return &FQRN{
		Module:    moduleName,
		Type:      typeName,
		Resource:  resourceName,
		Attribute: attribute,
	}, nil
}

func formatErrorString(fqdn string) string {
	return fmt.Sprintf("ParseFQRN expects the fqdn to be formatted as variable.name, local.name, output.name, resource.type.name, module.module1.module2, or module.module1.module2.resource.type.name. The fqrn: %s, does not contain a resource type", fqdn)
}

// AppendParentModule creates a new FQRN by adding the parent module
// to the reference.
func (f *FQRN) AppendParentModule(parent string) FQRN {
	newFQRN := FQRN{}

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

// FQRNFromResource returns the ResourceFQDN for the given Resource
func FQRNFromResource(r types.Resource) *FQRN {
	return &FQRN{
		Module:   r.Metadata().Module,
		Resource: r.Metadata().Name,
		Type:     r.Metadata().Type,
	}
}

func (f FQRN) String() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	attrPart := ""
	if f.Attribute != "" {
		attrPart = fmt.Sprintf(".%s", f.Attribute)
	}

	if f.Type == TypeOutput || f.Type == TypeLocal || f.Type == TypeVariable || f.Type == TypeProvider {
		return fmt.Sprintf("%s%s.%s%s", modulePart, f.Type, f.Resource, attrPart)
	}

	if f.Type == TypeModule {
		if f.Module == "" {
			return fmt.Sprintf("module.%s", f.Resource)
		}

		return fmt.Sprintf("%s%s", modulePart, f.Resource)
	}

	return fmt.Sprintf("%sresource.%s.%s%s", modulePart, f.Type, f.Resource, attrPart)
}

func (f FQRN) StringWithoutAttribute() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	if f.Type == TypeOutput || f.Type == TypeLocal || f.Type == TypeVariable {
		return fmt.Sprintf("%s%s.%s", modulePart, f.Type, f.Resource)
	}

	if f.Type == TypeModule {
		if f.Module == "" {
			return fmt.Sprintf("module.%s", f.Resource)
		}

		return fmt.Sprintf("%s%s", modulePart, f.Resource)
	}

	return fmt.Sprintf("%sresource.%s.%s", modulePart, f.Type, f.Resource)
}
