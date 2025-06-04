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

  run_as {
    user = "root"
    group = "root"
  }

  port {
    local = 80
    remote = 80
  }
}

output "struct" {
  value = resource.container.nginx.run_as.user
}

output "list" {
  value = resource.container.nginx.port.0.local
}


output "list_direct" {
  value = resource.container.nginx.port.0
}


output "list_bracket_direct" {
  value = resource.container.nginx.port[0]
}


output "list_bracket" {
  value = resource.container.nginx.port[0].local
}


output "map" {
  value = resource.container.nginx.env.key
}


output "map_bracket" {
  value = resource.container.nginx.env["key"]
}


output "complex_map" {
  value = resource.container.nginx.created_network_map.first.subnet
}

output "complex_map_bracket" {
  value = resource.container.nginx.created_network_map["first"].subnet
}


output "cty" {
  value = resource.container.nginx.output.value
}

output "created_network_id" {
  value = resource.container.nginx.created_network.0.name
}