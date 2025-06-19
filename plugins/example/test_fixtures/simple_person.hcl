// Simple single person resource for basic testing
resource "person" "test_person" {
  first_name = "Test"
  last_name  = "User"
  age        = 42
  email      = "test@example.com"
}