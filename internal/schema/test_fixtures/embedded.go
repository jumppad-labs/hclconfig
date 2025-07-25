package fixtures

import "github.com/jumppad-labs/hclconfig/types"

type Embedded struct {
	types.ResourceBase `hcl:",remain"`
	Name               string `json:"name" hcl:"name"`
}

var EmbeddedJson = `{
  "type": "fixtures.Embedded",
  "properties": [
   {
    "type": "types.ResourceBase",
    "tags": "hcl:\",remain\"",  
    "anonymous": true,
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
        "name": "Properties",
        "type": "map[string]interface {}",
        "tags": "json:\"properties,omitempty\""
       },
       {
        "name": "Links",
        "type": "[]string",
        "tags": "json:\"links,omitempty\""
       },
       {
        "name": "Status",
        "type": "string",
        "tags": "json:\"status,omitempty\""
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

type EmbeddedInEmbedded struct {
	Embedded  `hcl:",remain"`
	ChildName string `json:"child_name" hcl:"child_name"`
}

var EmbeddedInEmbeddedJson = `{
  "type": "fixtures.EmbeddedInEmbedded",
  "properties": [
   {
    "type": "fixtures.Embedded",
    "tags": "hcl:\",remain\"",
    "anonymous": true,
    "properties": [
     {
      "type": "types.ResourceBase",
      "tags": "hcl:\",remain\"",
      "anonymous": true,
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
          "name": "Properties",
          "type": "map[string]interface {}",
          "tags": "json:\"properties,omitempty\""
         },
         {
          "name": "Links",
          "type": "[]string",
          "tags": "json:\"links,omitempty\""
         },
         {
          "name": "Status",
          "type": "string",
          "tags": "json:\"status,omitempty\""
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
   },
   {
    "name": "ChildName",
    "type": "string",
    "tags": "json:\"child_name\" hcl:\"child_name\""
   }
  ]
 }`
