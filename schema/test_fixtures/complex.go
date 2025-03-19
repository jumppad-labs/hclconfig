package fixtures

var MyComplex128JSON = `{
  "type": "fixtures.MyComplex",
  "properties": [
   {
    "name": "Complex",
    "type": "complex128",
    "tags": "json:\"complex\""
   }
  ]
 }`

var MyComplex64JSON = `{
  "type": "fixtures.MyComplex",
  "properties": [
   {
    "name": "Complex",
    "type": "complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`

type MyComplexPtr struct {
	Complex *complex64 `json:"complex"`
}

var MyComplexPtrJSON = `{
  "type": "fixtures.MyComplexPtr",
  "properties": [
   {
    "name": "Complex",
    "type": "*complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`

type MyComplexSlice struct {
	Complex []complex64 `json:"complex"`
}

var MyComplexSliceJSON = `{
  "type": "fixtures.MyComplexSlice",
  "properties": [
   {
    "name": "Complex",
    "type": "[]complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`

type MyComplexPtrSlice struct {
	Complex []*complex64 `json:"complex"`
}

var MyComplexPtrSliceJSON = `{
  "type": "fixtures.MyComplexPtrSlice",
  "properties": [
   {
    "name": "Complex",
    "type": "[]*complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`

type MyComplexMap struct {
	Complex map[string]complex64 `json:"complex"`
}

var MyComplexMapJSON = `{
  "type": "fixtures.MyComplexMap",
  "properties": [
   {
    "name": "Complex",
    "type": "map[string]complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`

type MyComplexMapPtr struct {
	Complex map[string]*complex64 `json:"complex"`
}

var MyComplexMapPtrJSON = `{
  "type": "fixtures.MyComplexMapPtr",
  "properties": [
   {
    "name": "Complex",
    "type": "map[string]*complex64",
    "tags": "json:\"complex\""
   }
  ]
 }`
