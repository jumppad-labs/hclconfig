# HCL Configuration Parser

This package allows you to process configuration files written using the HashiCorp Configuration Language (HCL).
It has full resource linking where a parameter in one configuration stanza can reference a parameter in another stanza.
Variable support, and Modules allowing configuration to be loaded from local or remote sources.

HCLConfig also has a full AcyclicGraph that allows you to process configuration with strict dependencies. This ensures
that a parameter from one configuration has been set before the value is interpolated in a dependent resource.

The project aims to provide a simple API allowing you to define resources as Go structs without needing to understand
the HashiCorp HCL2 library. 

## Example

Resources to be parsed are defined as Go structs that implement the Resource interface and annoted with the `hcl` tag

```go
const TypeContainer types.ResourceType = "container"

type Container struct {
	// embedded type holding name, etc
	types.ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Networks []NetworkAttachment `hcl:"network,block" json:"networks,omitempty"` // Attach to the correct network // only when Image is specified
	
  CPU string `hcl:"cpu" json:"cpu"`
}

// New creates a new Nomad job config resource, implements Resource New method
func (c *Container) New(name string) types.Resource {
	return &Container{ResourceInfo: types.ResourceInfo{Name: name, Type: TypeContainer, Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (c *Container) Info() *types.ResourceInfo {
	return &c.ResourceInfo
}

// Called when the resource is parsed from a file
func (c Container) Parse(file string) error {
  return nil
}

// Called when the DAG processes the resource
func (n *Network) Process(file string) error {
  return nil
}

// TypeNetwork is the string resource type for Network resources
const TypeNetwork types.ResourceType = "network"

// Network defines a Docker network
type Network struct {
	types.ResourceInfo `hcl:",remain" mapstructure:",squash"`

	Subnet string `hcl:"subnet" json:"subnet"`
}

// New creates a new Nomad job config resource, implements Resource New method
func (n *Network) New(name string) types.Resource {
	return &Network{ResourceInfo: types.ResourceInfo{Name: name, Type: TypeNetwork, Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (n *Network) Info() *types.ResourceInfo {
	return &n.ResourceInfo
}

// Called when the resources is parsed
func (n *Network) Parse(file string) error {
  return nil
}

// Called when the DAG processes the resource
func (n *Network) Process(file string) error {
  return nil
}
```

You can then create a parser and register these resources with it:

```go
p := NewParser(DefaultOptions())
p.RegisterType("container", &structs.Container{})
p.RegisterType("network", &structs.Network{})
```

To process the following HCL configuration:

```javascript
variable "cpu_resources" {
  default = 2048
}

network "onprem" {
  subnet = "10.6.0.0/16"
}

container "base" {
  network {
    name       = resources.network.onprem.name
    ip_address = "10.6.0.200"
  }

  cpu = var.cpu_resources
}
```

You only need to call the following methods:

```go
// create a config, all processed resources are encapuslated by config
c := NewConfig()

// parse a single hcl file
// config passed to this function is not mutated but a copy with the new resources parsed is returned
c, err := p.ParseFile("myfile.hcl", c)

// walk through all resources in the dag processing the links
// when a resource is processed by the DAG, Process is called
// at this point all linked attributes will have been resolved
//
// optionally you can provide a function that will be called for every resource
// this callback is executed after the resources Process method

c.Walk(func(r types.Resource) error {
  fmt.Println("processed:", r.Name)

  return nil
})

// find a resource based on it's type and name
r, err := c.FindResource("container.consul")

// cast it back to the original type and access the paramters
cont := r.(*Container)
fmt.Println("cpu", cont.CPU) // 2048
fmt.Println("network name", cont.Networks[0].Name) // onprem
```

## TODO
[x] Basic parsing 
[x] Variables 
[x] Resource links 
[ ] Modules 
