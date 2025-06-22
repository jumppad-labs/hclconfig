a "abc" "456" {
  foo   = var.a
  count = 2
  float = 3.33

  foo_ref   = var.a
  count_ref = 3
  float_ref = 4.33

  map = {
    a = "b"
  }
  slice = ["a", "b", "c"]

  network_map = {
    default = {
      name    = "default"
      enabled = true
    }
    other = {
      name    = "other"
      enabled = false
    }
  }

  network {
    name    = "one"
    enabled = true
  }

  network {
    name    = "two"
    enabled = false
  }

  network_struct {
    name    = "struct"
    enabled = true
  }

  network_ref {
    name    = "ref"
    enabled = false
  }

  nested_1 {
    name = "foo"
  }

  nested_2 {
    name = "bar"
    inner {
      name = "baz"
    }
  }
}