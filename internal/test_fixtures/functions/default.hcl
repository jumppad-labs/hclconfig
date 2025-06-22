resource "network" "one" {
  subnet = "10.0.0.1/16"
}

resource "network" "two" {
  subnet = "10.0.0.1/16"
}


resource "container" "with_networks" {
  network {
    name       = "one"
    ip_address = "127.0.0.1"
  }

  network {
    name       = "two"
    ip_address = "127.0.0.2"
  }

  created_network_map = {
    "one" = resource.network.one
    "two" = resource.network.two
  }
}

resource "container" "default" {

  env = {
    "len_string"     = len("abc")
    "len_collection" = len(["one", "two"])
    "env"            = env("MYENV")
    "home"           = home()
    "file"           = file("./default.hcl")
    "dir"            = dir()
    "trim"           = trim("  foo bar  ")
    "template_file" = template_file("template.tmpl", {
      name   = "Raymond"
      number = 43
      list   = ["cheese", "ham", "pineapple"]
      map = {
        foo = "bar"
        x   = 1
      }
    })
  }

  dns = resource.container.with_networks.created_network_map != null ? values(resource.container.with_networks.created_network_map).*.meta.name : []

  entrypoint = values({
    "one" = { "id" = "123" }
    "two" = { "id" = "abc" }
  }).*.id

  command = keys({
    "one" = "1"
    "two" = "2"
  })
}