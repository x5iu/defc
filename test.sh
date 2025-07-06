#!/bin/bash

set -e
set -x

go test -cover ./gen
go test -cover ./runtime
go test -cover ./sqlx
go test -tags=test ./gen/integration