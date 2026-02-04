package parser

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// processAttribute extracts the resources out of the HCL
// attribute like a function or resource parameter so we can determine
// which attributes are lazy evaluated due to dependency on another resource.
// Attributes can be nested, therefore this function needs to return an array of
// attributes
// examples:
// something = resource.mine.attr
// something = resource.mine.array.0.attr
// something = env(resource.mine.attr)
// something = "${resource.mine.attr}"
// something = "testing/${resource.mine.attr}"
// something = "testing/${env(resource.mine.attr)}"
// something = resource.mine.attr == "abc" ? resource.mine.attr : "abc"
func processExpr(expr hclsyntax.Expression) ([]string, error) {
	resources := []string{}

	switch ex := expr.(type) {
	// a template is a mix of functions, scope expressions and literals
	// we need to check each part
	case *hclsyntax.TemplateExpr:
		for _, v := range ex.Parts {
			res, err := processExpr(v)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	case *hclsyntax.TemplateWrapExpr:
		res, err := processExpr(ex.Wrapped)
		if err != nil {
			return nil, err
		}

		resources = append(resources, res...)

	// function call expressions are user defined functions
	// myfunction(resource.container.base.name)
	case *hclsyntax.FunctionCallExpr:
		for _, v := range ex.Args {
			res, err := processExpr(v)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	// a function can contain args that may also have an expression
	case *hclsyntax.ScopeTraversalExpr:
		ref, err := processScopeTraversal(ex)
		if err != nil {
			return nil, err
		}

		// only add if a resource has been returned
		if ref != "" {
			resources = append(resources, ref)
		}

	case *hclsyntax.ObjectConsExpr:
		for _, v := range ex.Items {
			res, err := processExpr(v.ValueExpr)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	case *hclsyntax.TupleConsExpr:
		for _, v := range ex.Exprs {
			res, err := processExpr(v)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	// conditional expressions are like if statements
	// resource.container.base.name == "hello" ? "this" : "that"
	case *hclsyntax.ConditionalExpr:
		conditions, err := processExpr(ex.Condition)
		if err != nil {
			return nil, err
		}
		resources = append(resources, conditions...)

		trueResults, err := processExpr(ex.TrueResult)
		if err != nil {
			return nil, err
		}
		resources = append(resources, trueResults...)

		falseResults, err := processExpr(ex.FalseResult)
		if err != nil {
			return nil, err
		}
		resources = append(resources, falseResults...)
	// binary expressions are two part comparisons
	// resource.container.base.name == "hello"
	// resource.container.base.name != "hello"
	// resource.container.base.name > 3
	case *hclsyntax.BinaryOpExpr:
		lhs, err := processExpr(ex.LHS)
		if err != nil {
			return nil, err
		}
		resources = append(resources, lhs...)

		rhs, err := processExpr(ex.RHS)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rhs...)
	case *hclsyntax.SplatExpr:
		ref, err := processExpr(ex.Source)
		if err != nil {
			return nil, err
		}

		// only add if a resource has been returned
		if len(ref) > 0 {
			resources = append(resources, ref...)
		}
	}

	return resources, nil
}

func processScopeTraversal(expr *hclsyntax.ScopeTraversalExpr) (string, error) {
	strExpression := ""
	for i, t := range expr.Traversal {
		if i == 0 {
			strExpression += t.(hcl.TraverseRoot).Name

			// if this is not a resource reference quit
			if strExpression != "resource" && strExpression != "module" && strExpression != "variable" && strExpression != "output" {
				return "", nil
			}
		} else {
			// does this exist in the context
			switch tt := t.(type) {
			case hcl.TraverseAttr:
				strExpression += "." + tt.Name
			case hcl.TraverseIndex:
				// Handle both string and numeric indices
				switch tt.Key.Type() {
				case cty.String:
					strExpression += "[\"" + tt.Key.AsString() + "\"]"
				case cty.Number:
					strExpression += "[" + tt.Key.AsBigFloat().String() + "]"
				default:
					// For other types, use the bigfloat representation as fallback
					strExpression += "[" + tt.Key.AsBigFloat().String() + "]"
				}
			}
		}
	}

	// add to the references collection and replace with a nil value
	// we will resolve these references before processing
	return strExpression, nil
}
