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

	defc "github.com/x5iu/defc/runtime"

	_ "github.com/mattn/go-sqlite3"
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

	// Test constbind: GetUserByName
	userByName, err := executor.GetUserByName(ctx, "defc_test_0001")
	if err != nil {
		log.Fatalln(err)
	}
	if userByName.id != 1 || userByName.name != "defc_test_0001" {
		log.Fatalf("unexpected user from GetUserByName: User(id=%d, name=%q)\n",
			userByName.id,
			userByName.name)
	}

	// Test constbind: QueryUsersByNameAndID
	usersByPattern, err := executor.QueryUsersByNameAndID(ctx, "defc_test_000%", 2)
	if err != nil {
		log.Fatalln(err)
	}
	if len(usersByPattern) != 3 {
		log.Fatalf("expected 3 users from QueryUsersByNameAndID, got %d\n", len(usersByPattern))
	}
	// Should get users with id > 2: defc_test_0003, defc_test_0004, defc_test_0005
	expectedIDs := []int64{3, 4, 5}
	for i, u := range usersByPattern {
		if u.id != expectedIDs[i] {
			log.Fatalf("unexpected user id at index %d: expected %d, got %d\n", i, expectedIDs[i], u.id)
		}
	}

	// Test constbind: UpdateUserName
	_, err = executor.UpdateUserName(ctx, 1, "defc_test_updated")
	if err != nil {
		log.Fatalln(err)
	}
	updatedUser, err := executor.GetUserByName(ctx, "defc_test_updated")
	if err != nil {
		log.Fatalln(err)
	}
	if updatedUser.id != 1 || updatedUser.name != "defc_test_updated" {
		log.Fatalf("unexpected updated user: User(id=%d, name=%q)\n",
			updatedUser.id,
			updatedUser.name)
	}

	log.Println("All constbind tests passed!")

	// Test that panic in Scan propagates correctly
	// When a struct field's Scan method panics, the panic should propagate
	// to the caller without causing deadlock.
	func() {
		// Create a separate database connection for this test
		panicDB := defc.MustOpen("sqlite3", ":memory:")
		panicDB.Exec("CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT)")
		panicDB.Exec("INSERT INTO user (id, name) VALUES (1, 'test')")
		panicCore := &sqlc{panicDB}
		panicExecutor := NewExecutorFromCore(panicCore)

		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r == nil {
					log.Fatalln("Expected panic but got none")
				} else {
					log.Printf("Expected panic occurred: %v\n", r)
				}
			}()
			// This should panic, not return an error
			_, _ = panicExecutor.GetPanicUser(ctx, 1)
		}()
		select {
		case <-done:
			// Test completed successfully (panic was caught in goroutine)
		case <-time.After(30 * time.Second):
			log.Fatalln("Test timeout: panic test exceeded 30 seconds")
		}
	}()

	log.Println("All tests passed!")
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

	// GetUserByName query constbind
	// /* {"name": "defc", "action": "test"} */
	// select id, name from user where name = ${name};
	GetUserByName(ctx context.Context, name string) (*User, error)

	// QueryUsersByNameAndID query constbind
	// /* {"name": "defc", "action": "test"} */
	// select id, name from user where name like ${pattern} and id > ${minID} order by id asc;
	QueryUsersByNameAndID(ctx context.Context, pattern string, minID int64) ([]*User, error)

	// UpdateUserName exec constbind
	// /* {"name": "defc", "action": "test"} */
	// update user set name = ${newName} where id = ${id};
	UpdateUserName(ctx context.Context, id int64, newName string) (sql.Result, error)

	// GetPanicUser query constbind
	// /* {"name": "defc", "action": "test"} */
	// SELECT id, name from user where id = ${id};
	GetPanicUser(ctx context.Context, id int64) (*PanicUser, error)
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

type PanicUser struct {
	ID   PanicID `db:"id"`
	Name string  `db:"name"`
}

type PanicID int64

func (pid *PanicID) Scan(src any) error {
	panic("PanicID.Scan: should panic")
}

func sqlComment(context.Context) string {
	return `/* {"name": "defc", "action": "test"} */`
}
