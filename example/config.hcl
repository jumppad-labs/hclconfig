variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "password"
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
  port     = 5432
  name     = "mydatabase"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
}

// modules can use a git ref to be remotely downloaded from the source
module "mymodule_1" {
  source = "github.com/shipyard-run/hclconfig?ref=dbea14620e32b4e685cd6f7edf49648b45d21a70/example/modules//db"

  variables = {
    db_username = var.db_username
    db_password = "topsecret"
  }
}

// modules can also use 
module "mymodule_2" {
  source = "../example/modules/db"

  variables = {
    db_username = "root"
    db_password = "password"
  }
}

// outputs allow you to specify values that can be consumed from other 
// modules
output "module1_connection_string" {
  value = module.mymodule_1.output.connection_string
}

output "module2_connection_string" {
  value = module.mymodule_2.output.connection_string
}