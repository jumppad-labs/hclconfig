package fixtures

type MyStructPtr struct {
	Name   string       `json:"name"`
	Struct *MyStructPtr `json:"struct"`
}

var MyStructPtrDepth1JSON = `{
  "type": "fixtures.MyStructPtr",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   }
  ]
 }`

var MyStructPtrDepth2JSON = `{
  "type": "fixtures.MyStructPtr",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "*fixtures.MyStructPtr",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     }
    ]
   }
  ]
 }`

var MyStructPtrDepth3JSON = `{
  "type": "fixtures.MyStructPtr",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "*fixtures.MyStructPtr",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     },
     {
      "name": "Struct",
      "type": "*fixtures.MyStructPtr",
      "tags": "json:\"struct\"",
      "properties": [
       {
        "name": "Name",
        "type": "string",
        "tags": "json:\"name\""
       }
      ]
     }
    ]
   }
  ]
 }`

type MyStructSlice struct {
	Name   string          `json:"name"`
	Struct []MyStructSlice `json:"struct"`
}

var MyStructSliceDepth1JSON = `{
  "type": "fixtures.MyStructSlice",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   }
  ]
 }`

var MyStructSliceDepth2JSON = `{
  "type": "fixtures.MyStructSlice",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "[]fixtures.MyStructSlice",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     }
    ]
   }
  ]
 }`

type MyStructPtrSlice struct {
	Name   string         `json:"name"`
	Struct []*MyStructPtr `json:"struct"`
}

var MyStructPtrSliceDepth1JSON = `{
  "type": "fixtures.MyStructPtrSlice",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   }
  ]
 }`

var MyStructPtrSliceDepth2JSON = `{
  "type": "fixtures.MyStructPtrSlice",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "[]*fixtures.MyStructPtr",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     }
    ]
   }
  ]
 }`

type MyStructMap struct {
	Name   string                 `json:"name"`
	Struct map[string]MyStructMap `json:"struct"`
}

var MyStructMapDepth1JSON = `{
  "type": "fixtures.MyStructMap",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   }
  ]
 }`

var MyStructMapDepth2JSON = `{
  "type": "fixtures.MyStructMap",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "map[string]fixtures.MyStructMap",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     }
    ]
   }
  ]
 }`

type MyStructMapPtr struct {
	Name   string                     `json:"name"`
	Struct map[string]*MyStructMapPtr `json:"struct"`
}

var MyStructMapPtrDepth1JSON = `{
  "type": "fixtures.MyStructMapPtr",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   }
  ]
 }`

var MyStructMapPtrDepth2JSON = `{
  "type": "fixtures.MyStructMapPtr",
  "properties": [
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\""
   },
   {
    "name": "Struct",
    "type": "map[string]*fixtures.MyStructMapPtr",
    "tags": "json:\"struct\"",
    "properties": [
     {
      "name": "Name",
      "type": "string",
      "tags": "json:\"name\""
     }
    ]
   }
  ]
 }`
