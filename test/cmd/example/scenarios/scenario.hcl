//resource "remote_exec" "setup" {
//  command = [""]
//}

variable "next_button" {
  default = ".next_button"
}

variable "outputs" {
  default = {
    consul_leader = "10.1.23.3"
  }
}

resource "scenario" "testing_modules" {
  description = "The configurations modules should be tested"

  // load the module from the given folder
  source = "../"

  // test is executed for each context
  // the blueprint is run up and down for 
  // each test
  context "with consul 1.14" {
    env = {
      CONSUL_VERSION = "1.8.0"
      ENVOY_VERSION  = "1.14.3"
    }
  }

  context "with consul 1.16" {
    variables = {
      consul_version = "1.8.0"
      envoy_version  = "1.16.3"
    }
  }

  tests = [
    resource.test.welcome_page
  ]
}

resource "test" "welcome_page" {
  description = "we should test the features of the welcom page to ensure correctness"

  before = []

  it "should create the following the resources for the module" {
    expect = [
      resources_are_created([
        resource.postgres.mydb.id,
        resource.postgres.other1.id,
        resource.postgres.other2.id,
        resource.config.myapp.id,
        ]
      ) // assertion function
    ]
  }

  it "should be possible to query the consul members on the second cluster" {
    expect = [
      http_post("http://consul.container.shipyard.run:8500/v1/agent/members"), // command function
      with_headers({ "x-consul-api-key" = "api_key" }),                        // parameter function
      and(),                                                                   // comparitor function
      with_body("Blah"),                                                       // parameter function
    ]

    to = [
      return_status_code(200),    // assertion function
      and(),                      // comparitor function
      body_contains("10.10.1.1"), // assertion function
    ]

    outputs = {
      consul_leader = body() // output function
    }
  }

  it "should be possible to check the leader is reachable" {
    expect = [
      script("./script.sh"),
      with_arguments({ leader_ip = variable.outputs.consul_leader }), // parameter function
    ]

    to = [
      have_an_exit_code(0),      // assertion function
      and(),                     // comparitor function
      output("correct version"), // assertion function
    ]
  }

  after = []
}