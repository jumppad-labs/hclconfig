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
// Config defines the type `config`
type Config struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceMetadata `hcl:",remain"`

	ID string `hcl:"id"`

	DBConnectionString string `hcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `hcl:"timeouts,block"`
}

func (t *Config) Process() error {
	// override default values
	if t.Timeouts.TLSHandshake == 0 {
		t.Timeouts.TLSHandshake = 5
	}

	return nil
}

// PostgreSQL defines the Resource `postgres`
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceMetadata `hcl:",remain"`

	Location string `hcl:"location"`
	Port     int    `hcl:"port"`
	DBName   string `hcl:"name"`
	Username string `hcl:"username"`
	Password string `hcl:"password"`

	// ConnectionString is a computed field and must be marked optional
	ConnectionString string `hcl:"connection_string,optional"`
}

// Process is called using an order calculated from the dependency graph
// this is where you can set any computed fields
func (t *PostgreSQL) Process() error {
	t.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", t.Username, t.Password, t.Location, t.Port, t.DBName)
	return nil
}
```

You can then create a parser and register these resources with it:

```go
p := NewParser(DefaultOptions())
p.RegisterType("container", &structs.Container{})
p.RegisterType("network", &structs.Network{})
```

The following configuration reflects the previously defined structs. `config` refers to `postgres` through the link
`resource.postgres.mydb.connection_string`. The parser understands these links and will process `postgres` first allowing
you to set any calculated fields in the `Process` callback.  `config` also leverages a custom function `random_number`,
custom functions allow you to set values at parse time using go functions.

```javascript
variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "admin"
}

config "myapp" {
  // Custom functions can be created to enable functionality like generating random numbers
  id = "myapp_${random_number()}"

  // resource.postgres.mydb.connection_string will be available after the `Process` has
  // been called on the `postgres` resource. HCLConfig understands dependency and will
  // call Process in a strict order
  db_connection_string = resource.postgres.mydb.connection_string

  timeouts {
    connection = 10
    keep_alive = 60
    // optional parameter tls_handshake not specified
    // TLSHandshake = 10
  }
}

postgres "mydb" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = var.db_username
  password = var.db_password
}
```

To process the above config, first you need to register the custom `random_number` function.

```go
// register a custom function
p.RegisterFunction("random_number", func() (int, error) {
	return rand.Intn(100), nil
})
```

Then you can create the config and parse the file. 

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
r, err := c.FindResource("config.myapp")

// cast it back to the original type and access the paramters
c := r.(*Config)
fmt.Println("id", c.ID) // = myapp_81, where 81 is a random number between 0 and 100
fmt.Println("db_connection_string", c.db_connection_string) // = postgresql://admin:admin@localhost:5432/mydatabase
```

## TODO
[x] Basic parsing   
[x] Variables  
[x] Resource links and lazy evaluation   
[x] Enable custom interpolation functions   
[x] Modules   
