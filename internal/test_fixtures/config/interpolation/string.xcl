
resource "container" "container3" {
  port {
    local  = 8500
    remote = 8500
  }
}

resource "container" "container4" {
  env = {
    "port_string" = "${resource.container.container3.port.0.local}"
  }
}