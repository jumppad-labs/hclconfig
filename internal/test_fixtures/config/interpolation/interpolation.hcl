resource "network" "test" {
  subnet = "hello erik"
}

// These fixtures define the various types of expressions that can be used within
// hcl config
resource "container" "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  networkobj = resource.network.test

  volume {
    source      = "images.volume.shipyard.run"
    destination = "/cache"
    type        = "volume"
  }

  volume {
    source      = "images.volume.jumppad.dev"
    destination = "/cache2"
    type        = "volume2"
  }

  port {
    local  = 8500
    remote = 8500
  }
}


output "function" {
  value = len(resource.container.consul.volume)
}

output "index" {
  value = resource.container.consul.volume.0.source
}

output "index_interpolated" {
  value = "root/${resource.container.consul.volume.0.source}"
}

output "splat" {
  value = resource.container.consul.volume.*.destination
}

output "splat_with_null" {
  value = resource.container.consul.created_network != null ? resource.container.consul.created_network.*.name : []
}

output "binary" {
  value = resource.container.consul.volume.0.source == resource.container.consul.volume.1.source
}

output "condition" {
  value = resource.container.consul.volume.0.source == resource.container.consul.volume.1.source ? resource.container.consul.volume.0.destination : resource.container.consul.volume.0.destination
}

output "template" {
  value = "abc/${len(resource.container.consul.volume)}"
}