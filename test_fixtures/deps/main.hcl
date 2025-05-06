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
}

# resource "container" "second" {
#   env = {
#     key = resource.container.nginx.output
#   }
# }

# output "struct_o" {
#   value = resource.container.nginx.resources.cpu
# }

# output "struct_x" {
#   value = resource.container.nginx.resources.fail
# }

# output "list_o" {
#   value = resource.container.nginx.ports.local
# }

# output "list_x" {
#   value = resource.container.nginx.ports.fail
# }

# output "map_o" {
#   value = resource.container.nginx.env.key
# }

# output "map_x" {
#   value = resource.container.nginx.env.fail
# }

output "cty_o" {
  value = resource.container.nginx.output.value
}

output "cty_x" {
  value = resource.container.nginx.network.value
}