# Changelog

All notable changes to `defc` will be documented in this file.

## v1.45.0 — 2026-04-21

### 🔒 Security — unsafe.Pointer UB

- **`MultipartBody[T]` / `JSONBody[T]` first-embedded-field invariant
  was enforced *after* an `unsafe.Pointer` cast.** Both `Read`
  implementations performed `x = *(*T)(unsafe.Pointer(b))` and *then*
  checked that `MultipartBody` / `JSONBody` was the first embedded
  field of `T`. When a user violated the layout invariant (e.g. a
  non-empty field preceded the embedded body), the cast read past the
  allocation boundary of `b`: under `-race` or
  `-gcflags=all=-d=checkptr=1` this triggered an unrecoverable
  `fatal error: checkptr: converted pointer straddles multiple
  allocations`; without `checkptr` it silently read adjacent memory —
  classical undefined behaviour. `JSONBody.Read` happened to not fault
  today because its estimation loop implicitly ran the first-field
  check before the cast, but the ordering was fragile.

  Both sites now hoist the guards above the cast. `Read` computes
  `reflect.TypeOf(x)`, verifies `Kind() == reflect.Struct`, then
  verifies `NumField() > 0 && Field(0).Anonymous && Field(0).Type ==
  reflect.TypeOf(b).Elem()` — and only then performs the
  `*(*T)(unsafe.Pointer(b))` cast. The friendly panic messages
  (`"use the value type of a struct rather than a pointer type as the
  value for generics"`, `"JSONBody is not the first embedded field of
  struct type T"`, `"MultipartBody is not the first embedded field of
  struct type T"`) are preserved verbatim; regression tests now cover
  both the dislocated and zero-field-generic variants and the suite is
  green under `-race -count=2` and `-gcflags=all=-d=checkptr=1`.

  `test.sh` grew a `go test -gcflags=all=-d=checkptr=1 ./runtime/...`
  gate (between the plain runtime test and `sqlx`) so contributors
  without `-race` still catch any regression of this UB class.

### 🔒 Security / 🐛 Bug fixes

- **Pool-backed response-body aliasing (api mode).** Fixed a concurrency
  hazard in api-mode generated code where `__rt.NewResponseError` (and its
  `api/nort` equivalent) and user-defined `Response.FromBytes`
  implementations received a `[]byte` that aliased a `sync.Pool`-owned
  `*bytes.Buffer`. After the enclosing method returned, the buffer was
  reset and recycled; any `ResponseError.Body()` captured by the caller,
  and any slice retained by a custom `Response` implementation, could then
  observe bytes written by a subsequent unrelated request on another
  goroutine.

  `runtime.NewResponseError` now defensively copies its `body` argument,
  and generated code copies the response-body slice through the new
  `runtime.DetachBytes(*bytes.Buffer) []byte` helper before passing it to
  `Response.FromBytes` and to `NewResponseError`. The `api/nort` local
  constructor `__<Ident>NewResponseError` is regenerated with the same
  copy semantics, alongside a local `__<Ident>DetachBytes` helper.
  `runtime.PutBuffer` additionally resets the buffer on return to the
  pool (belt-and-suspenders against future regressions).

  This fix is mandatory and has no opt-out flag; users with tight
  allocation budgets should migrate to the `api/future` code path, whose
  `FromResponse(name string, r *http.Response)` contract streams the body
  directly and never materializes it through a pool.

  **Action required:** regenerate all api-mode schemas after upgrading.

### ✨ Tooling

- New `poolalias` analyzer under `gen/analyzer/poolalias` detects any
  future reintroduction of the pool-aliasing pattern — a `[]byte` from
  `*bytes.Buffer.Bytes()` on a pool-owned buffer flowing into
  `NewResponseError`, `Response.FromBytes`, or any
  `__<Ident>NewResponseError`, without passing through `DetachBytes` or
  `__<Ident>DetachBytes` first. Install as a `go vet` vettool:

  ```
  go install github.com/x5iu/defc/gen/analyzer/poolalias/cmd/poolalias
  go vet -vettool="$(go env GOPATH)/bin/poolalias" ./...
  ```

  Silence a specific call site with a `// lint:pool-alias-ok <reason>`
  comment on the sink line or the line immediately preceding it. Empty
  reasons are themselves a diagnostic.

- `test.sh` now runs a `ripgrep` stopgap check and the `poolalias`
  analyzer before the existing `go test` matrix, gating CI on the
  pool-aliasing invariants.

### ⏫ Dependencies

- Bumped `golang.org/x/tools` to `v0.28.0` (required by the analyzer and
  compatible with Go 1.22+ toolchains). Implicit `go` directive bumped
  from `1.19` to `1.22.0`; integration sub-modules (`gen/integration/api`,
  `gen/integration/sqlx`) retain their existing `go` directives.
