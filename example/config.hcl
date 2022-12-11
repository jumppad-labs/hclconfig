variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "password"
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

module "mymodule" {
  #source = "github.com/shipyard-run/hclconfig//example"
  source = "./modules/db"

  variables = {
    db_username = "root"
    db_password = "topsecret"
  }
}
