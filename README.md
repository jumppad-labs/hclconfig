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
c := hclconfig.NewConfig()

// define the options for the parser
opts := hclconfig.DefaultOptions()

// Callback is executed when the parser processes a resource
opts.Callback = func(r *types.Resource) error {
  fmt.Println("Parser has processed", r.Info().Name)
}

// parse a single hcl file.
// config passed to this function is not mutated but a copy with the new resources parsed is returned
//
// when configuration is parsed it's dependencies on other resources are evaluated and this order added
// to a acyclic graph ensuring that any resources are processed before resources that depend on them.
c, err := p.ParseFile("myfile.hcl", c)
```

You can then access the properties from your types by retrieving them from the returned config.


```go
// find a resource based on it's type and name
r, err := c.FindResource("container.base")

// cast it back to the original type and access the paramters
cont := r.(*Container)
fmt.Println("cpu", cont.CPU) // 2048
fmt.Println("network name", cont.Networks[0].Name) // onprem
```

## TODO
[x] Basic parsing   
[x] Variables  
[x] Resource links and lazy evaluation   
[ ] Enable custom interpolation functions   
[ ] Modules   
