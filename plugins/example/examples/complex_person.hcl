// Complex person resource configuration demonstrating various scenarios

// Full person with all fields
resource "person" "full_profile" {
  first_name = "Robert"
  last_name  = "Williams"
  age        = 35
  email      = "robert.williams@company.com"
  address    = "789 Corporate Blvd, Suite 100, Business City, BC 12345"
}

// Person with special characters in name
resource "person" "international" {
  first_name = "José"
  last_name  = "García-Rodríguez"
  age        = 28
  email      = "jose.garcia@universidad.es"
  address    = "Calle Principal 123, Madrid, España"
}

// Very young person
resource "person" "young_user" {
  first_name = "Emma"
  last_name  = "Thompson"
  age        = 18
  email      = "emma.thompson@school.edu"
}

// Senior person
resource "person" "senior_user" {
  first_name = "Frank"
  last_name  = "Miller"
  age        = 75
  email      = "frank.miller@retirement.org"
  address    = "Senior Living Community, Peaceful Valley"
}

// Person with long address
resource "person" "detailed_address" {
  first_name = "Sarah"
  last_name  = "Anderson"
  age        = 40
  email      = "sarah.anderson@longcompanyname.co.uk"
  address    = "Apartment 4B, The Grand Victorian Building, 1234 Westminster Boulevard, London Borough of Camden, Greater London, United Kingdom, EC1A 1BB"
}