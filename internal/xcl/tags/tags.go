// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

// Package tags parses the `xcl` struct tags that describe how configuration
// maps onto Go types. It is shared by the decoder in gohcl and the value
// conversion in gocty so that both read the same tag syntax.
package tags

import (
	"fmt"
	"strings"
)

// Name is the struct tag key that XCL reads, e.g.
//
//	Address string `xcl:"address,optional,computed"`
const Name = "xcl"

// Field kinds, a tag has at most one, fields without a kind are attributes
const (
	KindAttr     = "attr"
	KindBlock    = "block"
	KindLabel    = "label"
	KindRemain   = "remain"
	KindBody     = "body"
	KindOptional = "optional"
)

// Field options that XCL uses, the decoder accepts them but does not act on them
const (
	// OptionComputed marks a field that is owned by the provider
	OptionComputed = "computed"

	// OptionKey marks a field that identifies an element in a list of blocks
	OptionKey = "key"
)

// Field is a parsed `xcl` struct tag
type Field struct {
	// Name is the name configuration uses for the field
	Name string

	// Kind is how the field is decoded, one of the Kind constants
	Kind string

	// Computed is true when the tag has the computed option
	Computed bool

	// Key is true when the tag has the key option
	Key bool
}

// Parse parses the value of an `xcl` struct tag. The tag is the field
// name followed by comma separated options, at most one of which is a kind.
func Parse(tag string) (Field, error) {
	parts := strings.Split(tag, ",")

	ft := Field{
		Name: strings.TrimSpace(parts[0]),
	}

	for _, option := range parts[1:] {
		option = strings.TrimSpace(option)

		switch option {
		case KindAttr, KindBlock, KindLabel, KindRemain, KindBody, KindOptional:
			if ft.Kind != "" {
				return Field{}, fmt.Errorf("xcl field tag %q has more than one kind, %q and %q", tag, ft.Kind, option)
			}

			ft.Kind = option
		case OptionComputed:
			ft.Computed = true
		case OptionKey:
			ft.Key = true
		default:
			return Field{}, fmt.Errorf("invalid xcl field tag option %q in %q", option, tag)
		}
	}

	if ft.Kind == "" {
		ft.Kind = KindAttr
	}

	return ft, nil
}
