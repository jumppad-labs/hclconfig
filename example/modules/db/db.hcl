variable "db_username" {
  default = "admin"
}

variable "db_password" {
  default = "password"
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
