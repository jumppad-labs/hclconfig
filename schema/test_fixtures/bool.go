package fixtures

type MyBool struct {
	Boolean bool `json:"boolean"`
}

var MyBoolJSON = `{
  "type": "fixtures.MyBool",
  "properties": [
   {
    "name": "Boolean",
    "type": "bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`

type MyBoolPtr struct {
	Boolean *bool `json:"boolean"`
}

var MyBoolPtrJSON = `{
  "type": "fixtures.MyBoolPtr",
  "properties": [
   {
    "name": "Boolean",
    "type": "*bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`

type MyBoolSlice struct {
	Boolean []bool `json:"boolean"`
}

var MyBoolSliceJSON = `{
  "type": "fixtures.MyBoolSlice",
  "properties": [
   {
    "name": "Boolean",
    "type": "[]bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`

type MyBoolPtrSlice struct {
	Boolean []*bool `json:"boolean"`
}

var MyBoolPtrSliceJSON = `{
  "type": "fixtures.MyBoolPtrSlice",
  "properties": [
   {
    "name": "Boolean",
    "type": "[]*bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`