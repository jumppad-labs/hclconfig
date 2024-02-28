package main

import (
	"fmt"

	"github.com/jumppad-labs/hclconfig/types"
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
	types.ResourceBase `hcl:",remain"`

	FQN string `hcl:"fqn"`

	DBConnectionString string `hcl:"db_connection_string"`

	// references a complete resource
	MainDBConnection PostgreSQL `hcl:"main_db_connection"`

	// references a list of resources
	OtherDBConnections []PostgreSQL `hcl:"other_db_connections"`

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

type DBCommon struct {
	types.ResourceBase `hcl:",remain"`
	ErikIsA            string `hcl:"erik_is_a,optional"`
}

// PostgreSQL defines the Resource `postgres`
type PostgreSQL struct {

	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	DBCommon `hcl:",remain"`

	ID       string `hcl:"id,optional"`
	Location string `hcl:"location"`
	Port     int    `hcl:"port"`
	DBName   string `hcl:"db_name"`
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
