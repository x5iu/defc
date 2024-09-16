#!/bin/bash

go test -cover ./gen
go test -cover ./runtime

go test -tags=test ./gen/integration