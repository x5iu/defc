//go:build test
// +build test

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	defc "github.com/x5iu/defc/runtime"
)

var executor Executor

func init() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("[defc] ")
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	db := defc.MustOpen("sqlite3", ":memory:")
	defer db.Close()
	executor = NewExecutorFromCore(&sqlc{db})
	defer executor.(io.Closer).Close()
	if err := executor.InitTable(ctx); err != nil {
		log.Fatalln(err)
	}
	var id int64
	err := executor.WithTx(func(tx Executor) error {
		r, errTx := tx.CreateUser(ctx,
			&User{name: "defc_test_0001"},
			&User{name: "defc_test_0002"},
		)
		if errTx != nil {
			return errTx
		}
		r, errTx = tx.CreateUser(ctx,
			&User{name: "defc_test_0003"},
			&User{name: "defc_test_0004"},
		)
		if errTx != nil {
			return errTx
		}
		r, errTx = tx.CreateUser(ctx, &User{
			name: "defc_test_0005",
			projects: []*Project{
				{name: "defc_test_0005_project_01"},
				{name: "defc_test_0005_project_02"},
			},
		})
		if errTx != nil {
			return errTx
		}
		if id, errTx = r.LastInsertId(); errTx != nil {
			return errTx
		}
		return nil
	})
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
	if len(user.projects) != 2 {
		log.Fatalf("unexpected projects: %v\n", user.projects)
	}
	if !reflect.DeepEqual(user.projects, []*Project{
		{id: 1, name: "defc_test_0005_project_01", userID: 5},
		{id: 2, name: "defc_test_0005_project_02", userID: 5},
	}) {
		log.Fatalf("unexpected projects: %v\n", user.projects)
	}
	users, err := executor.QueryUsers("defc_test_0001", "defc_test_0004")
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
	userIDs, err := executor.QueryUserIDs("defc_test_0001", "defc_test_0004")
	if err != nil {
		log.Fatalln(err)
	}
	if !reflect.DeepEqual(userIDs, UserIDs{{1}, {4}}) {
		log.Fatalf("unexpected userIDs: %v\n", userIDs)
	}
}

type sqlc struct {
	*defc.DB
}

func (c *sqlc) Log(
	_ context.Context,
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
	if !strings.HasPrefix(strings.TrimSpace(query), `/* {"name": "defc", "action": "test"} */`) {
		log.Fatalf("%q query not starts with sqlcomment header\n", name)
	}
}

var cmTemplate = `{{ define "sqlcomment" }}{{ sqlcomment . }}{{ end }}`

//go:generate defc generate -T Executor -o executor.gen.go --features sqlx/future,sqlx/log,sqlx/callback --template :cmTemplate --function sqlcomment=sqlComment
type Executor interface {
	// WithTx isolation=7
	WithTx(func(Executor) error) error

	// InitTable exec
	/*
		{{ template "sqlcomment" .ctx }}
		create table if not exists user
		(
			id   integer not null
					constraint user_pk
						primary key autoincrement,
			name text not null
		);
		{{ template "sqlcomment" .ctx }}
		create table if not exists project
		(
			id      integer not null
						constraint project_pk
							primary key autoincrement,
			name    text not null,
			user_id integer not null
		);
	*/
	InitTable(ctx context.Context) error

	// CreateUser exec bind isolation=sql.LevelLinearizable
	/*
		{{ $context := .ctx }}
		{{ range $index, $user := .users }}
			{{ if $user.Projects }}
				{{ template "sqlcomment" $context }}
				insert into project ( name, user_id ) values
				{{ range $index, $project := $user.Projects }}
					{{ if gt $index 0 }},{{ end }}
					(
							{{ bind $project.Name }},
							0
					)
				{{ end }}
				;
			{{ end }}
			{{ template "sqlcomment" $context }}
			insert into user ( name ) values ( {{ bind $user.Name }} );
			{{ if $user.Projects }}
				{{ template "sqlcomment" $context }}
				update project set user_id = last_insert_rowid() where user_id = 0;
			{{ end }}
		{{ end }}
	*/
	CreateUser(ctx context.Context, users ...*User) (sql.Result, error)

	// GetUserByID query named
	// {{ template "sqlcomment" .ctx }}
	// select id, name from user where id = :id;
	GetUserByID(ctx context.Context, id int64) (*User, error)

	// QueryUsers query named const
	// /* {"name":: "defc", "action":: "test"} */
	// select id, name from user where name in (:names);
	QueryUsers(names ...string) ([]*User, error)

	// QueryUserIDs query many named const
	// /* {"name":: "defc", "action":: "test"} */
	// select id, name from user where name in (:names) order by id asc;
	QueryUserIDs(names ...string) (UserIDs, error)

	// GetProjectsByUserID query const
	// /* {"name": "defc", "action": "test"} */
	// select id, name, user_id from project where user_id = ? and id != 0 order by id asc;
	GetProjectsByUserID(userID int64) ([]*Project, error)
}

type UserID struct {
	UserID int64
}

type UserIDs []UserID

func (ids *UserIDs) FromRows(rows defc.Rows) error {
	if ids == nil {
		return errors.New("UserIDs.FromRows: nil pointer")
	}
	for rows.Next() {
		var id UserID
		if err := defc.ScanRow(rows, "id", &id.UserID); err != nil {
			return err
		}
		*ids = append(*ids, id)
	}
	return nil
}

type User struct {
	id       int64
	name     string
	projects []*Project
}

func (user *User) Name() string         { return user.name }
func (user *User) Projects() []*Project { return user.projects }

func (user *User) Callback(ctx context.Context, e Executor) (err error) {
	user.projects, err = e.GetProjectsByUserID(user.id)
	return err
}

func (user *User) FromRow(row defc.Row) error {
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

type Project struct {
	id     int64
	name   string
	userID int64
}

func (project *Project) Name() string { return project.name }

func (project *Project) FromRow(row defc.Row) error {
	return defc.ScanRow(row,
		"id", &project.id,
		"name", &project.name,
		"user_id", &project.userID,
	)
}

func sqlComment(context.Context) string {
	return `/* {"name": "defc", "action": "test"} */`
}
