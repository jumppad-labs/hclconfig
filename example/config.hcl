variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "password"
}


resource "postgres" "mydb" {
  location = "localhost"
  port     = 5432
  db_name  = "mydatabase"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
}

resource "postgres" "other1" {
  location = "1.other.location"
  port     = 5432
  db_name  = "other1"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password
}

resource "postgres" "other2" {
  depends_on = ["resource.postgres.other1"]
  disabled   = false

  id       = "lekker.broodje"
  location = "2.other.location"
  port     = 5432
  db_name  = "other2"

  // Varaibles can be used to set values, the default values for these variables will be overidden
  // by values set by the environment variables HCL_db_username and HCL_db_password
  username = variable.db_username
  password = variable.db_password

  erik_is_a = "a Dutchman"
}

resource "config" "myapp" {
  // Custom functions can be created to enable functionality like generating random numbers
  fqn = "myapp_${random_number()}"

  // resource.postgres.mydb.connection_string will be available after the `Process` has
  // been called on the `postgres` resource. HCLConfig understands dependency and will
  // call Process in a strict order
  db_connection_string = resource.postgres.mydb.connection_string

  // reference an entire other resource
  main_db_connection = resource.postgres.mydb

  // collection of entire other resources
  other_db_connections = [
    resource.postgres.other1,
    resource.postgres.other2
  ]

  timeouts {
    connection = 10
    keep_alive = 60
    // optional parameter tls_handshake not specified
    // TLSHandshake = 10
  }
}

resource "person" "erik" {
  first_name = "Erik"
  last_name  = "Veld"
}

output "erik" {
  value = resource.postgres.other2.erik_is_a
}

output "id" {
  value = resource.postgres.other2.id
}

output "disabled" {
  value = resource.postgres.other2.disabled
}

output "depends_on" {
  value = resource.postgres.other2.depends_on
}

//output "meta_id" {
//  value = resource.postgres.other2.meta.id
//}

// modules can use a git ref to be remotely downloaded from the source
//module "mymodule_1" {
//  source = "github.com/jumppad-labs/hclconfig?ref=9173050/example/modules//db"
//
//  variables = {
//    db_username = variable.db_username
//    db_password = "topsecret"
//  }
//}

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
//output "module1_connection_string" {
//  value = module.mymodule_1.output.connection_string
//}

output "module2_connection_string" {
  value = module.mymodule_2.output.connection_string
}