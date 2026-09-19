package main

import (
	"github.com/jumppad-labs/xcl/types"
)

// Timeouts is not a resource but a block and does not need `ResourceInfo` embedded or the `Resource`
// interface methods
type Timeouts struct {
	Connection   int `xcl:"connection,optional"`
	KeepAlive    int `xcl:"keep_alive,optional"`
	TLSHandshake int `xcl:"tls_handshake,optional"`
}

// Config defines the type `config`
type Config struct {
	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	types.ResourceBase `xcl:",remain"`

	FQN string `xcl:"fqn"`

	DBConnectionString string `xcl:"db_connection_string"`

	// references a complete resource
	MainDBConnection PostgreSQL `xcl:"main_db_connection"`

	// references a list of resources
	OtherDBConnections []PostgreSQL `xcl:"other_db_connections"`

	// Fields that are of `struct` type must be marked using the `block`
	// parameter in the tags. To make a `block` Field, types marked as block must be
	// a reference i.e. *Timeouts
	Timeouts *Timeouts `xcl:"timeouts,block"`
}

type DBCommon struct {
	types.ResourceBase `xcl:",remain"`
	ErikIsA            string `xcl:"erik_is_a,optional"`
}

// PostgreSQL defines the Resource `postgres`
type PostgreSQL struct {

	// For a resource to be parsed by HCLConfig it needs to embed the ResourceInfo type and
	// add the methods from the `Resource` interface
	DBCommon `xcl:",remain"`

	ID       string `xcl:"id,optional"`
	Location string `xcl:"location"`
	Port     int    `xcl:"port"`
	DBName   string `xcl:"db_name"`
	Username string `xcl:"username"`
	Password string `xcl:"password"`

	// ConnectionString is a computed field and must be marked optional
	ConnectionString string `xcl:"connection_string,optional"`
}
