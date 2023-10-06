package structs

import (
	"fmt"

	"github.com/jumppad-labs/hclconfig/types"
)

// TypeParseError defines a resource that always retuns an error when parsing
const TypeParseError = "parse_error"

type ParseError struct {
	// embedded type holding name, etc
	types.ResourceMetadata `hcl:"rm,remain"`
}

func (c *ParseError) Parse(conf types.Findable) error {
	return fmt.Errorf("boom")
}
