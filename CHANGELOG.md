# Changelog

## Unreleased

### 🔒 Security — generate-time RCE via sqlx #SCRIPT / arbitrary read via #INCLUDE

sqlx `#INCLUDE` anchors to the schema dir / `--include-root`, rejects
symlinks and `..` escapes, enforces 1 MiB per-file and 4 MiB
aggregate caps. `#SCRIPT` is disabled by default; opt in with
`--allow-script` (plus `--script-timeout`, `--script-env`) and it
runs with a scrubbed env, bounded timeout, and stderr cap.
`#SCRIPT` is deprecated and slated for removal in v1.47.0.
