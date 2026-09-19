# internal/dag — derived from silas/dag

This directory contains an unmodified copy of the dag library, which XCL uses
to order resources by their dependencies. silas/dag is itself extracted from
HashiCorp Terraform.

| | |
|---|---|
| Upstream | https://github.com/silas/dag |
| Module | `github.com/silas/dag` |
| Version | `v0.0.0-20220518035006-a7e85ada93c5` |
| License | Mozilla Public License, version 2.0 — see [LICENSE](LICENSE) |

## Licensing

Every file in this directory is licensed under MPL-2.0, including any
modifications made to it. When modifying a file, add
`// Modifications Copyright (c) Jumppad Labs` at the top of it and list the
change below.

## Modifications

None. Only the import path changed, from `github.com/silas/dag` to
`github.com/jumppad-labs/xcl/internal/dag`.
