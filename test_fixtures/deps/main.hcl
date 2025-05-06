resource "network" "main" {
  subnet = "10.0.5.0/24"
}

resource "container" "nginx" {
  command = ["bla", "bla"]

  env = {
    key = "value"
  }

  created_network_map = {
    first = resource.network.main
  }
}

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

output "map_o" {
  value = resource.container.nginx.env.key
}

output "map_x" {
  value = resource.container.nginx.env.fail
}

# resource "container" "dependency" {
#   command = [resource.container.nginx.something]
# }

# output "correct_field" {
#   value = resource.container.nginx.command
# }

# output "incorrect_field" {
#   value = resource.container.nginx.incorrect
# }


