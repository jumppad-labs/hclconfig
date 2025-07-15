provider "simple1" {
  source = "test/simple"
  version = "1.0.0"
  
  config {
    value = "provider1"
    count = 1
  }
}

provider "simple2" {
  source = "test/simple"
  version = "1.0.0"
  
  config {
    value = "provider2"
    count = 2
  }
}