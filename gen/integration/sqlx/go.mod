module github.com/x5iu/defc/gen/integration/sqlx

go 1.19

require (
	github.com/mattn/go-sqlite3 v1.14.23
	github.com/x5iu/defc v0.0.0
)

require github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect

replace github.com/x5iu/defc => ../../..
