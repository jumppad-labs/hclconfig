package fixtures

type MyUInt struct {
	Integer uint `json:"integer"`
}

var MyUIntJSON = `{
  "type": "fixtures.MyUInt",
  "properties": [
   {
    "name": "Integer",
    "type": "uint",
    "tags": "json:\"integer\""
   }
  ]
 }`

var MyUInt32JSON = `{
  "type": "fixtures.MyUInt",
  "properties": [
   {
    "name": "Integer",
    "type": "uint32",
    "tags": "json:\"integer\""
   }
  ]
 }`

var MyUInt64JSON = `{
  "type": "fixtures.MyUInt",
  "properties": [
   {
    "name": "Integer",
    "type": "uint64",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyUIntPtr struct {
	Integer *uint `json:"integer"`
}

var MyUIntPtrJSON = `{
  "type": "fixtures.MyUIntPtr",
  "properties": [
   {
    "name": "Integer",
    "type": "*uint",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyUIntSlice struct {
	Integer []uint `json:"integer"`
}

var MyUIntSliceJSON = `{
  "type": "fixtures.MyUIntSlice",
  "properties": [
   {
    "name": "Integer",
    "type": "[]uint",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyUIntPtrSlice struct {
	Integer []*uint `json:"integer"`
}

var MyUIntPtrSliceJSON = `{
  "type": "fixtures.MyUIntPtrSlice",
  "properties": [
   {
    "name": "Integer",
    "type": "[]*uint",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyUIntMap struct {
	Integer map[string]uint `json:"integer"`
}

var MyUIntMapJSON = `{
  "type": "fixtures.MyUIntMap",
  "properties": [
   {
    "name": "Integer",
    "type": "map[string]uint",
    "tags": "json:\"integer\""
   }
  ]
 }`

type MyUIntMapPtr struct {
	Integer map[string]*uint `json:"integer"`
}

var MyUIntMapPtrJSON = `{
  "type": "fixtures.MyUIntMapPtr",
  "properties": [
   {
    "name": "Integer",
    "type": "map[string]*uint",
    "tags": "json:\"integer\""
   }
  ]
 }`