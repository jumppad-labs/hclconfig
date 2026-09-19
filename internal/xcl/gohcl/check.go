// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"fmt"
	"reflect"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// CheckBody checks the given body for attributes and blocks that the type of
// val does not have, following the schema DecodeBody would use (remain fields,
// nested and repeated blocks), without decoding anything or evaluating any
// expression. val must be a struct, or a pointer to one.
//
// Only unknown attributes and blocks are reported. Missing required attributes
// and blocks, repeated blocks where only one is allowed, and problems with the
// values of expressions are left for DecodeBody to report.
func CheckBody(body hcl.Body, val any) hcl.Diagnostics {
	ty := reflect.TypeOf(val)
	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	if ty.Kind() != reflect.Struct {
		panic(fmt.Sprintf("given value must be struct, not %T", val))
	}

	return checkBodyAgainstStruct(body, ty)
}

// checkBodyAgainstStruct mirrors decodeBodyToStruct, it follows remain fields
// and nested blocks, collecting the unknown attributes and blocks decoding
// would report
func checkBodyAgainstStruct(body hcl.Body, ty reflect.Type) hcl.Diagnostics {
	schema, partial := ImpliedBodySchema(reflect.New(ty).Interface())

	// Missing attributes are left for decoding to report, only unknown
	// attributes and blocks are checked
	for i := range schema.Attributes {
		schema.Attributes[i].Required = false
	}

	var content *hcl.BodyContent
	var leftovers hcl.Body
	var diags hcl.Diagnostics
	if partial {
		content, leftovers, diags = body.PartialContent(schema)
	} else {
		content, diags = body.Content(schema)
	}
	if content == nil {
		return diags
	}

	tags := getFieldTags(ty)

	if tags.Body != nil {
		field := ty.Field(*tags.Body)
		if !bodyType.AssignableTo(field.Type) {
			diags = append(diags, checkBodyAgainstValueType(body, field.Type)...)
		}
	}

	if tags.Remain != nil {
		field := ty.Field(*tags.Remain)
		switch {
		case bodyType.AssignableTo(field.Type):
			// the leftovers are kept as a body, anything is allowed
		case attrsType.AssignableTo(field.Type):
			_, attrsDiags := leftovers.JustAttributes()
			diags = append(diags, attrsDiags...)
		default:
			diags = append(diags, checkBodyAgainstValueType(leftovers, field.Type)...)
		}
	}

	blocksByType := content.Blocks.ByType()

	for typeName, fieldIdx := range tags.Blocks {
		blocks := blocksByType[typeName]
		field := ty.Field(fieldIdx)

		elemType := field.Type
		if elemType.Kind() == reflect.Slice {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		for _, block := range blocks {
			diags = append(diags, checkBodyAgainstValueType(block.Body, elemType)...)
		}
	}

	return diags
}

// checkBodyAgainstValueType mirrors decodeBodyToValue, a struct is checked
// against its schema, a map only allows attributes
func checkBodyAgainstValueType(body hcl.Body, ty reflect.Type) hcl.Diagnostics {
	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	switch ty.Kind() {
	case reflect.Struct:
		return checkBodyAgainstStruct(body, ty)
	case reflect.Map:
		_, diags := body.JustAttributes()
		return diags
	default:
		panic(fmt.Sprintf("target value must be pointer to struct or map, not %s", ty.String()))
	}
}
