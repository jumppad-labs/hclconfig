resource "container" "with_networks" {
  network {
    name       = "one"
    ip_address = "127.0.0.1"
  }

  network {
    name       = "two"
    ip_address = "127.0.0.2"
  }
}

resource "container" "default" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  env = {
    "len_string"     = len("abc")
    "len_collection" = len(["one", "two"])
    "env"            = env("MYENV")
    "home"           = home()
    "file"           = file("./default.hcl")
    "dir"            = dir()
    "trim"           = trim("  foo bar  ")
  }
}
