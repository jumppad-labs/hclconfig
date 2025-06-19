resource "container" "postgres" {
  image = "postgres:15"
  
  network {
    id = resource.network.postgres_network.meta.id
    ip_address = "10.10.0.100"
    aliases = ["postgres.db"]
  }
  
  env = {
    POSTGRES_DB       = "myapp"
    POSTGRES_USER     = "postgres"
    POSTGRES_PASSWORD = "secretpassword"
  }
  
  port {
    local  = 5432
    remote = 5432
    host   = "localhost"
  }
  
  volume {
    source      = "./postgres-data"
    destination = "/var/lib/postgresql/data"
    type        = "bind"
  }
}