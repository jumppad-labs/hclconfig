a "abc" "123" {
  foo = "bar"
  count = 1
  float = 1.23
  map = {
    key = "value"
  }

  network {
    name = "default"
    enabled = true
  }
}

a "abc" "456" {
  foo = var.a
  count = 2
  float = 3.33
  map = {
    a = "b"
  }

  network {
    name = "default"
    enabled = false
  }
}

/*
[{
  "name": "foo",
  "tags": "hcl:'"foo\"",
  "type": "string",
}]
*/