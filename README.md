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
r, err := c.FindResource("config.myapp")

// cast it back to the original type and access the paramters
c := r.(*Config)
fmt.Println("id", c.ID) // = myapp_81, where 81 is a random number between 0 and 100
fmt.Println("db_connection_string", c.db_connection_string) // = postgresql://admin:admin@localhost:5432/mydatabase
```

## Default functions

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

HCLConfig has the following default functions

### len(type)

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

### env(name)

Returns the value of a system environment variable

```javascript
mytype "test" {
  // returns the value of the system environment variable $GOPATH
  gopath = env("GOPATH")
}
```

### home()

Returns the location of the users home directory

```javascript
mytype "test" {
  // returns the value of the system home directory
  home_folder = home()
}
```

### file(path)

Returns the contents of a file at the given path.

```javascript

# given the file "./myfile.txt" with the contents "foo bar"

mytype "test" {
  // my_file = "foobar"
  my_file = file("./myfile.txt")
}
```

### dir()

Returns the absolute path of the directory containing the current resource

```javascript
mytype "test" {
  resource_folder = dir()
}
```

### trim(string)

Returns the given string with leading and trailing whitespace removed
of the given string

```javascript
mytype "test" {
  // trimmed = "abc 123"
  trimmed = trim("  abc  123   ")
}
```

## Custom Functions

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

### Errors in custom functions
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