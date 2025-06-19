// Example person resource configuration
// This demonstrates the HCL syntax for defining a person resource

resource "person" "john_doe" {
  first_name = "John"
  last_name  = "Doe"
  age        = 30
  email      = "john.doe@example.com"
  address    = "123 Main St, Anytown, USA"
}

resource "person" "jane_smith" {
  first_name = "Jane"
  last_name  = "Smith"
  age        = 25
  email      = "jane.smith@example.com"
  address    = "456 Oak Ave, Somewhere, USA"
}

// Minimal person resource with only required fields
resource "person" "minimal_person" {
  first_name = "Alice"
  last_name  = "Johnson"
  // Optional fields (age, email, address) are omitted
}