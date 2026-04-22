# Changelog

## Unreleased

### Added

- sqlx codegen now emits a warning when a method's SQL template interpolates
  a method argument as raw SQL text outside bind/bindvars. This surfaces a
  historical foot-gun (#17) without breaking existing schemas.
