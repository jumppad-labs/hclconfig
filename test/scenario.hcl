resource "remote_exec" "setup" {
  command = [""]
}

variable "next_button" {
  default = ".next_button"
}

scenario "Test installation setup" {
  // load the module from the parent folder
  source = "../"

  test {
    context "the docs are loaded on on the home page" {
      before = [
        is_complete(resource.remote_exec.setup.id),
        navigate_to(resource.docs.docs.url),
        and_wait_for_selector("#thing")
      ]
    }

    it "runs the check script which should fail" {
      expect = run_check_script(resource.task.install.id).to().fail
    }

    it "runs the solve script to advance to the next steps" {
      expect = run_solve_script(resource.task.install.id).to().succeed
    }

    it "runs the check script" {
      expect = run_check_script(resource.task.install.id).to().succeed
    }

    it "clicks next and the next page should be displayed" {
      expect = click_element(variable.next_button).and().wait_for_text("Congratulations")
    }
  }

  test {
    context "the docs are loaded on on the home page" {
      before = [
        is_complete(resource.remote_exec.setup.id),
        navigate_to(resource.docs.docs.url).and().wait_for_selector("#thing")
      ]
    }

    it "runs the check script which should fail" {
      execute = check_script(resource.task.install.id)
      expect { result = failure() }
    }

    it "runs the solve script to advance to the next steps" {
      execute = solve_script(resource.task.install.id)
      expect { result = success() }
    }

    it "runs the check script" {
      execute = solve_script(resource.task.install.id)
      expect { result = success() }
    }

    it "clicks next and the next page should be displayed" {
      execute = check_script(resource.task.install.id)
      expect { result = success() }
    }
  }
}

scenario "Testing Modules" {
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
    test.welcome_page
  ]
}

test "welcome_page" {
  description = "we should test the features of the welcom page to ensure correctness"

  it "the resources for the module should have been created" {
    expect = resources_are_created(
      resource.network.cloud,
      resource.network.onprem,
      resource.container.consul,
      resource.sidecar.envoy,
      resource.k8s_cluster.k3s,
      resource.docs.docs
    ) // assertion function
  }

  it "should be possible to query the consul members on the first cluster" {
    execute = http_post("http://consul.container.shipyard.run:8500/v1/agent/members",
      {
        body    = "something",
        headers = { "x-consul-api-key" = "api_key" }
    })

    expect { result = to_return_status_code(200) }
    expect { result = the_body_contains("10.10.1.1") }
  }

  it "should be possible to query the consul members on the second cluster" {
    expect = [
      http_post("http://consul.container.shipyard.run:8500/v1/agent/members"), // command function
      with_headers({ "x-consul-api-key" = "api_key" }),                        // parameter function
      and_body("Blah"),                                                        // parameter function
      to_return_status_code(200),                                              // assertion function
      and(),                                                                   // comparitor function
      the_body_contains("10.10.1.1"),                                          // assertion function
    ]
  }

  it "should be possible to query the consul members on the second cluster" {
    exec = [
      http_post("http://consul.container.shipyard.run:8500/v1/agent/members"), // command function
      with_headers({ "x-consul-api-key" = "api_key" }),                        // parameter function
      and_body("Blah"),                                                        // parameter function
    ]

    expect = [
      to_return_status_code(200),     // assertion function
      and(),                          // comparitor function
      the_body_contains("10.10.1.1"), // assertion function
    ]
  }
}