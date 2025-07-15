variable "test_value" {
  default = "test"
}

provider "simple" {
  source = "test/simple"
  version = "1.0.0"
  
  config {
    value = variable.test_value
    count = 42
  }
}

resource "simple" "test" {
  data = "hello world"
}