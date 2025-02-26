variable "default" {
  type = "string"
  default = "test"
}

variable "typed" {
  type = "number"
}

resource "container" "consul" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  resources {
    # Max CPU to consume, 1024 is one core, default unlimited
    cpu = variable.typed
  }

  volume {
    source      = "images.volume.shipyard.run"
    destination = variable.default
    type        = "volume"
  }
}