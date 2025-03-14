a "abc" "123" {
  foo = "bar"

  network {
    name = "default"
  }
}

a "abc" "456" {
  foo = var.a

  network {
    name = "default"
  }
}

/*
[{
  "name": "foo",
  "tags": "hcl:'"foo\"",
  "type": "string",
}]
*/