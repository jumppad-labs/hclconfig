// Package resources holds the Go types for the blocks in the configuration
// this example applies, ../config. Two plugins provide them, each providing
// two of them:
//
//   - postgres and redis come from the in-process plugin, ../internal
//   - app and ingress come from the external plugin, ../external
//
// Each type embeds types.ResourceBase, which gives it the common block
// metadata. The xcl tags map configuration to fields, and the json tags name
// the fields the same way when the resources are saved to state or passed to
// a plugin.
package resources

import "github.com/jumppad-labs/xcl/types"

// PostgreSQL defines the block type `postgres`
type PostgreSQL struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`
	DBName   string `xcl:"db_name" json:"db_name"`
	Username string `xcl:"username" json:"username"`
	Password string `xcl:"password" json:"password"`

	// Timeouts is a nested block, block fields must be pointers
	Timeouts *Timeouts `xcl:"timeouts,block" json:"timeouts,omitempty"`

	// ConnectionString is computed, it can not be set in configuration. A
	// provider fills it in when the database is created, so it is only set in
	// the plugin example, in the configuration only example it stays empty.
	ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}

// Timeouts is the nested `timeouts` block of a postgres block, it is not a
// resource so it does not embed types.ResourceBase
type Timeouts struct {
	Connection int `xcl:"connection,optional" json:"connection,omitempty"`
	KeepAlive  int `xcl:"keep_alive,optional" json:"keep_alive,omitempty"`
}

// Redis defines the block type `redis`, the second type the in-process plugin
// provides
type Redis struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`

	// ConnectionString is computed the same way as the one on PostgreSQL, the
	// redis provider fills it in when the cache is created
	ConnectionString string `xcl:"connection_string,optional,computed" json:"connection_string,omitempty"`
}

// App defines the block type `app`, its values are read from a variable, a
// postgres block, a redis block and a module output
type App struct {
	types.ResourceBase `xcl:",remain"`

	DatabaseLocation  string `xcl:"database_location" json:"database_location"`
	DatabaseUser      string `xcl:"database_user" json:"database_user"`
	ConnectionString  string `xcl:"connection_string,optional" json:"connection_string,omitempty"`
	AnalyticsLocation string `xcl:"analytics_location" json:"analytics_location"`

	// CacheConnectionString is read from the redis block, the value crosses
	// from the in-process plugin to the external one
	CacheConnectionString string `xcl:"cache_connection_string,optional" json:"cache_connection_string,omitempty"`

	// URL is computed, the app provider fills it in when the app is created.
	// The ingress block reads it, so a computed value also crosses between two
	// types provided by the same plugin.
	URL string `xcl:"url,optional,computed" json:"url,omitempty"`
}

// Ingress defines the block type `ingress`, the second type the external
// plugin provides. It routes a hostname to an app.
type Ingress struct {
	types.ResourceBase `xcl:",remain"`

	Hostname string `xcl:"hostname" json:"hostname"`

	// AppURL is read from the computed url of an app block, it is empty in the
	// configuration only example, where nothing computes it
	AppURL string `xcl:"app_url,optional" json:"app_url,omitempty"`
}
