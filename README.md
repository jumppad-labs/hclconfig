# HCL Configuration Parser

[![Go Reference](https://pkg.go.dev/badge/github.com/shipyard-run/hclconfig.svg)](https://pkg.go.dev/github.com/shipyard-run/hclconfig)

This package allows you to process configuration files written using the HashiCorp Configuration Language (HCL).
It has full resource linking where a parameter in one configuration stanza can reference a parameter in another stanza.
Variable support, and Modules allowing configuration to be loaded from local or remote sources.

The project aims to provide a simple API allowing you to define resources as Go structs without needing to fully understand
the HashiCorp HCL2 library. 

HCLConfig has a full AcyclicGraph that allows you to process configuration with strict dependencies. This ensures
that a parameter from one configuration has been set before the value is interpolated in a dependent resource.

Parsing is a two step approach, first the parser reads the HCL configuration from the supplied files, at this stage a 
graph is computed based on any references inside the configuration. For example given the following two resources.

```javascript
resource "postgres" "mydb_2" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  username = "db2"
  password = resource.postgres.mydb_1.password
}

resource "postgres" "mydb_1" {
  location = "localhost"
  port = 5432
  name = "mydatabase"
  
  username = "db1"
  password = random_password()
}
```

#### Step 1:
When the first pass of the parser runs it will read `mydb_2` before `mydb_1`, marshaling each resource into a struct and
calling the optional `Parse` method on that struct. At this point none of the interpolated properties like 
`resource.postgres.mydb_1.password` have a value as it is assumed that the referenced resources does not yet exist. At
this point the parser replaces the interpolated value with a default value for the field.  

#### Step 2:
After resources have been processed from the HCL configuration a graph of dependent resources
is calculated. Given the previous example where resource `mydb_2` references a property from 
`mydb_1`, the resultant graph would look like the following.

```
| -- resource.postgres.mydb_2
     |  -- resource.postgres.mydb_1
```

This graph is then walked, as each resource is processed, any referenced properties are resolved and assigned
to the struct. For example, when `resource.postgres.mydb_2` is processed the `password` field that contains
a reference to `resource.postgres.mydb_1` will be assigned the actual value from the linked resource.

The optional `Process` method on the struct is also called, where a resource may contain computed fields the
user can implement these computations in `Process` as this will make their value available to the next
node in graph.


## Example

Resources to be parsed are defined as Go structs that implement the Resource interface and annotated with the `hcl` tag

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

resource "config" "myapp" {
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

resource "postgres" "mydb" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  // Variables can be used to set values, the default values for these variables will be overridden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
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
c, err := p.ParseFile("myfile.hcl")
```

You can then access the properties from your types by retrieving them from the returned config.


```go
// find a resource based on it's type and name
r, err := c.FindResource("resource.config.myapp")

// cast it back to the original type and access the paramters
c := r.(*Config)
fmt.Println("id", c.ID) // = myapp_81, where 81 is a random number between 0 and 100
fmt.Println("db_connection_string", c.db_connection_string) // = postgresql://admin:admin@localhost:5432/mydatabase
```

## Struct Tags

To create types that can be converted from HCL your top level resource needs to embed the
following type into your structs.

``types.ResourceMetadata `hcl:",remain"` ``

The struct tag `` `hcl:",remain"` ``, must be included with this type as it tells the HCL parser
to unfold the default properties such as `disabled` and `depends_on` from your custom type.

### Basic Attributes

If you add the field `` Location string `hcl:"location"` `` to your type this will mean that 
the hcl attribute `location` will be parsed into this Field. This creates a required
attribute for HCL, not providing the `location` attribute on the hcl representing 
the `PostgresSQL` struct will result in a parser error.

```go
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceMetadata `hcl:",remain"`

	Location string `hcl:"location"`
}
```

### Optional Attributes
To create optional attribute you can add the `optional` keyword to the struct tag
the previous example has been modified to make `location` optional.

```go
type PostgreSQL struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceMetadata `hcl:",remain"`

	Location string `hcl:"location,optional"`
}
```

### Mandatory Blocks

To define child blocks in your configuration you can specify a field that contains
another struct. In the following example the `Timeouts` field specifies that
the `Config` must be specified with a mandatory child stanza `timeouts`.

To configure a block the `block` struct tag is used after the hcl attribute
name.

```
`hcl:"timeouts,block"`
```

This can be seen in the following code sample.

```go
type Config struct {
	types.ResourceMetadata `hcl:",remain"`

	DBConnectionString string `hcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts Timeouts `hcl:"timeouts,block"`
}
```

This would be configured using the following HCL.

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
  timeouts {
    tls_handshake = 10
  }
}
```

The `Timeout` type used by the field `Timeout` does not need to embed `ResourceMetadata`
as it is not a top level resource but all other struct tags that define blocks and
optional parameters are required.

### Optional Blocks

To make child blocks optional you simply need to change the Field type to a reference

```go
type Config struct {
	types.ResourceMetadata `hcl:",remain"`

	DBConnectionString string `hcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `hcl:"timeouts,block"`
}
```

`timeouts` is now optional and will not result in a parser error if not 
specified.

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
}
```

### Multiple Blocks

To allow a block to be used 0 or more times you can define the Field as a
slice.

```go
type Config struct {
	types.ResourceMetadata `hcl:",remain"`

	DBConnectionString string `hcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts []Timeouts `hcl:"timeouts,block"`
}
```

`timeouts` can now be specified multiple times

```javascript
resource "config" "myconfig" {
  db_connection_string = "abc"
  
  timeouts {
    tls_handshake = 10
  }
  
  timeouts {
    tls_handshake = 10
  }
}
```

Note: when parsing the configuration the order of the `Timeouts` field will correspond 
to the order of the `timeouts` blocks as defined in the `config`.


## Modules

HCLConfig supports modular configuration that enables you to group your configuration or encapsulate certain
functionality into modules.

A module is a default type, however you still need to create the go structs that 
define the resources included in your module. The following example shows how you can
use the module that is defined in [./example/modules/db/db.hcl](./example/modules/db/db.hcl)

Any sub folder can be a module, to create a module all that is needed is one or more `.hcl` files
that contain your custom resources.

```javascript
// modules can also use 
module "mymodule_1" {
  source = "../example/modules/db"

  variables = {
    db_username = "root"
    db_password = "password"
  }
}
```

Modules can also be imported from remote sources such as a GitHub repository, to version
a module the SHA of the commit can be used.

```javascript
module "mymodule_1" {
  source = "github.com/shipyard-run/hclconfig?ref=9173050/example/modules//db"

  variables = {
    db_username = variable.db_username
    db_password = "topsecret"
  }
}
```

### Inputs

To enable dynamic module use, `variables` and `outputs` can be used to define
the interface for your module. Variables can be define inside the module and
the value set explicitly using the `variables` block as shown in the previous
example.

### Outputs

To return a value from a module you can define an `output`, the `db` module
defines the output `connection_string`. 

```javascript
output "connection_string" {
  value = resource.postgres.mydb.connection_string
}
```

To read this value you can use the interpolation syntax `module.mymodule_1.output.name`
The following example shows how an output from one module can be used as an
input to another module. Because HCLConfig understands the links between resources
the resources in `my_other_module` will only be processed after the resources
in `mymodule_1`.

```javascript
module "mymodule_1" {
  source = "../example/modules/db"

  variables = {
    db_username = "root"
    db_password = "password"
  }
}

module "my_other_module" {
  source = "../example/modules/app"

  variables = {
    db_connection_string = module.mymodule_1.output.connection_string
  }
}
```

## Functions

HCLConfig supports functions that can be used inside your configuration


```javascript
postgres "mydb" {
  location = "localhost"
  port = 5432
  name = "mydatabase"

  username = var.db_username

  // functions can be used inside the configuration,
  // functions are evaluated when the configuration is parsed 
  password = env("DB_PASSWORD")
}
```
### Default functions

For convenience HCLConfig has the following default functions:

#### len(type)

Returns the length of a string or collection

```javascript
mytype "test" {
  collection = ["one", "two"]
  string = "mystring"
}

myothertype "test" {
  // Value = 2
  collection_length = len(resource.mytype.test.collection)

  // Value = 8
  string_length = len(resource.mytype.test.string)
}
```

#### env(name)

Returns the value of a system environment variable

```javascript
mytype "test" {
  // returns the value of the system environment variable $GOPATH
  gopath = env("GOPATH")
}
```

#### home()

Returns the location of the users home directory

```javascript
mytype "test" {
  // returns the value of the system home directory
  home_folder = home()
}
```

#### file(path)

Returns the contents of a file at the given path.

```javascript

# given the file "./myfile.txt" with the contents "foo bar"

mytype "test" {
  // my_file = "foobar"
  my_file = file("./myfile.txt")
}
```

#### template_file(path, variables)

Returns the rendered contents of a template file at the given path with the given input variables.

# given the file "./mytemplate.tmpl" with the contents "hello {{name}}"

mytype "test" {
  // my_file = "foobar"
  my_file = template_file("./mytemplate.tmpl", {
    name = "world"
  })
}

#### dir()

Returns the absolute path of the directory containing the current resource

```javascript
mytype "test" {
  resource_folder = dir()
}
```

#### trim(string)

Returns the given string with leading and trailing whitespace removed
of the given string

```javascript
mytype "test" {
  // trimmed = "abc 123"
  trimmed = trim("  abc  123   ")
}
```

### Custom Functions

In addition to the default functions it is possible to register custom functions.

For example, given a requirement to have a function that returns a random number in a set
range you could write a go function that looks like the following. Note: only a single
return type can be consumed by the HCL parser and assigned to the resource value.

```go
func RandRange(min, max int) int {
	return rand.Intn((max-min)+1) + min
}
```

This could then be referenced in the following config

```javascript
postgres "mydb" {
  location = "localhost"

  // custom function to return a random number between 5000 and 6000
  port = rand(5000,6000)
  
  name = "mydatabase"
}
```

You set up the parser as normal

```go
p := NewParser(DefaultOptions())
p.RegisterType("postgres", &structs.Postgres{})
```

However, in order to use the custom function before parsing you register it with the 
`RegisterFunction` method as shown below.

```go
p.RegisterFunction("rand", RandRange)
```

At present only the following simple types are supported for custom functions

* string
* uint
* uint32
* uint64
* int
* int32
* int64
* float32
* float64

#### Errors in custom functions
To signify that an error occurred in a custom function and to halt parsing of the
config your function can optionally return a tuple of (type, error). For example
to add error handling to the random function you could write it as shown below.

```go
func RandRange(min, max int) (int, error) {
  if min >= max {
    return -1, fmt.Errorf("minimum value '%d' must be smaller than the maximum value '%d')
  }

	return rand.Intn((max-min)+1) + min
}
```

## Lifecycle Callbacks

HCLConfig provides three hooks that can be used when parsing configuration.

* Resource `Processable` interface
* Parser Callback
* Config Process Callback

### Resource Processable interface

The resource `Processable` interface can be added to your resources by adding
a an optional method with the following singature.

```go
Process() error
```

For example, the `PostgresSQL` resource implement the `Processable` interface
to compute the value of the attribute `connection_string`.

```go
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

`Process` is called in strict order depending on the dependencies for your resources.

For example, given the following custom resources

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

And the following configuration that uses these resources

```javascript
resource "config" "myconfig" {
  // resource.postgres.mydb.connection_string will be available after the `Process` has
  // been called on the `postgres` resource. HCLConfig understands dependency and will
  // call Process in a strict order
  db_connection_string = resource.postgres.mydb.connection_string
}

resource "postgres" "mydb" {
  location = "localhost"
  port     = 5432
  name     = "mydatabase"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
}
```

Because you are referencing the attribute `resource.postgres.mydb.connection_string`
to set a value in the `config` resource. `Process` for the `PostgreSQL` type will be called
before `Process` for the `Config` type. This allows you to perform any computations or validations
needed to calculate `connection_string` before `config` attempts to consume the value.

Returning an `error` from `Process` will immediately exit the `ParseFile` or `ParseDirectory`
method.

### Parser Callback

Rather than implementing individual resource functions you may prefer to leverage the global
callback that can be set on the `ParserOptions`.

```go
o := hclconfig.DefaultOptions()

// set the callback that will be executed when a resource has been created
// this function can be used to execute any external work required for the
// resource.
o.ParseCallback = func(r types.Resource) error {
	fmt.Printf(
    "resource '%s' named '%s' has been parsed from the file: %s\n", 
    r.Metadata().Type, 
    r.Metadata().Name, 
    r.Metadata().File,
  )

  // cast the Resource into a concrete type
  switch r.Metadata().Type {
    case "config":
      myconfig := r.(*Config)
      fmt.Println(myconfig.DBConnectionString)
  }

	return nil
}
```

The `ParseCallback` function is executed `after` the `Processable` interface
and respects the same call order that is implemented for `Processable`.

### Config `Process` function

A final callback is available using the `Process(wf ProcessCallback, reverse bool) error`
function that is available on the `hclconfig.Config` type.

`Process` builds a Directed Acyclic Graph for your configuration based on
the dependency and calls the provided `ProcessCallback` for each resource 
in the graph.

```go
nc, _ := p.ParseFile("./config.hcl")

nc.Process(func(r types.Resource) error {
	fmt.Println("  ", r.Metadata().ID)
	return nil
}, false)
```

**Note**  
While you can mutate the values of the `Resource` passed to the
ProcessCallback, it will not update any resources that reference this attribute.

When ParseFile resolves interpolated values it `copies` the value to the destination
resource. Given the earlier example mutating the `ConnectionString` field on the 
`postgres` resource would not update the `config` resource even though the  `ProcessCallback`
will be called with the `PostgreSQL` type before `Config`.

### Walking dependencies in reverse

To reverse the order of resources that are provided to the `ProcessCallback` you can
set the second process method attribute to `true`.

```go
nc, _ := p.ParseFile("./config.hcl")

nc.Process(func(r types.Resource) error {
	fmt.Println("  ", r.Metadata().ID)
	return nil
}, true)
```

`ProcessCallback` will be called first for resources lowest down in the dependency
graph `children` before the resources they depend on.

An ideal use for this method is to clean up any operations that may have been created
with the `Processable` interface on your resource or the `ParseCallback`.

## Serialization

To save state the `hclconfig.Config` type can be serialized to JSON using the following
method.

```go
d, err := c.ToJSON()
ioutil.WriteFile("./config.json", d, os.ModePerm)
```
## Deserialization

To deserialize `hclconfig.Config` that has been serialized with the `ToJSON` method
you can use the `UnmarshalJSON` method on the `Parser`.

`UnmarshalJSON` will reconstruct the concrete types based on the configured resources.

```go
d, _ := ioutil.ReadFile("./config.json")
nc, err := p.UnmarshalJSON(d)
if err != nil {
	fmt.Printf("An error occurred unmarshalling the config: %s\n", err)
	os.Exit(1)
}
```
