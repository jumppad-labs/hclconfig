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

type MyBoolMap struct {
	Boolean map[string]bool `json:"boolean"`
}

var MyBoolMapJSON = `{
  "type": "fixtures.MyBoolMap",
  "properties": [
   {
    "name": "Boolean",
    "type": "map[string]bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`

type MyBoolPtrMap struct {
	Boolean map[string]*bool `json:"boolean"`
}

var MyBoolPtrMapJSON = `{
  "type": "fixtures.MyBoolPtrMap",
  "properties": [
   {
    "name": "Boolean",
    "type": "map[string]*bool",
    "tags": "json:\"boolean\""
   }
  ]
 }`
