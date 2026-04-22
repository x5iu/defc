# Changelog

## Unreleased

### 🔒 Security — pool-aliased response body

- `runtime.NewResponseError` now owns an independent copy of `body` via
  `append([]byte(nil), body...)`. A new exported `runtime.DetachBytes`
  helper is available for call sites that hand pool-owned bytes to
  retaining consumers.
- Generator: `gen/template/api.tmpl` hoists a `bodyCopy<Method>` local via
  `DetachBytes` / `__<Ident>DetachBytes` at the three sites that
  previously aliased the pool buffer. In nort mode the emitted
  `__<Ident>NewResponseError` also defensively copies `body`.
- Toolchain bumped to go 1.22.0; CI matrix drops 1.19/1.20/1.21.
- Users should regenerate all api-mode schemas to pick up the template
  changes.
