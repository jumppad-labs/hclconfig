package ctydebug_test

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/cty/ctydebug"
	"github.com/jumppad-labs/xcl/internal/cty"
)

func ExampleValueString() {
	v := cty.ObjectVal(map[string]cty.Value{
		"source_account":  cty.StringVal("GBAMSPIE6NRUVRV4ZIPI2ZFR3NAIAIXQHGCMVLPSVQCM46IPWTHEVOID"),
		"fee":             cty.NumberIntVal(2),
		"sequence_number": cty.NumberIntVal(4523452343),
		"time_bounds": cty.TupleVal([]cty.Value{
			cty.StringVal("2019-12-14T00:00:00Z"),
			cty.StringVal("2019-12-14T00:05:00Z"),
		}),
		"memo": cty.MapValEmpty(cty.String),
		"payments": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"destination_account": cty.StringVal("GALAXYVOIDAOPZTDLHILAJQKCVVFMD4IKLXLSZV5YHO7VY74IWZILUTO"),
				"asset":               cty.StringVal("XLM"),
				"amount":              cty.NumberIntVal(55442098181),
			}),
		}),
	})

	fmt.Print(ctydebug.ValueString(v))

	// Output:
	// cty.ObjectVal(map[string]cty.Value{
	//     "fee": cty.NumberIntVal(2),
	//     "memo": cty.MapValEmpty(cty.String),
	//     "payments": cty.ListVal([]cty.Value{
	//         cty.ObjectVal(map[string]cty.Value{
	//             "amount": cty.NumberIntVal(5.5442098181e+10),
	//             "asset": cty.StringVal("XLM"),
	//             "destination_account": cty.StringVal("GALAXYVOIDAOPZTDLHILAJQKCVVFMD4IKLXLSZV5YHO7VY74IWZILUTO"),
	//         }),
	//     }),
	//     "sequence_number": cty.NumberIntVal(4.523452343e+09),
	//     "source_account": cty.StringVal("GBAMSPIE6NRUVRV4ZIPI2ZFR3NAIAIXQHGCMVLPSVQCM46IPWTHEVOID"),
	//     "time_bounds": cty.TupleVal([]cty.Value{
	//         cty.StringVal("2019-12-14T00:00:00Z"),
	//         cty.StringVal("2019-12-14T00:05:00Z"),
	//     }),
	// })
}
