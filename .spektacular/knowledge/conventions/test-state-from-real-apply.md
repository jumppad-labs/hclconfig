---
tags: [testing, state, fixtures]
---

# Generate test state with a real apply, not a hand-written state file

- When a test needs earlier state, such as a resource that is `created`, `updated`, `failed` or `destroy_failed`, produce it by running a real apply that leads to that state: a first apply, a failing create, a failing destroy, or an edited config. Then run the apply under test against it.
- This keeps test state identical to what the system actually writes, so tests don't break when the state format changes. It also exercises the whole lifecycle along the way.
- **Exception:** hand-write a state file only when the test is *about* the state format itself, for example asserting that apply writes state in a specific shape. That test needs a fixed expected document to compare against.
