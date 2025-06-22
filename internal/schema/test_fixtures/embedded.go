package fixtures

import "github.com/jumppad-labs/hclconfig/types"

type Embedded struct {
	types.ResourceBase
	Name string `json:"name" hcl:"name"`
}

var EmbeddedJson = `{
  "type": "fixtures.Embedded",
  "properties": [
   {
    "name": "ResourceBase",
    "type": "types.ResourceBase",
    "properties": [
     {
      "name": "DependsOn",
      "type": "[]string",
      "tags": "hcl:\"depends_on,optional\" json:\"depends_on,omitempty\""
     },
     {
      "name": "Disabled",
      "type": "bool",
      "tags": "hcl:\"disabled,optional\" json:\"disabled,omitempty\""
     },
     {
      "name": "Meta",
      "type": "types.Meta",
      "tags": "hcl:\"meta,optional\" json:\"meta,omitempty\"",
      "properties": [
       {
        "name": "ID",
        "type": "string",
        "tags": "hcl:\"id,optional\" json:\"id\""
       },
       {
        "name": "Name",
        "type": "string",
        "tags": "hcl:\"name,optional\" json:\"name\""
       },
       {
        "name": "Type",
        "type": "string",
        "tags": "hcl:\"type,optional\" json:\"type\""
       },
       {
        "name": "Module",
        "type": "string",
        "tags": "hcl:\"module,optional\" json:\"module,omitempty\""
       },
       {
        "name": "File",
        "type": "string",
        "tags": "hcl:\"file,optional\" json:\"file\""
       },
       {
        "name": "Line",
        "type": "int",
        "tags": "hcl:\"line,optional\" json:\"line\""
       },
       {
        "name": "Column",
        "type": "int",
        "tags": "hcl:\"column,optional\" json:\"column\""
       },
       {
        "name": "Checksum",
        "type": "types.Checksum",
        "tags": "hcl:\"checksum,optional\" json:\"checksum\"",
        "properties": [
         {
          "name": "Parsed",
          "type": "string",
          "tags": "hcl:\"parsed,optional\" json:\"parsed,omitempty\""
         },
         {
          "name": "Processed",
          "type": "string",
          "tags": "hcl:\"processed,optional\" json:\"processed,omitempty\""
         }
        ]
       },
       {
        "name": "Properties",
        "type": "map[string]interface {}",
        "tags": "json:\"properties,omitempty\""
       },
       {
        "name": "Links",
        "type": "[]string",
        "tags": "json:\"links,omitempty\""
       }
      ]
     }
    ]
   },
   {
    "name": "Name",
    "type": "string",
    "tags": "json:\"name\" hcl:\"name\""
   }
  ]
 }`
