package main

import (
	"fmt"

	"github.com/shipyard-run/hclconfig/types"
)

// Timeouts is not a resource but a block and does not need `ResourceInfo` embedded or the `Resource`
// interface methods
type Timeouts struct {
	Connection   int `hcl:"connection,optional"`
	KeepAlive    int `hcl:"keep_alive,optional"`
	TLSHandshake int `hcl:"tls_handshake,optional"`
}

// Config defines the type `config`
type Config struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceInfo `hcl:",remain"`

	ID string `hcl:"id"`

	DBConnectionString string `hcl:"db_connection_string"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `hcl:"timeouts,block"`
}

// New creates a new Config config resource, implements Resource New method
func (t *Config) New(name string) types.Resource {
	return &Config{ResourceInfo: types.ResourceInfo{Name: name, Type: types.ResourceType("config"), Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *Config) Info() *types.ResourceInfo {
	return &t.ResourceInfo
}

func (t *Config) Parse(file string) error {
	return nil
}

func (t *Config) Process() error {
	fmt.Println("timeout", t.Timeouts.TLSHandshake)
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
	types.ResourceInfo `hcl:",remain"`

	Location string `hcl:"location"`
	Port     int    `hcl:"port"`
	DBName   string `hcl:"name"`
	Username string `hcl:"username"`
	Password string `hcl:"password"`

	// ConnectionString is a computed field and must be marked optional
	ConnectionString string `hcl:"connection_string,optional"`
}

// New creates a new Config config resource, implements Resource New method
func (t *PostgreSQL) New(name string) types.Resource {
	return &PostgreSQL{ResourceInfo: types.ResourceInfo{Name: name, Type: types.ResourceType("postgres"), Status: types.PendingCreation}}
}

// Info returns the resource info implements the Resource Info method
func (t *PostgreSQL) Info() *types.ResourceInfo {
	return &t.ResourceInfo
}

func (t *PostgreSQL) Parse(file string) error {
	return nil
}

// Process is called using an order calculated from the dependency graph
// this is where you can set any computed fields
func (t *PostgreSQL) Process() error {
	fmt.Println("postgres")
	t.ConnectionString = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", t.Username, t.Password, t.Location, t.Port, t.DBName)
	return nil
}
