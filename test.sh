#!/bin/bash

go test -cover ./gen && \
go test -cover ./runtime && \
go test -cover ./sqlx && \
go test -tags=test ./gen/integration