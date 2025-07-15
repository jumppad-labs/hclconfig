variable "endpoint" {
  default = "https://api.example.com"
}

provider "simple" {
  source = "test/simple"
  version = "1.0.0"
  
  config {
    value = variable.endpoint
    count = 100
  }
}

resource "simple" "app1" {
  provider = "simple"
  data = "application 1"
}

resource "simple" "app2" {
  provider = "simple" 
  data = "application 2"
}