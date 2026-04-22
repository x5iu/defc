# Changelog

## Unreleased

### 🔒 Security — template-interpolation validators (opt-in)

Runtime validators guarding generated api/sqlx code against
CRLF / SSRF / SQL-injection / placeholder-arity bugs introduced
via templated caller values. Gated by three new feature flags
(default OFF): `api/strict-headers`, `api/strict-url`, `sqlx/strict`.
Typed errors all wrap `runtime.ErrUnsafeInterpolation`.
