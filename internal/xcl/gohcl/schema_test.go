// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/jumppad-labs/xcl/internal/xcl"
)

func TestImpliedBodySchema(t *testing.T) {
	tests := []struct {
		val         interface{}
		wantSchema  *hcl.BodySchema
		wantPartial bool
	}{
		{
			struct{}{},
			&hcl.BodySchema{},
			false,
		},
		{
			struct {
				Ignored bool
			}{},
			&hcl.BodySchema{},
			false,
		},
		{
			struct {
				Attr1 bool `xcl:"attr1"`
				Attr2 bool `xcl:"attr2"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "attr1",
						Required: true,
					},
					{
						Name:     "attr2",
						Required: true,
					},
				},
			},
			false,
		},
		{
			struct {
				Attr *bool `xcl:"attr,attr"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "attr",
						Required: false,
					},
				},
			},
			false,
		},
		{
			struct {
				Thing struct{} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type: "thing",
					},
				},
			},
			false,
		},
		{
			struct {
				Thing struct {
					Type string `xcl:"type,label"`
					Name string `xcl:"name,label"`
				} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "thing",
						LabelNames: []string{"type", "name"},
					},
				},
			},
			false,
		},
		{
			struct {
				Thing []struct {
					Type string `xcl:"type,label"`
					Name string `xcl:"name,label"`
				} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "thing",
						LabelNames: []string{"type", "name"},
					},
				},
			},
			false,
		},
		{
			struct {
				Thing *struct {
					Type string `xcl:"type,label"`
					Name string `xcl:"name,label"`
				} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "thing",
						LabelNames: []string{"type", "name"},
					},
				},
			},
			false,
		},
		{
			struct {
				Thing struct {
					Name      string `xcl:"name,label"`
					Something string `xcl:"something"`
				} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "thing",
						LabelNames: []string{"name"},
					},
				},
			},
			false,
		},
		{
			struct {
				Doodad string `xcl:"doodad"`
				Thing  struct {
					Name string `xcl:"name,label"`
				} `xcl:"thing,block"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "doodad",
						Required: true,
					},
				},
				Blocks: []hcl.BlockHeaderSchema{
					{
						Type:       "thing",
						LabelNames: []string{"name"},
					},
				},
			},
			false,
		},
		{
			struct {
				Doodad string `xcl:"doodad"`
				Config string `xcl:",remain"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "doodad",
						Required: true,
					},
				},
			},
			true,
		},
		{
			struct {
				Expr hcl.Expression `xcl:"expr"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "expr",
						Required: false,
					},
				},
			},
			false,
		},
		{
			struct {
				Meh string `xcl:"meh,optional"`
			}{},
			&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{
						Name:     "meh",
						Required: false,
					},
				},
			},
			false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v", test.val), func(t *testing.T) {
			schema, partial := ImpliedBodySchema(test.val)
			if !reflect.DeepEqual(schema, test.wantSchema) {
				t.Errorf(
					"wrong schema\ngot:  %s\nwant: %s",
					spew.Sdump(schema), spew.Sdump(test.wantSchema),
				)
			}

			if partial != test.wantPartial {
				t.Errorf(
					"wrong partial flag\ngot:  %#v\nwant: %#v",
					partial, test.wantPartial,
				)
			}
		})
	}
}
