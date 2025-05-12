resource "network" "main" {
  subnet = "10.0.5.0/24"
}

resource "container" "nginx" {
  command = ["bla", "bla"]

  network {
    name = resource.network.main.meta.name
  }

  env = {
    key = 2
  }

  created_network_map = {
    first = resource.network.main
  }

  port {
    local = 80
    remote = 80
  }
}

output "struct_invalid_field" {
  value = resource.container.nginx.network.value
}

output "struct_nil" {
  value = resource.container.nginx.resources.cpu
}

output "struct_nil_invalid_field" {
  value = resource.container.nginx.resources.fail
}


output "list_invalid_field" {
  value = resource.container.nginx.port.0.fail
}


output "list_direct_invalid_index" {
  value = resource.container.nginx.port.1
}


output "list_bracket_direct_invalid_index" {
  value = resource.container.nginx.port[1]
}


output "list_bracket_invalid_field" {
  value = resource.container.nginx.port[0].fail
}


output "map_invalid_key" {
  value = resource.container.nginx.env.fail
}


output "map_bracket_invalid_key" {
  value = resource.container.nginx.env["fail"]
}

output "complex_map_bracket_invalid_field" {
  value = resource.container.nginx.created_network_map["first"].fail
}

output "invalid_func" {
  value = invalid("first")
}