---
tags: [hcl, parsing, struct-tags, fork]
---

# XCL will own a full fork of the HCL library, with XCL struct tags

**Decision:** XCL will fork the HCL parsing library completely into an XCL-owned version. Resource types will declare fields with XCL-specific struct tags that carry everything XCL needs, for example `xcl:"connection_string,optional,computed"`, instead of `hcl:"..."` tags.

**Accepted cost:** once forked, XCL no longer takes upstream HCL updates. The fork may later be rewritten outright.

**Why:** upstream gohcl only understands its own `hcl` tag kinds and panics on any option it doesn't know (`gohcl/schema.go`, "invalid hcl field tag kind"). XCL-specific concepts such as provider-owned `computed` fields therefore can't live in the tag that drives decoding. Every new concept means another parallel tag key, or reflection tricks that rewrite tags before decode, and those tricks only help the host side. The plugin side would still need matching `json` tags. Owning the parser lets the tag syntax fit XCL.

**Alternatives considered:**
- **Keep upstream HCL and add separate tag keys**, such as `xcl:"computed"` next to `hcl:"...,optional"`. This works today and is what the provider-lifecycle-read plan uses as the interim step. The cost is that each resource type carries two or three overlapping tags.
- **Keep upstream HCL and generate `hcl`/`json` tags from `xcl` tags with reflection** when the plugin schema is rebuilt (`reflect.StructOf` in `internal/schema/deserialize.go`). This is possible on the host, but the plugin-side adapter unmarshals into the author's real type, so it still needs `json` tags or a mirror-type translation. Builtin types and the plugin test helpers need the same treatment.

**Status:** the direction is agreed but not yet specced. It will be a follow-up spec. Until then, new XCL-only field markers go in a separate `xcl` tag key alongside the `hcl` tag, so that they can be folded into the forked tag syntax later.
