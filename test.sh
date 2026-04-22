#!/bin/bash

set -e
set -x

go test -cover ./gen
go test -cover ./runtime
go test -gcflags=all=-d=checkptr=1 ./runtime/...
go test -cover ./sqlx
go test -tags=test ./gen/integration