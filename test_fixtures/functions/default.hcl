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

  dns = values({
    "one" = "1"
    "two" = "2"
  })

  command = keys({
    "one" = "1"
    "two" = "2"
  })
}