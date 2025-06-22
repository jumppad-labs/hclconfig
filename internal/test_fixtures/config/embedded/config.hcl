resource "container" "mine" {
  // common properties
  id         = "mycontainer"
  entrypoint = ["echo"]
  command    = ["hello", "world"]
  env = {
    NAME = "value"
  }
  dns               = ["container-dns"]
  privileged        = true
  max_restart_count = 5

  // specific container properties
  container_id = "mycontainer"
}

resource "sidecar" "mine" {
  // common properties
  id         = resource.container.mine.id
  entrypoint = ["echo"]
  command    = ["hello", "world"]
  env = {
    NAME = "value"
  }
  dns               = ["container-dns"]
  privileged        = false
  max_restart_count = 3

  // specific sidecar properties
  sidecar_id = "mysidecar"
}