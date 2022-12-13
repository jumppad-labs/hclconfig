container "base" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  env = {
    "len" = len("abc")
    "env" = env("MYENV")
    "home" = home()
    "file" = file("./default.hcl")
    "dir" = dir()
  }
}
