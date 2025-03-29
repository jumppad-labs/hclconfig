package entities

type Address struct {
	Street  string `hcl:"street" json:"street"`
	City    string `hcl:"city" json:"city"`
	State   string `hcl:"state" json:"state"`
	Zip     string `hcl:"zip" json:"zip"`
	Country string `hcl:"country" json:"country"`
}

type Person struct {
	Name    string  `hcl:"name" json:"name"`
	Address Address `hcl:"address,block" json:"address"`
}
