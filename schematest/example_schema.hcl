schema "person" {
  field "name" {
    type = "string"
  }

  field "age" {
    type = "int"
  }

  field "pets" {
    type = "block"

    field "name" {
      type = "string"
    }

    field "age" {
      type     = "int"
      required = true
    }

    field "id" {
      type     = "int"
      computed = true
    }
  }
}