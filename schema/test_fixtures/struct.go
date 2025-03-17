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

type MyStructPtrSlice struct {
	Name   string         `json:"name"`
	Struct []*MyStructPtr `json:"struct"`
}

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
