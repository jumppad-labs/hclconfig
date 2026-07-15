# Testing & Mocking

**Tier:** always-applied

- Include unit tests for all business logic.
- Use testify `require` for unit tests.
- Use Mockery for mocking interfaces.
- NEVER use table-driven tests.
- Add integration tests for HTTP handlers.
- NEVER mix positive and negative tests in the same test function.
- Ensure tests are easy to read, favor verbosity over too much abstraction.
