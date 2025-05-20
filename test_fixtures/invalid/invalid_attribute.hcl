resource "container" "name"{
  command = ["consul", "agent", "-dev", "-client", "0.0.0.0"]

  something = "invalid"
}