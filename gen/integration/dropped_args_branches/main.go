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
	if _, err := db.ExecContext(ctx, `CREATE TABLE u (id INTEGER PRIMARY KEY, n TEXT); INSERT INTO u (id, n) VALUES (1, 'z');`); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := NewRepoFromCore(&sqlc{db})
	if _, err := r.Branch(ctx, "1", true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := r.Branch(ctx, "1", false); err != nil {
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
	// Branch exec
	// {{ if .flag }}DELETE FROM u WHERE id='{{ .id }}'{{ else }}UPDATE u SET n='x' WHERE id='{{ .id }}'{{ end }};
	Branch(ctx context.Context, id string, flag bool) (sql.Result, error)
}
