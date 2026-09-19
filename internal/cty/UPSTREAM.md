# internal/cty — derived from go-cty

This directory contains a copy of the go-cty library, which implements the
type system and values that XCL configuration is evaluated with.

| | |
|---|---|
| Upstream | https://github.com/zclconf/go-cty |
| Module | `github.com/zclconf/go-cty` |
| Version | `v1.15.0` |
| Copyright | Copyright (c) 2017-2018 Martin Atkins |
| License | MIT License — see [LICENSE](LICENSE) |

`ctydebug` is from a separate module:

| | |
|---|---|
| Upstream | https://github.com/zclconf/go-cty-debug |
| Module | `github.com/zclconf/go-cty-debug` |
| Version | `v0.0.0-20240509010212-0d6042c53940` |
| Copyright | Copyright (c) 2019 Martin Atkins |
| License | MIT License — see [ctydebug/LICENSE](ctydebug/LICENSE) |

## Licensing

Every file in this directory is licensed under the MIT License, including any
modifications made to it. Keep `LICENSE` and `ctydebug/LICENSE` alongside the
code.

## What was imported

Only the packages this module depends on, from the module's `cty` directory:

- `.` (root `cty` package)
- `convert`
- `ctystrings`
- `function`, `function/stdlib`
- `gocty`
- `json`
- `set`
- `ctydebug` (from go-cty-debug, used by tests)

`msgpack` was not imported. Import paths were rewritten from
`github.com/zclconf/go-cty/cty/...` and `github.com/zclconf/go-cty-debug/ctydebug`
to `github.com/jumppad-labs/xcl/internal/cty/...`.

## Modifications

- `gocty/helpers.go`, `gocty/in.go`, `gocty/out.go`, `gocty/type_implied.go`:
  attributes of embedded (anonymous) structs are flattened into the parent
  object. Carried over from the jumppad-labs/go-cty fork.
- `gocty/helpers.go`: attribute names are read from `xcl` struct tags, using
  `internal/xcl/tags`, instead of `cty` tags.
- `gocty/type_implied.go`: error message refers to `xcl` field tags.
- `gocty/in.go`: gofmt.
- `gocty` test files: struct tags changed from `cty` and `hcl` to `xcl`.
