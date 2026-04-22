//go:build test
// +build test

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	defc "github.com/x5iu/defc/runtime"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	ctx := context.Background()
	db := defc.MustOpen("sqlite3", ":memory:")
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, a INTEGER);
CREATE TABLE t2 (b INTEGER);
INSERT INTO t (id, a) VALUES (1, 0);
INSERT INTO t2 (b) VALUES (9);`); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := NewRepoFromCore(&sqlc{db})
	if _, err := r.DoShort(ctx, 1, 1, 9); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("ok")
}

type sqlc struct {
	*defc.DB
}

//go:generate go run github.com/x5iu/defc --mode=sqlx -T Repo -o repo.gen.go --features sqlx/future
type Repo interface {
	// DoShort exec
	// UPDATE t SET a=?; DELETE FROM t2 WHERE b=?;
	DoShort(ctx context.Context, a, id, b int) (sql.Result, error)
}
