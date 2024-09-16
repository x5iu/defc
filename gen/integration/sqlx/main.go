//go:build test
// +build test

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/x5iu/defc/sqlx"

	_ "github.com/mattn/go-sqlite3"
)

var executor Executor

func init() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("[defc] ")
}

func main() {
	ctx := context.Background()
	db := sqlx.MustOpen("sqlite3", ":memory:")
	defer db.Close()
	executor = NewExecutorFromCore(&sqlc{db})
	defer executor.(io.Closer).Close()
	if err := executor.InitTable(ctx); err != nil {
		log.Fatalln(err)
	}
	r, err := executor.CreateUser(ctx,
		&User{name: "defc_test_0001"},
		&User{name: "defc_test_0002"},
		&User{name: "defc_test_0003"},
		&User{name: "defc_test_0004"},
	)
	if err != nil {
		log.Fatalln(err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		log.Fatalln(err)
	}
	user, err := executor.GetUserByID(ctx, id)
	if err != nil {
		log.Fatalln(err)
	}
	if !(user.id == id && user.name == fmt.Sprintf("defc_test_%04d", id)) {
		log.Fatalf("unexpected user: User(id=%d, name=%q)\n",
			user.id,
			user.name)
	}
	users, err := executor.QueryUsers(ctx, "defc_test_0001", "defc_test_0004")
	if err != nil {
		log.Fatalln(err)
	}
	if len(users) != 2 || users[0].id != 1 || users[1].id != 4 {
		var msg strings.Builder
		msg.WriteString("unexpected users: [")
		for i, unexpected := range users {
			if i > 0 {
				msg.WriteString(", ")
			}
			fmt.Fprintf(&msg, "User(id=%d, name=%q)",
				unexpected.id,
				unexpected.name)
		}
		msg.WriteString("]")
		log.Fatalln(msg.String())
	}
}

type sqlc struct {
	*sqlx.DB
}

func (c *sqlc) Log(
	ctx context.Context,
	name string,
	query string,
	args any,
	elapse time.Duration,
) {
	argsjson, _ := json.Marshal(args)
	fmt.Printf("=== %s\n query: %s \n  args: %v \nelapse: %s\n",
		name,
		query,
		string(argsjson),
		elapse,
	)
}

//go:generate defc generate -T Executor --features sqlx/future,sqlx/log
type Executor interface {
	// InitTable exec const
	/*
		create table if not exists user
		(
			id   integer not null
				constraint users_pk
					primary key autoincrement,
			name text not null
		);
	*/
	InitTable(ctx context.Context) error

	// CreateUser exec bind
	/*
		insert into user ( name ) values
		{{ range $index, $user := .users }}
			{{ if $index }},{{ end }}
			( {{ bind $user.Name }} )
		{{ end }}
		;
	*/
	CreateUser(ctx context.Context, users ...*User) (sql.Result, error)

	// GetUserByID query named const
	// select id, name from user where id = :id;
	GetUserByID(ctx context.Context, id int64) (*User, error)

	// QueryUsers query named const
	// select id, name from user where name in (:names);
	QueryUsers(ctx context.Context, names ...string) ([]*User, error)
}

type User struct {
	id   int64
	name string
}

func (user *User) ID() int64    { return user.id }
func (user *User) Name() string { return user.name }

func (user *User) FromRow(row sqlx.IRow) error {
	const (
		FieldID   = "id"
		FieldName = "name"
	)
	columns, err := row.Columns()
	if err != nil {
		return err
	}
	scanner := make([]any, 0, 2)
	for _, column := range columns {
		switch column {
		case FieldID:
			scanner = append(scanner, &user.id)
		case FieldName:
			scanner = append(scanner, &user.name)
		default:
			scanner = append(scanner, new(sql.RawBytes))
		}
	}
	if err = row.Scan(scanner...); err != nil {
		return err
	}
	return nil
}
