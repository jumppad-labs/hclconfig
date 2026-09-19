---
tags: [testing, schema, fixtures, meta, golden]
---

# New `types.Meta` fields must be added to the golden schema

Adding a field to `types.Meta` (`types/resource.go`) changes the schema that
`internal/schema` generates for every resource. `TestSerializeEmbedded`
compares that output with the hand-written golden JSON `EmbeddedJson` in
`internal/schema/test_fixtures/embedded.go`, so the test fails until the new
field is added there as well.

- Add the field in the same position as in the struct, in **both** places
  `Meta` appears in `EmbeddedJson` (it is embedded twice), with its exact
  `name`, `type` and `tags` strings.
- `embedded.go` is an ordinary `.go` file, not a `_test.go` file, so it is easy
  to miss when only test files are being updated.

Found on 2026-09-19 when `Meta.Parents` was added (destroy-cycle plan, Phase 1.1).
