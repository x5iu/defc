# Security Policy

## Trust boundary

`defc` is a **code-generation tool** executed by the developer (or by CI on
behalf of the developer) with the full privileges of that user. Its inputs —
the schema `.go` files it scans, any files referenced via `#INCLUDE`, and the
command lines executed via `#SCRIPT` — are trusted build inputs, not
untrusted user data.

**Do not run `defc generate` on schema files from untrusted sources**
(e.g. an attacker-controlled pull request merged and then `go generate`-d on
a developer laptop or in CI). Review `//go:generate` lines and any
`#INCLUDE` / `#SCRIPT` directives in a schema file the same way you would
review a shell script before invoking the generator.

## `#INCLUDE` — filesystem boundary

Since v1.45.0, `#INCLUDE` in `sqlx`-mode headers is constrained at
generate time:

- Relative paths resolve against the directory of the schema `.go` file.
- Absolute paths are rejected unless they resolve under a configured
  `--include-root=DIR`.
- `..` escapes out of the schema directory (and out of any
  `--include-root`) are rejected.
- Any symlink component in the resolved path is rejected
  (`lstat`-based walk from the anchor to the target).
- Per-file cap: 1 MiB. Aggregate cap per directive: 4 MiB.
- Glob expansion results are sorted deterministically.

Error messages include the originating `file:line` for diagnostics.

## `#SCRIPT` — deprecated

`#SCRIPT` runs an arbitrary command at code-generation time. As of
v1.45.0 it is:

- **Disabled by default.** Invocation without `--allow-script` fails with
  `#SCRIPT is disabled by default; re-run with --allow-script …`.
- **Sandboxed** when enabled: the child process runs with a scrubbed
  environment whose baseline is `PATH`, `HOME`, `USER`, `LANG`, `LC_ALL`,
  `LC_CTYPE`, `TMPDIR`, `GOCACHE`, `GOMODCACHE`, `GOPATH`. Additional
  variables can be allow-listed one at a time via repeatable
  `--script-env=NAME` (names must match `^[A-Z_][A-Z0-9_]*$`).
- **Time-bounded** via `--script-timeout` (e.g. `30s`), capped at
  10 minutes.
- **Stderr-bounded**: at most 64 KiB of the child's stderr is captured on
  failure; the remainder is truncated with a marker.
- **Logged** on success with a deprecation notice to the generator's
  stderr.

### Deprecation timeline

- **v1.45.0**: `#SCRIPT` disabled by default, gated behind
  `--allow-script`, scrubbed env, bounded timeout, deprecation warning.
- **v1.46.x**: documentation-only reminders; migration tooling stable.
- **v1.47.0** (planned): `#SCRIPT` is removed. `#INCLUDE` remains as the
  supported mechanism for pulling external SQL into a header.

### Migration

Commit the rendered SQL produced by the `#SCRIPT` body into the
repository and reference it from `#INCLUDE` instead:

```go
// Before:
// #SCRIPT cat queries/list_users.sql
// After:
// #INCLUDE "queries/list_users.sql"
```

## Template interpolation — strict runtime validators

Schema templates in `sqlx`- and `api`-mode methods render arbitrary
user data into SQL statements, URLs, and HTTP headers. A bare
`{{.x}}` action in any of those contexts is unsafe whenever the
value crosses a trust boundary: it enables SQL injection, SSRF /
URL smuggling, and CR/LF header injection respectively.

As of v1.45 the runtime ships strict validators that generated code
can call to fail closed on such inputs:

- `runtime.SQLArityCheck(query, argc)` — backed by
  `runtime/token.CountPlaceholders`; rejects queries whose rendered
  `?` count does not match the argument count, before any DB round
  trip.
- `runtime.HeaderValue`, `runtime.PreExecHeaderMap` — reject HTTP
  header values containing CR, LF, NUL, or other control bytes.
- `runtime.StrictURL(constPrefix, rendered)` — rejects control
  bytes, unparseable URLs, non-http(s) schemes, and any drift of
  scheme or host away from the template's constant prefix.
- `runtime.PathSegment`, `runtime.QueryValue`, `runtime.QuoteIdentifier`
  — safe building blocks for helper-wrapped interpolations.

All strict emission is gated by opt-in feature flags (`sqlx/strict`,
`api/strict-headers`, `api/strict-url`) that default **OFF**, so
generated output is byte-identical to prior versions unless a
feature is explicitly enabled.

## `splitArgs` is not a shell

The directive argument parser (`splitArgs` consumed by `#INCLUDE`, `#SCRIPT`,
etc.) supports `${…}` as a **grouping** construct for readability; it is
**not** shell parameter expansion. It does not expand environment
variables, does not perform command substitution, and does not invoke a
shell. Anything that looks like `${PATH}` inside a method option is a
literal token boundary, not a variable reference.

## Named-args scoping

`runtime.MergeNamedArgs` flattens every outer argument into a single
flat `map[string]any` keyed by bind name. Inputs may be authoritative
scoping values (e.g. a `context`-extracted `tenantCtx.ToNamedArgs()`
returning `{"tenant_id": real}`) and/or attacker-influenced payloads
(e.g. a request body struct tagged `db:"tenant_id"`). When two sources
contribute the same key, Go's randomised map iteration means the
winner is non-deterministic — and on a bad roll the attacker value
can silently overwrite the authoritative one, bypassing tenant /
row-level scoping in the rendered SQL.

As of v1.45.0, the opt-in `sqlx/strict-merge` feature flag causes
the generator to emit a call to `runtime.MergeNamedArgsStrict`,
which returns an error wrapping `runtime.ErrNamedArgsCollision`
whenever two distinct sources contribute the same bind key, *before*
the SQL is executed. Use `errors.As` to extract
`*runtime.NamedArgsCollisionError{Key, Sources}` for telemetry.
When the flag is absent, `runtime.MergeNamedArgs` retains its
legacy last-writer-wins behaviour. v1.46.0 will make strict mode
the default.

Provenance labels follow a stable scheme:

| Source branch                       | Label                                   |
|-------------------------------------|-----------------------------------------|
| `ToNamedArgs()` implementor         | `ToNamedArgs(<outer>)`                  |
| `driver.Valuer`                     | `driver.Valuer(<outer>)`                |
| `ToArgs` (no `ToNamedArgs`)         | `ToArgs(<outer>)`                       |
| `map[string]any` flatten            | `map(<outer>)`                          |
| `db`-tagged struct field            | `struct(<outer>.<field-path>)`          |
| Fallback scalar                     | `scalar(<outer>)`                       |

## Supported versions

Only the latest minor release line receives security fixes. Users on
older lines should upgrade before reporting issues.

| Version  | Status                   |
|----------|--------------------------|
| 1.45.x   | ✅ Supported             |
| < 1.45.0 | ⚠️ Upgrade recommended   |

## Reporting a vulnerability

Please report suspected vulnerabilities **privately** via GitHub Security
Advisories on the `x5iu/defc` repository
(`Security` → `Report a vulnerability`). Do not file a public issue for
security problems. We aim to acknowledge reports within 5 business days
and will coordinate a fix and disclosure window with the reporter.
