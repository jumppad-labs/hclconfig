package fixtures

type MyInt struct {
	Integer int `json:"integer"`
}

var MyIntJSON = `{
  "type": "fixtures.MyInt",
  "properties": [
   {
    "name": "Integer",
    "type": "int",
    "tags": "json:\"integer\""
   }
  ]
 }`

var MyInt32JSON = `{
  "type": "fixtures.MyInt",
  "properties": [
   {
    "name": "Integer",
    "type": "int32",
    "tags": "json:\"integer\""
   }
  ]
 }`

var MyInt64JSON = `{
  "type": "fixtures.MyInt",
  "properties": [
   {
    "name": "Integer",
    "type": "int64",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyIntPtr struct {
	Integer *int `json:"integer"`
}

var MyIntPtrJSON = `{
  "type": "fixtures.MyIntPtr",
  "properties": [
   {
    "name": "Integer",
    "type": "*int",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyIntSlice struct {
	Integer []int `json:"integer"`
}

var MyIntSliceJSON = `{
  "type": "fixtures.MyIntSlice",
  "properties": [
   {
    "name": "Integer",
    "type": "[]int",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyIntPtrSlice struct {
	Integer []*int `json:"integer"`
}

var MyIntPtrSliceJSON = `{
  "type": "fixtures.MyIntPtrSlice",
  "properties": [
   {
    "name": "Integer",
    "type": "[]*int",
    "tags": "json:\"integer\""
   }
  ]
 }`
