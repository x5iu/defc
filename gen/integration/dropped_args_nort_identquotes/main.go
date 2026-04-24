//go:build test

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
	if _, err := db.ExecContext(ctx, `CREATE TABLE u (id INTEGER PRIMARY KEY, name TEXT, email TEXT); INSERT INTO u (id, name, email) VALUES (1, 'x', 'y');`); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := NewRepoFromCore(&sqlc{db})
	if _, err := r.Pick(ctx, 1, true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := r.Pick(ctx, 1, false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("ok")
}

type sqlc struct {
	*defc.DB
}

//go:generate go run github.com/x5iu/defc --mode=sqlx -T Repo -o repo.gen.go --features sqlx/future,sqlx/nort
type Repo interface {
	// Pick exec
	// {{ if .flag }}UPDATE u SET "name" = 'a' WHERE id = {{ .id }}{{ else }}UPDATE u SET "email" = 'a' WHERE id = {{ .id }}{{ end }};
	Pick(ctx context.Context, id int, flag bool) (sql.Result, error)
}
