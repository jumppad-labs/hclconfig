package fixtures

type MyString struct {
	String string `json:"string"`
}

var MyStringJSON = `{
  "type": "fixtures.MyString",
  "properties": [
   {
    "name": "String",
    "type": "string",
    "tags": "json:\"string\""
   }
  ]
 }`

type MyStringPtr struct {
	String *string `json:"string"`
}

var MyStringPtrJSON = `{
  "type": "fixtures.MyStringPtr",
  "properties": [
   {
    "name": "String",
    "type": "*string",
    "tags": "json:\"string\""
   }
  ]
 }`

type MyStringSlice struct {
	String []string `json:"string"`
}

var MyStringSliceJSON = `{
  "type": "fixtures.MyStringSlice",
  "properties": [
   {
    "name": "String",
    "type": "[]string",
    "tags": "json:\"string\""
   }
  ]
 }`

type MyStringPtrSlice struct {
	String []*string `json:"string"`
}

var MyStringPtrSliceJSON = `{
  "type": "fixtures.MyStringPtrSlice",
  "properties": [
   {
    "name": "String",
    "type": "[]*string",
    "tags": "json:\"string\""
   }
  ]
 }`
