package fixtures

import "github.com/jumppad-labs/xcl/types"

type Embedded struct {
	types.ResourceBase `xcl:",remain"`
	Name               string `json:"name" xcl:"name"`
}

var EmbeddedJson = `{
  "type": "fixtures.Embedded",
  "properties": [
   {
    "type": "types.ResourceBase",
    "tags": "xcl:\",remain\"",  
    "anonymous": true,
    "properties": [
     {
      "name": "DependsOn",
      "type": "[]string",
      "tags": "xcl:\"depends_on,optional\" json:\"depends_on,omitempty\""
     },
     {
      "name": "Disabled",
      "type": "bool",
      "tags": "xcl:\"disabled,optional\" json:\"disabled,omitempty\""
     },
     {
      "name": "Meta",
      "type": "types.Meta",
      "tags": "xcl:\"meta,optional\" json:\"meta,omitempty\"",
      "properties": [
       {
        "name": "ID",
        "type": "string",
        "tags": "xcl:\"id,optional\" json:\"id\""
       },
       {
        "name": "Name",
        "type": "string",
        "tags": "xcl:\"name,optional\" json:\"name\""
       },
       {
        "name": "Type",
        "type": "string",
        "tags": "xcl:\"type,optional\" json:\"type\""
       },
       {
        "name": "Module",
        "type": "string",
        "tags": "xcl:\"module,optional\" json:\"module,omitempty\""
       },
       {
        "name": "File",
        "type": "string",
        "tags": "xcl:\"file,optional\" json:\"file\""
       },
       {
        "name": "Line",
        "type": "int",
        "tags": "xcl:\"line,optional\" json:\"line\""
       },
       {
        "name": "Column",
        "type": "int",
        "tags": "xcl:\"column,optional\" json:\"column\""
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
        "name": "Parents",
        "type": "[]string",
        "tags": "json:\"parents,omitempty\""
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
    "tags": "json:\"name\" xcl:\"name\""
   }
  ]
 }`

type EmbeddedInEmbedded struct {
	Embedded  `xcl:",remain"`
	ChildName string `json:"child_name" xcl:"child_name"`
}

var EmbeddedInEmbeddedJson = `{
  "type": "fixtures.EmbeddedInEmbedded",
  "properties": [
   {
    "type": "fixtures.Embedded",
    "tags": "xcl:\",remain\"",
    "anonymous": true,
    "properties": [
     {
      "type": "types.ResourceBase",
      "tags": "xcl:\",remain\"",
      "anonymous": true,
      "properties": [
       {
        "name": "DependsOn",
        "type": "[]string",
        "tags": "xcl:\"depends_on,optional\" json:\"depends_on,omitempty\""
       },
       {
        "name": "Disabled",
        "type": "bool",
        "tags": "xcl:\"disabled,optional\" json:\"disabled,omitempty\""
       },
       {
        "name": "Meta",
        "type": "types.Meta",
        "tags": "xcl:\"meta,optional\" json:\"meta,omitempty\"",
        "properties": [
         {
          "name": "ID",
          "type": "string",
          "tags": "xcl:\"id,optional\" json:\"id\""
         },
         {
          "name": "Name",
          "type": "string",
          "tags": "xcl:\"name,optional\" json:\"name\""
         },
         {
          "name": "Type",
          "type": "string",
          "tags": "xcl:\"type,optional\" json:\"type\""
         },
         {
          "name": "Module",
          "type": "string",
          "tags": "xcl:\"module,optional\" json:\"module,omitempty\""
         },
         {
          "name": "File",
          "type": "string",
          "tags": "xcl:\"file,optional\" json:\"file\""
         },
         {
          "name": "Line",
          "type": "int",
          "tags": "xcl:\"line,optional\" json:\"line\""
         },
         {
          "name": "Column",
          "type": "int",
          "tags": "xcl:\"column,optional\" json:\"column\""
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
          "name": "Parents",
          "type": "[]string",
          "tags": "json:\"parents,omitempty\""
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
      "tags": "json:\"name\" xcl:\"name\""
     }
    ]
   },
   {
    "name": "ChildName",
    "type": "string",
    "tags": "json:\"child_name\" xcl:\"child_name\""
   }
  ]
 }`
