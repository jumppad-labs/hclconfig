# internal/xcl — derived from HashiCorp HCL

This directory contains a modified copy of the HashiCorp Configuration Language
(HCL) library.

| | |
|---|---|
| Upstream | https://github.com/hashicorp/hcl |
| Module | `github.com/hashicorp/hcl/v2` |
| Version | `v2.21.0` |
| Copyright | Copyright (c) 2014 HashiCorp, Inc. |
| License | Mozilla Public License, version 2.0 — see [LICENSE](LICENSE) |

## Licensing

Every file in this directory is licensed under MPL-2.0, including any
modifications made to it. The rest of this repository is licensed under
Apache-2.0; the MPL-2.0 boundary is this directory.

When working in this directory:

- Keep the existing `Copyright (c) HashiCorp, Inc.` and
  `SPDX-License-Identifier: MPL-2.0` header on every file.
- When modifying a file, add `// Modifications Copyright (c) Jumppad Labs`
  below the existing header if it is not already present.
- Any new file containing code copied or adapted from this directory must also
  carry the MPL-2.0 header and live in this directory.
- Do not move code from this directory into Apache-2.0 licensed files.
- Do not use the HCL or HashiCorp names in a way that implies endorsement.

## What was imported

Only the packages this module depends on:

- `.` (root `hcl` package)
- `ext/customdecode`
- `gohcl`
- `hclparse`
- `hclsyntax` (including the Ragel sources for the generated scanners)
- `hclwrite`
- `json`

Import paths were rewritten from `github.com/hashicorp/hcl/v2/...` to
`github.com/jumppad-labs/xcl/internal/xcl/...`.

## Modifications

- `hclparse/parser.go`: `ParseHCLFile` renamed to `ParseXCLFile`.
- Test files `ops_test.go`, `hclsyntax/parser_test.go`,
  `hclsyntax/structure_at_pos_test.go`, `hclsyntax/walk_test.go`,
  `hclsyntax/expression_static_test.go`: non-constant format strings passed to
  `t.Errorf`/`t.Logf` changed to use `"%s"`, required by `go vet` under Go 1.24+.
- `hclsyntax/token_type_string.go`, `json/tokentype_string.go`: added the
  missing MPL-2.0 header to these generated files. Re-running `go generate`
  (stringer) will drop it; re-add it afterwards.
