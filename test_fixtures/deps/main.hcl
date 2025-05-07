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
#   value = resource.container.nginx.port.0.local
# }

# output "list_x" {
#   value = resource.container.nginx.port.0.fail
# }

# output "list_direct_o" {
#   value = resource.container.nginx.port.0
# }

# output "list_direct_x" {
#   value = resource.container.nginx.port.1
# }

# output "list_direct_o" {
#   value = resource.container.nginx.port[0]
# }

# output "list_direct_x" {
#   value = resource.container.nginx.port[1]
# }

# output "list_bracket_o" {
#   value = resource.container.nginx.port[0].local
# }

# output "list_bracket_x" {
#   value = resource.container.nginx.port[0].fail
# }

# output "map_o" {
#   value = resource.container.nginx.env.key
# }

# output "map_x" {
#   value = resource.container.nginx.env.fail
# }

# output "map_bracket_o" {
#   value = resource.container.nginx.env["key"]
# }

# output "map_bracket_x" {
#   value = resource.container.nginx.env["fail"]
# }

# output "complex_map_o" {
#   value = resource.container.nginx.created_network_map.first.subnet
# }

# output "complex_map_bracket_o" {
#   value = resource.container.nginx.created_network_map["first"].subnet
# }

# output "complex_map_bracket_x" {
#   value = resource.container.nginx.created_network_map["first"].fail
# }

# output "cty_o" {
#   value = resource.container.nginx.output.value
# }

# output "cty_x" {
#   value = resource.container.nginx.network.value
# }