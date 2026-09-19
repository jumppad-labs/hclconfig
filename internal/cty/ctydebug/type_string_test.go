package ctydebug_test

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/cty/ctydebug"
	"github.com/jumppad-labs/xcl/internal/cty"
)

func ExampleTypeString() {
	ty := cty.Object(map[string]cty.Type{
		"source_account":  cty.String,
		"fee":             cty.Number,
		"sequence_number": cty.Number,
		"time_bounds":     cty.Tuple([]cty.Type{cty.String, cty.String}),
		"memo":            cty.DynamicPseudoType,
		"payments": cty.List(cty.Object(map[string]cty.Type{
			"destination_account": cty.String,
			"asset":               cty.String,
			"amount":              cty.Number,
		})),
	})

	fmt.Print(ctydebug.TypeString(ty))

	// Output:
	// cty.Object(map[string]cty.Type{
	//     "fee": cty.Number,
	//     "memo": cty.DynamicPseudoType,
	//     "payments": cty.List(
	//         cty.Object(map[string]cty.Type{
	//             "amount": cty.Number,
	//             "asset": cty.String,
	//             "destination_account": cty.String,
	//         }),
	//     ),
	//     "sequence_number": cty.Number,
	//     "source_account": cty.String,
	//     "time_bounds": cty.Tuple([]cty.Type{
	//         cty.String,
	//         cty.String,
	//     }),
	// })
}
