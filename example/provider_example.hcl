variable "containerd_socket" {
  default = "/run/containerd/containerd.sock"
}

provider "container" {
  source = "jumppad/containerd"
  version = "~> 1.0"
  
  config {
    socket = variable.containerd_socket
    namespace = "default"
    snapshotter = "overlayfs"
    runtime = "runc"
  }
}

resource "container" "web" {
  provider = "container"
  image = "nginx:latest"
  name = "web-server"
  command = ["nginx", "-g", "daemon off;"]
}