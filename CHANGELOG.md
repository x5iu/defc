# Changelog

## Unreleased

### 🔒 Security — strict NAMED-arg merge (opt-in)

New feature `sqlx/strict-merge` turns silent bind-key collisions in
NAMED-merge into fail-closed errors wrapping
`runtime.ErrNamedArgsCollision`. A companion fuzz test,
`runtime/merge_fuzz_test.go`, asserts that strict-merge never loses a
key without reporting it. Default OFF; opt-in per-schema via the new
feature flag.
