variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "password"
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

// outputs can be specified to allow values to be passed to config
// utilizing this module
output "connection_string" {
  value = resource.postgres.mydb.connection_string
}