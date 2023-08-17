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

resource "test_browser" "chrome" {
  config {
    height = 1920
    width  = 1080
  }
}


scenario "testing_configuration" {
  description = "The configurations should be tested"

  // load the module from the given folder
  source = "../"

  // test is executed for each context
  // the blueprint is run up and down for 
  // each test
  context "with consul x.15" {
    env = {
      HCL_VAR_consul_dc1_version = "1.15.0"
      HCL_VAR_consul_dc2_verison = "2.15.0"
    }
  }

  context "with consul x.16" {
    variables = {
      consul_dc1_version = "1.16.0"
      consul_dc2_version = "2.16.0"
    }
  }

  tests = [
    test.configuration_test
  ]
}

test "configuration_test" {
  description = "we should test the features of the configuration to ensure correctness"

  before = [
    navigate({ browser = resource.test_browser.chrome.id, to = "http://www.google.com" }),
    and(),
    wait_for_selector(".button")
  ]

  it " should create the following the resources for the module " {
    expect = [
      resources_are_created([
        resource.container.consul_dc1.id,
        resource.container.consul_dc2.id,
        ]
      ) // assertion function
    ]
  }

  it " should be possible to query the consul members on the first cluster " {
    expect = [
      http_post(" http : //consul_dc1.container.shipyard.run:8500/v1/agent/members"), // command function
      with_headers({ "x-consul-api-key" = "api_key" }),                               // parameter function
      and(),                                                                          // comparitor function
      with_body("Blah"),                                                              // parameter function
    ]

    to = [
      return_status_code(200),    // assertion function
      and(),                      // operand function
      body_contains("10.10.1.1"), // assertion function
    ]

    outputs = {
      consul_leader = body() // output function
    }
  }

  it "should be possible to query the consul members on the second cluster" {
    expect = [
      http_post("http://consul_dc2.container.shipyard.run:8500/v1/agent/members"), // command function
      with_headers({ "x-consul-api-key" = "api_key" }),                            // parameter function
      and(),                                                                       // comparitor function
      with_body("Blah"),                                                           // parameter function
    ]

    to = [
      return_status_code(200),    // assertion function
      and(),                      // comparitor function
      body_contains("10.10.1.2"), // assertion function
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

  after = [
    http_post("http://consul_dc1.container.shipyard.run:8500/v1/agent/members"), // command function
    http_post("http://consul_dc2.container.shipyard.run:8500/v1/agent/members")  // command function
  ]
}