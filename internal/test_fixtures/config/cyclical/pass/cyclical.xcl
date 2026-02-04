module "cyclical" {
  source = "./module"
}

resource "network" "one" {
  subnet = module.cyclical.output.network.subnet
}