module "disabled" {
  disabled = true

  source = "./modules"
}

module "disabled_internal" {
  source = "./modules"

  variables = {
    disable_resources = true
  }
}