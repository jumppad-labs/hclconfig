container "base" {
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  env = {
    "len" = constant_number()
  }
}
