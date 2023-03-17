
container "base" {
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
