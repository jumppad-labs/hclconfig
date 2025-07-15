variable "var1" {
  default = variable.var2
}

variable "var2" {
  default = variable.var1
}

provider "error_test" {
  source = "test/error"
  version = "1.0.0"
  
  config {
    required_field = variable.var1
  }
}