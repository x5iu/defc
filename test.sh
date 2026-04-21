#!/bin/bash

set -e
set -x

# Pool-aliasing stopgap: no callsite may pass buf.Bytes() directly into
# NewResponseError or FromBytes; must route through DetachBytes first.
# Note: test files intentionally construct the anti-pattern to verify
# -race behaviour and are excluded below.
RG="${RG:-rg}"
if command -v "$RG" >/dev/null 2>&1; then
  if "$RG" -n --glob '!**/*_test.go' \
       -e 'NewResponseError\([^)]*\.Bytes\(\)' \
       -e 'FromBytes\([^)]*\.Bytes\(\)' \
       gen/template runtime; then
    echo "pool-alias stopgap: raw .Bytes() into retaining sink" >&2
    exit 1
  fi
fi

# Build and run the poolalias analyzer against runtime + user-code fixtures.
go install ./gen/analyzer/poolalias/cmd/poolalias
go vet -vettool="$(go env GOPATH)/bin/poolalias" ./runtime ./gen/integration

go test -cover ./gen
go test -cover ./runtime
go test -cover ./sqlx
go test -tags=test ./gen/integration