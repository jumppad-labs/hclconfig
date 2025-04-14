package entities

import "github.com/jumppad-labs/hclconfig/types"

type Address struct {
	Street  string `hcl:"street" json:"street"`
	City    string `hcl:"city" json:"city"`
	State   string `hcl:"state" json:"state"`
	Zip     string `hcl:"zip" json:"zip"`
	Country string `hcl:"country" json:"country"`
}

type Person struct {
	types.ResourceBase

	Name    string  `hcl:"name" json:"name"`
	Address Address `hcl:"address,block" json:"address"`
}
