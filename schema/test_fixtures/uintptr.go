package fixtures

type MyUIntPtrT struct {
	Integer uintptr `json:"integer"`
}

var MyUIntPtrTJSON = `{
  "type": "fixtures.MyUIntPtrT",
  "properties": [
   {
    "name": "Integer",
    "type": "uintptr",
    "tags": "json:\"integer\""
   }
  ]
 }`
