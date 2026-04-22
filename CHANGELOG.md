# Changelog

## Unreleased

### 🔒 Security — unsafe.Pointer UB in JSON/MultipartBody

`JSONBody[T].Read` and `MultipartBody[T].Read` now verify the
first-embedded-field layout invariant BEFORE performing
`*(*T)(unsafe.Pointer(b))`. This eliminates a class of
"converted pointer straddles multiple allocations" fatals under
`-race` / `-gcflags=all=-d=checkptr=1` when callers violated the
layout. `test.sh` gains a dedicated checkptr gate across
`./runtime/...`.
