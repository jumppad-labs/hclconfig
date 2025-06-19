package example

import (
	"fmt"

	"github.com/jumppad-labs/hclconfig"
	"github.com/jumppad-labs/hclconfig/plugins/example/entities"
	"github.com/jumppad-labs/hclconfig/types"
)

func config() {
	c := hclconfig.NewConfig()
	p := entities.Person{
		ResourceBase: types.ResourceBase{Meta: types.Meta{ID: "blah"}},
		Name:         "blah",
	}

	c.AppendResource(&p)

	// find old method
	r, err := c.FindResource("blah")
	if err != nil {
		panic(err)
	}

	fmt.Println(r.(*entities.Person).Name) // Output: "blah"

	rs, err := c.FindResourcesByType("person")
	if err != nil {
		panic(err)
	}

	for _, r := range rs {
		fmt.Println(r.(*entities.Person).Name) // Output: "blah"
	}

	// find new method
	q := hclconfig.NewQuerier[*entities.Person](c)
	r2, err := q.FindResource("blah")
	if err != nil {
		panic(err)
	}
	fmt.Println(r2.Name) // Output: "blah"

	rs2, err := q.FindResourcesByType()
	if err != nil {
		panic(err)
	}

	for _, r := range rs2 {
		fmt.Println(r.Name) // Output: "blah"
	}
}
