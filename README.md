# defc

[![Go Reference](https://pkg.go.dev/badge/github.com/x5iu/defc.svg)](https://pkg.go.dev/github.com/x5iu/defc)
[![Go Report Card](https://goreportcard.com/badge/github.com/x5iu/defc)](https://goreportcard.com/report/github.com/x5iu/defc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Go code generator that automatically generates CRUD operations for databases and HTTP client code based on predefined
schemas.

## Overview

`defc` originates from the tedium of repetitively writing code for "create, read, update, delete" (CRUD) operations
and "network interface integration" in daily development work. By defining schemas, defc automatically generates
boilerplate code for database operations and HTTP requests, including parameter construction, error handling, result
mapping, and logging logic.

The name "defc" is a combination of "def" (define) and "c" (CLI - command line interface), reflecting its nature as a
command-line tool for defining and generating code schemas. Currently, defc provides two main code generation scenarios:

- **Database CRUD**: Code generation based on an enhanced fork of [sqlx](https://github.com/jmoiron/sqlx) for database
  operations
- **HTTP Client**: Request code generation based on Go's standard `net/http` package

## Features

- 🚀 **Automatic Code Generation**: Generate database CRUD and HTTP client code from interface definitions
- 📊 **SQL Query Support**: Full support for complex SQL queries with template functionality
- 🌐 **HTTP Client Generation**: Generate HTTP client code with request/response handling
- 🔧 **Template Engine**: Built-in template support for dynamic SQL and HTTP requests
- 🎯 **Transaction Support**: Automatic transaction handling for database operations
- 📝 **Logging Integration**: Built-in logging support for queries and requests
- 🔄 **Pagination Support**: Automatic pagination handling for HTTP APIs
- 🎨 **Flexible Configuration**: Multiple configuration options and feature flags
- 📁 **File Inclusion**: Support for including external SQL files and scripts
- 🔍 **Type Safety**: Generate type-safe code with proper error handling

> **✨ Enhanced Features**: Starting from v1.37.0, `sqlx/future` and `api/future` features are enabled by default,
> providing enhanced capabilities and improved interfaces. Use the `legacy` build tag to disable these features if needed.

## Installation

```bash
go install github.com/x5iu/defc@latest
```

Or use with `go run`:

```bash
go run -mod=mod github.com/x5iu/defc@latest --help
```

## Quick Start

### Database CRUD (sqlx mode)

1. Define your schema interface:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
// CreateUser EXEC
// INSERT INTO `user` (`name`, `age`) VALUES (?, ?);
CreateUser(ctx context.Context, user *User) error

// GetUser QUERY
// SELECT * FROM `user` WHERE `id` = ?;
GetUser(ctx context.Context, id int64) (User, error)

// GetUsers QUERY
// SELECT * FROM `user` WHERE `id` IN ({{ bindvars $.ids }});
GetUsers(ctx context.Context, ids []int64) ([]*User, error)
}
```

2. Run code generation:

```bash
go generate
```

3. Use the generated code:

```go
// Option 1: Create with driver name and DSN
query := NewQuery("mysql", "connection_string")
user, err := query.GetUser(context.Background(), 1)

// Option 2: Create from existing *sqlx.DB
db, _ := sqlx.Open("mysql", "connection_string")
query := NewQueryFromCore(db)
user, err := query.GetUser(context.Background(), 1)
```

### HTTP Client (api mode)

1. Define your API schema:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
Options() *Config
ResponseHandler() *Response

// CreateUser POST {{ $.Service.Host }}/user
// Content-Type: application/json
//
// { 
//   "name": {{ $.name }}, 
//   "age": {{ $.age }} 
// }
CreateUser(ctx context.Context, name string, age int) (*User, error)

// GetUsers GET {{ $.Service.Host }}/users?page={{ page }}
GetUsers(ctx context.Context) ([]*User, error)
}
```

2. Run code generation:

```bash
go generate
```

3. Use the generated code:

```go
config := &Config{
Host:   "https://api.example.com",
APIKey: "your-api-key",
}
service := NewService(config)
user, err := service.CreateUser(context.Background(), "John", 25)
```

## Documentation

### Quick Usage

```bash
# Simplest usage - auto-detect everything
defc generate schema.go

# With custom output file
defc generate --output=query.go schema.go

# With specific features
defc generate --features=sqlx/log,sqlx/rebind schema.go
```

**Smart Defaults:** The `defc generate` command provides intelligent defaults:

- **Auto-detect mode** by analyzing your interface methods
- **Auto-generate output filename** as `<source-file>.gen.go` when `--output` is not specified
- **sqlx mode** is detected when methods contain `EXEC`/`QUERY` operations, or when `WithTx` method is present
- **api mode** is detected when methods contain HTTP method names (GET, POST, etc.), or when `Options()`/
  `ResponseHandler()` methods are present

### Available Modes

- `sqlx`: Generate database CRUD operations using sqlx
- `api`: Generate HTTP client code

### Command Line Options

#### Global Flags

| Flag                    | Short | Type     | Description                                                                  |
|-------------------------|-------|----------|------------------------------------------------------------------------------|
| `--mode`                | `-m`  | string   | Generation mode: `sqlx` or `api` (auto-detected in `generate` command)       |
| `--output`              | `-o`  | string   | Output file name (auto-generated as `<source>.gen.go` in `generate` command) |
| `--features`            | `-f`  | []string | Enable specific features (see Features section above)                        |
| `--import`              |       | []string | Additional import packages                                                   |
| `--func` / `--function` |       | []string | Additional template functions (format: `name=function`)                      |
| `--disable-auto-import` |       | bool     | Disable automatic import detection                                           |
| `--help`                | `-h`  |          | Show help information                                                        |
| `--version`             | `-v`  |          | Show version information                                                     |

#### Generate Command Flags

| Flag         | Short | Type   | Description                                                      |
|--------------|-------|--------|------------------------------------------------------------------|
| `--type`     | `-T`  | string | Specify the target interface type when multiple candidates exist |
| `--template` | `-t`  | string | Additional template content (sqlx mode only, experimental)       |

#### Usage Examples

```bash
# Basic usage with explicit mode
defc --mode=sqlx --output=query.go

# Using generate command (auto-detect mode and output)
defc generate schema.go

# With features and custom imports
defc generate --features=sqlx/log,sqlx/rebind --import="fmt" schema.go

# Specify target type when multiple interfaces exist
defc generate --type=UserQuery user_schema.go

# Custom template (experimental, sqlx only)
defc generate --template="SELECT * FROM {{ .table }}" --type=MyQuery schema.go
```

### Features

#### sqlx Mode Features

**Note:** defc uses an enhanced fork of sqlx that provides additional interfaces and optimizations. Since v1.37.0,
`sqlx/future` is enabled by default. To use legacy behavior, build with the `legacy` tag: `go build -tags=legacy`.

- `sqlx/log`: Enable query logging
- `sqlx/rebind`: Automatic parameter placeholder rebinding for different databases
- `sqlx/in`: Enhanced IN query support
- `sqlx/future`: Use enhanced sqlx fork (`github.com/x5iu/defc/sqlx`) with additional interfaces and improvements *(
  enabled by default since v1.37.0)*
- `sqlx/callback`: Support for callback methods automatically executed after query completion
- `sqlx/any-callback`: Support for callback methods with flexible executor interface
- `sqlx/nort`: Generate code without runtime dependencies

#### api Mode Features

**Note:** Since v1.37.0, `api/future` is enabled by default, providing enhanced response handling capabilities. To use
legacy behavior, build with the `legacy` tag: `go build -tags=legacy`.

- `api/log`: Enable request logging
- `api/logx`: Enhanced logging with request/response details
- `api/client`: Custom HTTP client support
- `api/cache`: Response caching functionality
- `api/page`: Automatic pagination support
- `api/error`: Enhanced error handling with HTTP status codes
- `api/future`: Use enhanced response handling with `FromResponse()` method *(enabled by default since v1.37.0)*
- `api/nort`: Generate code without runtime dependencies

### Schema Definition

#### sqlx Schema Format

```go
type Query interface {
// MethodName OPERATION [ARGUMENTS]
// SQL_STATEMENT
MethodName(ctx context.Context, params...) (result, error)
}
```

**Operations:**

- `EXEC`: For INSERT, UPDATE, DELETE operations
- `QUERY`: For SELECT operations

**Arguments:**

- `NAMED`: Use named parameters (`:param`)
- `MANY`: Use `sqlx.Select` for multiple results
- `ONE`: Use `sqlx.Get` for single result
- `CONST`: Disable template processing for better performance
- `BIND`: Use binding mode for parameters
- `SCAN(expr)`: Custom scan target
- `WRAP=func`: Wrap the query with a custom function
- `ISOLATION=level`: Set transaction isolation level
- `ARGUMENTS=var`: Use custom arguments variable

#### api Schema Format

```go
type Service interface {
Options() *Config
ResponseHandler() *Response

// MethodName HTTP_METHOD [ARGUMENTS] URL
// HEADERS
//
// BODY
MethodName(ctx context.Context, params...) (result, error)
}
```

#### ResponseHandler Interface

The `ResponseHandler()` method must return a type that implements the Response interface with the following methods:

- **`Err() error`**: Checks if the response contains an error (e.g., non-200 status codes)
- **`ScanValues(...any) error`**: Scans response data into target objects (similar to `sql.Rows.Scan`)
- **`FromBytes(method string, data []byte) error`**: Processes response from raw bytes (traditional approach)
- **`FromResponse(method string, resp *http.Response) error`**: Processes response from HTTP response object (enabled by
  default since v1.37.0)
- **`Break() bool`**: Controls pagination flow - return `true` to stop pagination, `false` to continue

**HTTP Methods:** GET, POST, PUT, DELETE, PATCH, etc.

**Arguments:**

- `MANY`: For paginated results (returns slice)
- `ONE`: For single result (returns single value)
- `Scan(expr)`: Custom scan parameters for response processing
- `Options(expr)`: Custom request options
- `Retry=N`: Set maximum retry attempts (default: 2)

## Advanced Template Features

### SQL File Inclusion

The `sqlx` mode supports including external SQL files and script output using special directives:

#### #INCLUDE Directive

Include external SQL files or use glob patterns:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=user_query.go
type UserQuery interface {
// GetUser QUERY ONE
// #INCLUDE "queries/get_user.sql"
GetUser(ctx context.Context, id int64) (*User, error)

// GetActiveUsers QUERY MANY
// #INCLUDE "queries/*.sql"  // Include all SQL files
GetActiveUsers(ctx context.Context) ([]*User, error)
}
```

#### #SCRIPT Directive

Execute shell commands and include their output:

```go
type UserQuery interface {
// ListUsers QUERY MANY
// #SCRIPT cat "queries/list_users.sql"
ListUsers(ctx context.Context) ([]*User, error)

// GetUserCount QUERY ONE
// #SCRIPT echo "SELECT COUNT(*) as count FROM users"
GetUserCount(ctx context.Context) (int64, error)
}
```

### Template Debugging

#### Syntax Validation

defc automatically validates template syntax and provides detailed error messages:

```go
// Invalid template will show clear error
type UserQuery interface {
// GetUser QUERY ONE
// SELECT * FROM users WHERE id = {{ .invalid_function }};
GetUser(ctx context.Context, id int64) (*User, error)
}
```

Error output:

```
Error: template: defc(sqlx):1:45: executing "GetUser" at <.invalid_function>: 
can't evaluate field invalid_function in type map[string]interface {}
```

### Advanced Template Patterns

#### Conditional Rendering

```go
type UserQuery interface {
// GetUsers QUERY MANY
// SELECT * FROM users 
// WHERE 1=1
// {{if $.name}} AND name = {{ bind $.name }}{{end}}
// {{if $.age}} AND age >= {{ bind $.age }}{{end}}
GetUsers(ctx context.Context, name string, age int) ([]*User, error)
}
```

#### Loop Processing

```go
type UserQuery interface {
// GetUsersByIDs QUERY MANY
// SELECT * FROM users WHERE id IN ({{ bindvars $.ids }})
GetUsersByIDs(ctx context.Context, ids []int64) ([]*User, error)

// BulkInsertUsers EXEC
// INSERT INTO users (name, email) VALUES 
// {{range $i, $user := $.users}}
//   {{if $i}},{{end}}({{ bind $user.Name }}, {{ bind $user.Email }})
// {{end}}
BulkInsertUsers(ctx context.Context, users []*User) error
}
```

### Additional Template Content

The `--template` parameter (sqlx mode only, experimental) allows you to define additional template content that can be
shared across all methods in your interface.

#### Usage Formats

**Format 1: Template File Path**

```bash
defc generate --template path/to/template.tmpl schema.go
```

**Format 2: Direct Template Expression (with `:` prefix)**

```bash
# Using string literal (requires quotes)
defc generate --template ':"{{ define \"common\" }}SELECT * FROM {{ .table }}{{ end }}"' schema.go

# Using variable reference (no quotes needed)
defc generate --template ':templateVar' schema.go
```

#### Template File Example

Create a template file `templates/common.tmpl`:

```go
{{ define "audit_header" }}
/* Generated by defc - Method: {{ .method }} */
{{ end }}

{{ define "pagination" }}
LIMIT {{ .limit }} OFFSET {{ .offset }}
{{ end }}

{{ define "where_active" }}
WHERE active = 1 AND deleted_at IS NULL
{{ end }}
```

Use in your schema:

```go
//go:generate defc generate --template templates/common.tmpl --features sqlx/log -o user.gen.go
type UserQuery interface {
// GetUsers QUERY MANY
// {{ template "audit_header" . }}
// SELECT * FROM users 
// {{ template "where_active" }}
// ORDER BY created_at DESC
// {{ template "pagination" . }}
GetUsers(ctx context.Context, limit, offset int) ([]*User, error)

// GetActiveUserCount QUERY ONE
// {{ template "audit_header" . }}
// SELECT COUNT(*) FROM users {{ template "where_active" }}
GetActiveUserCount(ctx context.Context) (int64, error)
}
```

#### Direct Template Expression Example

```go
//go:generate defc generate --template ':"{{ define \"timestamp\" }}/* Generated at {{ .now }} */{{ end }}"' --function now=time.Now -o user.gen.go
type UserQuery interface {
// GetUser QUERY ONE
// {{ template "timestamp" . }}
// SELECT * FROM users WHERE id = ?;
GetUser(ctx context.Context, id int64) (*User, error)
}
```

#### Advanced Template with Bind Mode

When using `bind` function in templates, the system automatically enables `BIND` mode for all methods:

```go
//go:generate defc generate --template ':"{{ define \"bulk_insert\" }}INSERT INTO {{ .table }} ({{ .fields }}) VALUES {{ range $i, $item := .items }}{{ if $i }},{{ end }}({{ range $j, $field := .fields }}{{ if $j }},{{ end }}{{ bind (index $item $field) }}{{ end }}){{ end }}{{ end }}"' -o bulk.gen.go
type BulkQuery interface {
// BulkInsertUsers EXEC
// {{ template "bulk_insert" . }}
BulkInsertUsers(ctx context.Context, users []*User) error
}
```

#### Template with Custom Functions

```bash
defc generate --template ':"{{ define \"custom_log\" }}/* {{ logLevel . }} */{{ end }}"' --function logLevel=getLogLevel schema.go
```

#### Important Notes

- **sqlx mode only**: The `--template` parameter only works in sqlx mode, not in api mode
- **Experimental feature**: This is an experimental parameter that may change in future versions
- **Shared scope**: Templates are shared across all methods in the interface
- **Auto-bind detection**: If templates use the `bind` function, BIND mode is automatically enabled for all methods
- **Performance consideration**: Using `bind` in templates adds runtime overhead as templates are parsed on each call

## Examples

### Database Examples

#### Basic CRUD Operations

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=user_query.go
type UserQuery interface {
// CreateUser EXEC
// INSERT INTO users (name, email, age) VALUES (?, ?, ?);
CreateUser(ctx context.Context, name, email string, age int) error

// GetUserByID QUERY ONE
// SELECT * FROM users WHERE id = ?;
GetUserByID(ctx context.Context, id int64) (*User, error)

// GetUsersByAge QUERY MANY
// SELECT * FROM users WHERE age >= ? ORDER BY name;
GetUsersByAge(ctx context.Context, minAge int) ([]*User, error)

// UpdateUser EXEC
// UPDATE users SET name = ?, email = ? WHERE id = ?;
UpdateUser(ctx context.Context, name, email string, id int64) error

// DeleteUser EXEC
// DELETE FROM users WHERE id = ?;
DeleteUser(ctx context.Context, id int64) error
}
```

#### Named Parameters

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=user_query.go
type UserQuery interface {
// FindUsers QUERY NAMED
// SELECT * FROM users WHERE name = :name AND age >= :min_age;
FindUsers(ctx context.Context, name string, minAge int) ([]*User, error)
}
```

#### Transaction Support

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=user_query.go
type UserQuery interface {
WithTx(ctx context.Context, fn func (UserQuery) error) error

// CreateUser EXEC
// INSERT INTO users (name, email) VALUES (?, ?);
CreateUser(ctx context.Context, name, email string) error

// CreateProfile EXEC  
// INSERT INTO profiles (user_id, bio) VALUES (?, ?);
CreateProfile(ctx context.Context, userID int64, bio string) error
}

// Usage
err := query.WithTx(ctx, func(q UserQuery) error {
if err := q.CreateUser(ctx, "John", "john@example.com"); err != nil {
return err
}
return q.CreateProfile(ctx, userID, "Software Developer")
})
```

#### Callback Support

The `sqlx/callback` and `sqlx/any-callback` features allow structs to implement callback methods that are automatically
executed after query completion:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --features=sqlx/callback --output=user_query.go
type UserQuery interface {
// GetUser QUERY
// SELECT id, name FROM users WHERE id = ?;
GetUser(ctx context.Context, id int64) (*User, error)

// GetUsers QUERY
// SELECT id, name FROM users WHERE active = 1;
GetUsers(ctx context.Context) ([]*User, error)
}

type User struct {
ID       int64     `db:"id"`
Name     string    `db:"name"`
Projects []*Project // Will be populated by callback
}

// Callback method for sqlx/callback feature
func (u *User) Callback(ctx context.Context, query UserQuery) error {
// Automatically called after User is populated from query
projects, err := query.GetProjectsByUserID(ctx, u.ID)
if err != nil {
return err
}
u.Projects = projects
return nil
}

// For sqlx/any-callback feature (more flexible)
func (u *User) Callback(ctx context.Context, executor any) error {
// executor can be any type, providing more flexibility
if q, ok := executor.(UserQuery); ok {
return u.loadProjects(ctx, q)
}
return nil
}
```

**Callback Features:**

- `sqlx/callback`: Expects `Callback(context.Context, SpecificInterface) error`
- `sqlx/any-callback`: Expects `Callback(context.Context, any) error` for more flexibility

#### Logging Support

The `sqlx/log` feature allows you to log SQL queries and their execution details:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --features=sqlx/log --output=user_query.go
type UserQuery interface {
// GetUser QUERY
// SELECT id, name FROM users WHERE id = ?;
GetUser(ctx context.Context, id int64) (*User, error)
}

// Your database connection must implement the Log interface
type LoggingDB struct {
*sqlx.DB
}

func (db *LoggingDB) Log(
ctx context.Context,
caller string, // Method name (e.g., "GetUser")
query string,         // SQL query
args any,             // Query arguments
elapse time.Duration, // Execution time
) {
argsJSON, _ := json.Marshal(args)
log.Printf("=== %s\nquery: %s\nargs: %s\nelapse: %s\n",
caller, query, string(argsJSON), elapse)
}

// Usage
db := &LoggingDB{DB: sqlx.MustOpen("postgres", dsn)}
query := NewUserQueryFromCore(db)
```

#### Transaction with Isolation Level

The `WithTx` method supports setting transaction isolation levels using the `ISOLATION` argument:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=user_query.go
type UserQuery interface {
// WithTx ISOLATION=sql.LevelSerializable
WithTx(ctx context.Context, fn func (UserQuery) error) error

// CreateUser EXEC
// INSERT INTO users (name, email) VALUES (?, ?);
CreateUser(ctx context.Context, name, email string) error
}

// Usage with serializable isolation level
err := query.WithTx(ctx, func(q UserQuery) error {
return q.CreateUser(ctx, "John", "john@example.com")
})
```

### HTTP Client Examples

#### Basic API Client

```go
type Config struct {
Host   string
APIKey string
}

type Response struct {
Data   any `json:"data"`
Error  string      `json:"error"`
Status int         `json:"status"`
}

func (r *Response) Err() error {
// Check for API-level errors
if r.Error != "" {
return fmt.Errorf("API error: %s", r.Error)
}
// Check for HTTP status errors
if r.Status >= 400 {
return fmt.Errorf("HTTP error: status %d", r.Status)
}
return nil
}

func (r *Response) ScanValues(dest ...any) error {
// Scan response data into provided destinations
for _, d := range dest {
if err := json.Unmarshal([]byte(r.Data.(string)), d); err != nil {
return fmt.Errorf("failed to scan response data: %w", err)
}
}
return nil
}

func (r *Response) FromBytes(method string, data []byte) error {
// Process response from raw bytes
return json.Unmarshal(data, r)
}

func (r *Response) FromResponse(method string, resp *http.Response) error {
// Process response from HTTP response object (default since v1.37.0)
defer resp.Body.Close()
data, err := io.ReadAll(resp.Body)
if err != nil {
return fmt.Errorf("failed to read response body: %w", err)
}
return json.Unmarshal(data, r)
}

func (r *Response) Break() bool {
// Control pagination: return true to stop, false to continue
// Example: stop if no more data or reached limit
return r.Data == nil || len(r.Data.([]any)) == 0
}

//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=user_service.go
type UserService interface {
Options() *Config
ResponseHandler() *Response

// CreateUser POST {{ $.Service.Host }}/api/users
// Content-Type: application/json
// Authorization: Bearer {{ $.Service.APIKey }}
//
// {
//   "name": {{ $.name }},
//   "email": {{ $.email }}
// }
CreateUser(ctx context.Context, name, email string) (*User, error)

// GetUser GET {{ $.Service.Host }}/api/users/{{ $.id }}
// Authorization: Bearer {{ $.Service.APIKey }}
GetUser(ctx context.Context, id int64) (*User, error)

// ListUsers GET MANY {{ $.Service.Host }}/api/users?page={{ page }}
// Authorization: Bearer {{ $.Service.APIKey }}
ListUsers(ctx context.Context) ([]*User, error)
}
```

#### Custom HTTP Client

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --features=api/client --output=service.go
type Service interface {
Options() *Config
ResponseHandler() *Response

// GetData GET {{ $.Service.Host }}/data
GetData(ctx context.Context) (*Data, error)
}

type Config struct {
Host   string
client *http.Client
}

func (c *Config) Client() *http.Client {
return c.client
}

// Usage
config := &Config{
Host: "https://api.example.com",
client: &http.Client{Timeout: 30 * time.Second},
}
service := NewService(config)
```

#### HTTP Request Logging

The `api/log` and `api/logx` features provide HTTP request logging capabilities:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --features=api/log --output=user_service.go
type UserService interface {
Options() *Config
ResponseHandler() *Response

// GetUser GET {{ $.Service.Host }}/users/{{ $.id }}
GetUser(ctx context.Context, id int64) (*User, error)
}

// Basic logging with api/log
type Config struct {
Host string
}

func (c *Config) Log(
ctx context.Context,
caller string, // Method name (e.g., "GetUser")
method string,        // HTTP method (e.g., "GET")
url string,           // Request URL
elapse time.Duration, // Request duration
) {
log.Printf("=== %s %s %s\nelapse: %s\n", caller, method, url, elapse)
}

// Enhanced logging with api/logx
func (c *Config) Log(
ctx context.Context,
caller string, // Method name
request *http.Request,   // Full HTTP request
response *http.Response, // Full HTTP response
elapse time.Duration, // Request duration
) {
log.Printf("=== %s %s %s\nStatus: %d\nElapse: %s\n",
caller, request.Method, request.URL.String(),
response.StatusCode, elapse)
}

// Usage
config := &Config{Host: "https://api.example.com"}
service := NewUserService(config)
```

**API Log Features:**

- `api/log`: Basic HTTP request logging with URL and timing
- `api/logx`: Enhanced HTTP logging with full request/response objects

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built on top of an enhanced fork of [sqlx](https://github.com/jmoiron/sqlx) for database operations
- Inspired by the need to reduce boilerplate code in Go applications
- Thanks to the Go community for feedback and contributions
- This README was generated with assistance from [Claude](https://claude.ai)