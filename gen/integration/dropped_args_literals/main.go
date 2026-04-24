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
	if _, err := db.ExecContext(ctx, `CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO user (id, name) VALUES (1, 'a');`); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := NewRepoFromCore(&sqlc{db})
	if _, err := r.Drop(ctx, "a"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := r.Drop(ctx, "b"); err != nil {
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
	// Drop exec
	// DELETE FROM user WHERE name = '{{ .name }}';
	Drop(ctx context.Context, name string) (sql.Result, error)
}
