package fixtures

type MyFloat32 struct {
	Float float32 `json:"float"`
}

var MyFloat32JSON = `{
  "type": "fixtures.MyFloat",
  "properties": [
   {
    "name": "Float",
    "type": "float32",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloat64 struct {
	Float float64 `json:"float"`
}

var MyFloat64JSON = `{
  "type": "fixtures.MyFloat",
  "properties": [
   {
    "name": "Float",
    "type": "float64",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloatPtr struct {
	Float *float64 `json:"float"`
}

var MyFloatPtrJSON = `{
  "type": "fixtures.MyFloatPtr",
  "properties": [
   {
    "name": "Float",
    "type": "*float64",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloatSlice struct {
	Float []float64 `json:"float"`
}

var MyFloatSliceJSON = `{
  "type": "fixtures.MyFloatSlice",
  "properties": [
   {
    "name": "Float",
    "type": "[]float64",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloatPtrSlice struct {
	Float []*float64 `json:"float"`
}

var MyFloatPtrSliceJSON = `{
  "type": "fixtures.MyFloatPtrSlice",
  "properties": [
   {
    "name": "Float",
    "type": "[]*float64",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloatMap struct {
	Float map[string]float64 `json:"float"`
}

var MyFloatMapJSON = `{
  "type": "fixtures.MyFloatMap",
  "properties": [
   {
    "name": "Float",
    "type": "map[string]float64",
    "tags": "json:\"float\""
   }
  ]
 }`

type MyFloatMapPtr struct {
	Float map[string]*float64 `json:"float"`
}

var MyFloatMapPtrJSON = `{
  "type": "fixtures.MyFloatMapPtr",
  "properties": [
   {
    "name": "Float",
    "type": "map[string]*float64",
    "tags": "json:\"float\""
   }
  ]
 }`
