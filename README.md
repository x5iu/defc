# defc

Automatically generate code based on predefined Schema

## Introduction

`defc` originates from the tedium of repetitively writing code for "create, read, update, delete" (CRUD) operations and "network interface integration" in our daily work and life.

For example, for database queries, we often need to:

1. Define a new function or method;
2. Write a new SQL query;
3. Execute the query, handle errors, and map the results to a structure;
4. If there are multiple SQL statements, initiate a transaction, and perform commit or rollback;
5. Log the query;
6. ...

Similarly, for network interface integration, for a new interface, we often:

1. Define a new function or method;
2. Set the interface URL, configure parameters (such as Headers, Query, Body in HTTP requests);
3. Make the request, handle errors, and map the response to a structure;
4. If it involves pagination, concatenate the results of multiple paginated queries into the final result;
5. Log the request;
6. ...

All of the above are repeated several times when writing new requirements or scenarios. Especially the parts related to queries, requests, error handling, transaction commit/rollback, data mapping, list concatenation, and log recording, which are all logically identical repetitive codes. Writing them is very annoying; some codes are very long, and copying and pasting require various changes to variable names, method names, and configuration information, which greatly affects development efficiency;

Unfortunately, the Go language does not provide official macro features, and we cannot use macros to complete these complex repetitive codes like Rust does (of course, macros also have their limitations; they are devastating to code readability when not expanded and also affect IDE completion). However, fortunately, Go provides a workaround with `go generate`. Through `go generate`, we can approximately provide macro functionality, that is, code generation capabilities.

Based on the above background, I wanted to implement a code generation tool. By defining the **Schema** of a query or request, it is possible to automatically generate code for the related CRUD operations or HTTP requests, which includes **parameter construction, error handling, result mapping, and log recording** logic. `defc` is my experimental attempt at such a schema-to-code generation; "def" stands for "define," indicating the behavior of setting up a Schema. Currently, `defc` provides the following two scenarios of code generation features:

- CRUD code generation based on [sqlx](https://github.com/jmoiron/sqlx) for databases
- HTTP interface request code generation based on the `net/http` package in the Golang standard library

## sqlx mode

*Note: To use this mode, you need to import the sqlx package, please execute the following command in your project directory:*

```
go get github.com/jmoiron/sqlx
```

Firstly, we need to define the SQL query's Schema. A basic Schema is shown below:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // INSERT INTO `user` (`name`, `age`) VALUES (?, ?);
  CreateUser(ctx context.Context, user *User) error
  
  // GetUser QUERY
  // SELECT * FROM `user` WHERE `id` = ?;
  GetUser(ctx context.Context, id int64) (User, error)
}
```

A Schema should be an interface, with each method representing an SQL query. A Schema consists of three parts:

1. The `go generate` command;
2. Interface name definition;
3. Method definitions;

Method definitions include four parts:

1. Method name and query type definition;
2. SQL statement definition;
3. Query parameters definition;
4. Query results and error definition;

Firstly, there is the `go generate` part, which accepts a shell command. Here we use `go run -mod=mod "github.com/x5iu/defc"` to execute the code generation command. Of course, if you prefer not to use the `go run` command, or if there is no `go` compiler in the current environment, you can also use `defc` after compiling it separately. Add `defc` to the environment variable `PATH`, and then use the command `defc --mode=sqlx` instead of the previously mentioned `go run -mod=mod` command, which is also feasible. The `defc` command accepts two basic parameters: `--mode/-m` and `--output/-o`. The `--mode` command specifies the current mode for code generation, which in this case should be `--mode=sqlx`; the `--output` command specifies the location of the generated code file.

*Note: Additionally, the `defc` command also supports the `--features` parameter, which will be mentioned in the following explanations.*

Next are the interface name definition and method definitions. After defining the interface name (in this example, "Query"), `defc` will generate several constructors such as:

- `NewQuery`
- `NewQueryFrom*`

Method definitions are the most important part of the Schema, directly influencing the logic of the final generated code. We will elaborate further on this part. It is important to note that:

- the Schema's definition relies on the content of comments. Therefore, in the Schema, you may not be able to freely write what you want to express in the comments;
- ~~and please use single-line double-slash comments `//`, do not use multi-line `/* */` comments, otherwise `defc` will not be able to parse the Schema content defined in the comments;~~
- **comments should be closely attached to the method definition**;

### Method Name and Query Type Definition

This is the first line of content in the comments, always fixed as the first line, formatted as: method name, query type, and optional query parameters:

```
// <NAME> <CMD> <ARG>...
```

Here, `<NAME>` is the method name. You should keep it consistent with the current interface name (this is a convention and not strictly enforced, meaning you can use any word for `<NAME>`, but it must be present; it cannot be empty).

`<CMD>` is the definition of the current query mode. The commands currently supported (case insensitive) are:

- `EXEC`: Corresponding to the `Exec` function/method in `sqlx`
- `QUERY`: Corresponding to the `Query`/`Get` function/method in `sqlx`

`<ARG>` is an optional parameter, supporting only one value `NAMED` (also case insensitive). If this parameter is not provided, the normal querying method of `sqlx` is used, i.e., `Exec`/`Query`/`Get` function/method. If the `NAMED` parameter is provided, then the `PrepareNamed` function/method in `sqlx` is used to construct a `sqlx.NamedStmt`, and then the `Exec`/`Query`/`Get` function/method of `sqlx.NamedStmt` is called.

Starting from `v1.9.4`, `<ARG>` supports two additional values `MANY`/`ONE`, which are only available when the `<CMD>` value is `QUERY`. `MANY` represents using the `sqlx.Select` method for querying, where the result should be stored in a slice; `ONE` represents using the `sqlx.Get` method for querying, where the result should be stored in a structure, a basic type, or a type that implements the `sql.Scanner` interface. Additionally, from `v1.9.4`, the `[]byte` type will be treated as a separate type like `string`, not a slice type. When using `[]byte` as the return type without specifying `MANY`/`ONE`, `defc` will default to the `sqlx.Get` method for querying.

Starting from `v1.12.0`, `<ARG>` introduces a new parameter `CONST`. Using the `CONST` parameter indicates that you want `defc` **not** to generate template (`text/template`) construction-related code, meaning `defc` will treat the comment content as the complete SQL for database querying, and template syntax will not be effective. This can significantly improve performance (as template construction is very slow) when executing simple SQL (referring to SQL strings that do not require additional concatenation and judgment).

Starting from `v1.13.2`, `<ARG>` adds a new parameter `SCAN(expr)`, which will use `expr` instead of the return value as the parameter passed into the `sqlx.Select`/`sqlx.Get` method. When using the `SCAN` parameter, the method return value can only be `error`.

Starting from `v1.20.1`, a new argument `BIND` has been added to `<ARG>`. It will use the `binding` method to bind query parameters. For more details, please refer to the section "Query Parameter Definition".

### SQL Statement Definition

Starting from the second line of the comment, the SQL statement definition follows. Here, you can write the SQL statements you want to execute. SQL line breaks are supported (while still needing to be within the comment scope), and so are multiple SQL statements (multi-line SQL is only supported for use in `EXEC`, with multiple SQL statements separated by semicolons `;`; these SQL will be executed in the same transaction). In SQL, parameters are represented with question marks `?` as placeholders. **Additionally, if you are using `PostgreSQL` or other databases that do not use `?` as a placeholder, you can convert the `?` in SQL to the placeholder suitable for the respective database with the `--features=sqlx/rebind` feature in the `def` command. For instance, it changes to `$1` in `PostgreSQL`.**

### Query Parameter Definition

So, how to pass values to these question mark placeholders? Both `sqlx` and Go's standard library `database/sql` pass variable-length arguments to convey values to the database. `defc` also uses this approach but with some encapsulation. We'll explain these rules in detail:

First, let's describe the general case without `NAMED` parameters:

1. In method definitions, for parameters other than `ctx context.Context`, the remaining arguments are passed in their order of appearance to the underlying `Exec`/`Query` functions/methods (**note that all parameters must have names and cannot be defined with only types without parameter names**).

2. For slice (`Slice`) type parameters, when passing arguments, the slice will be expanded into several parameters based on the length of the slice, for example, parameter `[]int{1, 2, 3}` will be passed into the `Query` method like this (excluding `[]byte` type, as `[]byte` itself is a valid `driver.Value`):

   ```go
   sqlx.Query(sql, 1, 2, 3)
   ```

3. However, most of the time, the method's input parameters are not basic types or, to be more specific, not all `driver.Value` types. If they aren't `driver.Value` types, it will most likely cause a runtime error in the query. Moreover, we often use a structure to wrap some parameters to avoid lengthy method signatures. For these two scenarios, `defc` provides the `ToArgs` interface. For types implementing the `ToArgs` interface, when passing arguments, their `ToArgs` method will be called, and the returned slice of arguments will be split and merged into the query parameters. The `ToArgs` interface is defined in the `github.com/x5iu/defc/__rt` package as follows:

   ```go
   type ToArgs interface {
     ToArgs() []any
   }
   ```

4. Additionally, if you do not want a method's input parameter to serve as an SQL query parameter, you can implement the `NotAnArg` interface for this parameter's type. This interface is also located in the `github.com/x5iu/defc/__rt` package, and is defined as:

   ```go
   type NotAnArg interface {
     NotAnArg()
   }
   ```

For scenarios with `NAMED` parameters:

1. Replace the question mark `?` placeholders with `:name`, where `name` is the name of the parameter.

2. In method definitions, for parameters other than `ctx context.Context`, the remaining parameters are passed as `key:value` pairs, with `key` being the parameter name and `value` being the parameter value, to the `sqlx.PrepareNamed` function/method (**likewise, all parameters must have names and cannot be defined with only types without parameter names**).

3. **Note that the `NAMED` query method does not support slice parameters, and `defc` will not expand slice parameters in `NAMED` mode**.

4. For types defined with `ToNamedArgs`, `defc` will merge the contents of the `map` returned by `ToNamedArgs` into the query parameters. Additionally, if the parameter type is a structure or pointer to a structure, it will query the structure fields' `db` tags (as defined by `sqlx`). For fields with a `db` tag, the value of the `db` tag will be used as `key`, and the field value as `value` added to the query parameters. If the parameter type is a `map`, `defc` will merge this `map` into the query parameters. The `ToNamedArgs` interface is defined in the `github.com/x5iu/defc/__rt` package as follows:

   ```go
   type ToNamedArgs interface {
     ToNamedArgs() map[string]any
   }
   ```

5. Similarly, if you do not want a method's input parameter to serve as an SQL query parameter, you can implement the `NotAnArg` interface.

Typically, for complex SQL, we use string concatenation to build queries, but this is neither safe nor convenient, and the code becomes messy; hence, `defc` provides a default template function to construct SQL. The SQL statements in comments support all template syntax of Go's standard library `text/template`. You can access method input parameters in SQL with `{{ $.Arg }}`, where `Arg` refers to the name of the method input parameter. For example, in the method signature `GetUser(ctx context.Context, id int64)`, you can use `{{ $.id }}` to access the `id` parameter. Likewise, if these parameters have associated methods, you can invoke these methods through template syntax; the same support applies to conditional and loop statements.

*Note: Please do not use template syntax to construct SQL query parameters. Instead, use question mark `?` placeholders to pass parameters to avoid the risk of SQL injection.*

Additionally, `defc` offers a special template function `bindvars`. `bindvars` takes one parameter, which can be of any type. If it's an integer type (signed or not), it generates a corresponding number of question mark `?` placeholders, separated by commas. If it's a slice (`Slice`) type, it generates question mark `?` placeholders corresponding to the length of the slice, also separated by commas. For other types, it generates a single question mark `?`. `bindvars` is particularly useful for dealing with slice-type SQL parameters, enabling the quick generation of placeholders for query parameters. For instance:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // GetUsers QUERY
  // SELECT * FROM `user` WHERE `id` IN ({{ bindvars $.ids }});
  GetUsers(ctx context.Context, ids []int64) ([]*User, error)
}
```

Regarding the `ctx context.Context` parameter, it is optional, but we recommend including the `ctx context.Context` parameter in standard method definitions according to Go's conventions, with the parameter name designated as `ctx` (please do not use any other names, as `defc` only recognizes `ctx`).

Starting from `defc@v1.20.1`, `defc` has added a new parameter mode called `binding`. You can enable this mode by using the `BIND` argument, for example:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // GetUsers QUERY BIND
  // SELECT * FROM `user` WHERE `id` IN ({{ bind $.ids }}) AND `status` = {{ bind $.status.String }};
  GetUsers(ctx context.Context, status fmt.Stringer, ids []int64) ([]*User, error)
}
```

When using the `binding` mode, specify the values of the parameters that need to be bound using template syntax, and use the `bind` function to add the parameters to the query parameter list. For example, if you have a `user` object and you need to use `user.ID` as a query parameter, you can write in the SQL like this: `{{ bind $.user.ID }}`. `defc` will not only add `user.ID` to the query parameter list but also place a placeholder (or multiple, depending on the parameter type) at the position of `{{ bind $.user.ID }}` to prevent SQL injection attacks.

*Note that when using binding mode, since the template is built and rendered each time, its execution performance is significantly slower compared to when the `CONST` argument is enabled. Please choose the specific parameters according to your actual scenario.*

### Query Results and Error Definitions

It is important to clarify that **all methods must carry an `error` return value**. A method may not return a result, but it cannot omit returning an error. If there is no `error` return value, `defc` will throw an error during code generation. This is a strict constraint.

For `EXEC` type queries, `defc` will not call any `Scan` methods, as `EXEC` performs create, delete, and update operations, which should not return any data by nature. Therefore, usually, methods of the `EXEC` class should only return an `error`.

However, we often care about the `LastInsertId` after an `INSERT` statement or the `RowsAffected` value after an `UPDATE` statement. Thus, `defc` additionally provides support for the `sql.Result` interface as a return value for `EXEC`. For example, you can define a Schema like this:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // INSERT INTO `user` (`name`, `age`) VALUES (?, ?);
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

For `QUERY` type queries, you can use a basic type, a structure (or structure pointer), or a slice as a return value, and `defc` will automatically map the query results to the return value.

### Transaction Support

If you need to execute queries within a transaction, you can add a special method `WithTx` to the Schema, as shown in the following example:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  WithTx(context.Context, func(Query) error) error
}
```

The first parameter of the `WithTx` method is `context.Context` (also an optional parameter, but typically recommended to include), and the second parameter is a function whose argument is the interface defined by the Schema. Within this function, you can execute queries through this argument. These queries will be executed within the same transaction, which will be automatically committed. If an error occurs, it will automatically roll back.

### Logging

To record query logs, please add the `--features=sqlx/log` feature and use the `New*FromCore` constructor method. You need to implement the `Log` interface for the type of the `core` parameter passed in. The `Log` interface is defined as follows:

```go
type Log interface { 
  Log(ctx context.Context, caller string, query string, args any, elapse time.Duration) 
}
```

Here, the `caller` parameter is the name of the current method, `query` is the SQL statement of the current query, `args` are the parameters of the query, and `elapse` is the time spent on the query.

### Embedding Complex SQL

In a few cases, we write extremely complex and lengthy SQL query statements. Having these SQL statements written in comments obviously impacts the readability of the code and is not conducive to SQL maintenance. Therefore, `defc` provides the functionality to embed `.sql` files at compile time (in fact, all file types are supported, such as `.tmpl` files). You only need to use the `#include` statement to import the file (this looks similar to C language's `include`):

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // #include insert.sql
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

*Note: The `#include` statement should exist in a comment line on its own, and that line should only contain `#include` and the filename. Only one file can be included at a time, but you can use `#include` multiple times to import different files.*

Another best practice for `#include` is, if your SQL always contains some common query conditions, or query fields, you can write them in a separate `.sql` file and use `#include` to import them into different methods. In this way, you won't need to rewrite these common query statements and only need to update the `.sql` file to update all methods that imported this `.sql` file's schema.

From `v1.9.0` onwards, `defc` added the `#script` directive, which is used similarly to `#include`, but supports calling external commands to generate SQL statements. For example, you can invoke a Python script to generate the corresponding SQL at compile time like this:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // #script python gen_sql.py
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

The `#script` directive will compile the command's standard output `stdout` into the generated file as template content (this also means that the generated code is template code and supports template syntax).

Starting from `v1.11.5`, to enhance the readability and convenience of the `#script` directive, the following rules were added:

- If the current line starts with a whitespace character or `\t`, it will be considered as a continuation of the previous line. `defc` will append this line to the end of the previous line, separated by a space;
- Blank lines between lines will be discarded, for example, `\n\n` will be replaced with `\n`;

If you want to apply these rules, you can use the `/* */` type of comment, for example:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  /*
    #script
      python3
        -c "print(open('sql/sql.tmpl').read())"
  */
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

## API Mode

Under `api` mode, the definition of the Schema is largely similar to the `sqlx` mode. A basic Schema definition is as follows:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // CreateUser POST {{ $.Service.Host }}/user
  // Content-Type: application/json
  //
  // { 
  //   "name": {{ $.name }}, 
  //   "age": {{ $.age }} 
  // }
  CreateUser(ctx context.Context, name string, age int) (*User, error)
}
```

With the foundation of the `sqlx` mode, it is much easier to understand the Schema under the `api` mode. We will skip the concepts that are repetitive with `sqlx` and directly introduce the differences.

### The `Inner` and `Response` Methods

The first difference from the `sqlx` Schema is that the `api` Schema needs to define two auxiliary methods, `Inner` and `Response`.

The type returned by the `Inner` method will serve as a dependency for the Schema, and the Schema's constructor method `New*` will take the return type of the `Inner` method as an argument to complete dependency injection. Typically, information such as the host, key, secret, token, etc., used in interface calls, is placed in the return value of the `Inner` method.

*Note: From `v1.5.1` onwards, the `Response` and `Inner` methods are no longer case-sensitive, and they can also be written in the form of `response` and `inner` to avoid being mistakenly used by the calling party. In most cases, it is recommended to write them in lowercase to prevent exposing these two internally used methods to the caller.*

The type returned by the `Response` method will be used to construct the return value of the HTTP request. The return value of the `Response` method must implement the following interface:

```go
type Response interface {
   Err() error
   ScanValues(...any) error
   FromBytes(string, []byte) error
   Break() bool
}
```

The `Err` method represents errors related to the business layer rather than the network layer that occur during this request. For example, if we request an interface and the network request is successful with a status value of 200, but a business error occurs, such as a request parameter error resulting in a 400 business response code, then after obtaining the business response information, we should expose the error to the caller through the `Err` method.

The `ScanValues` method is used to map the response results into the method's return values. Note that the parameters here are variadic, meaning the method's return values can include more than one parameter (but this does not include the `error` return value). In our example, the return value is `*User`. This return value will first have memory allocated and then call the `ScanValues` method to write the response information into `*User`. If an error is encountered, it will return the error.

The `FromBytes` method is used to construct this `Response`. The response of the HTTP request is of type `[]byte`. The complete response value will be passed into the `FromBytes` method, and the `Response` type needs to handle the deserialization of the response itself and return an error if encountered. The first parameter of `FromBytes` is the current method name, suitable for performing different deserialization procedures for different interfaces.

The `Break` method is only applicable to pagination queries, i.e., when the method's return value is of a slice type. Pagination queries will use the `Break` method's return value as the criterion for whether the query objective has been reached. When the `Break` method returns a true value, the pagination query ends, returning the constructed slice result. Typically, for pagination queries, it is recommended that the `Response` store the current query progress, such as the number of elements queried and stored, and compare it with the desired number (which can be obtained from the response), and use the comparison result as the return value of the `Break` method. *Starting from `v1.10.0`, you can implement a `SetTotalCount(int)` method for `Response`, and `defc` will automatically call the `SetTotalCount` method to pass in the **cumulative** number of current pagination query results, so that `Response` can know more clearly when to end the pagination query and call the `Break` method.*

### Request Method, URL, Header, and Body

The schema definition in the `api` method is similar to that of `sqlx`. Definitions are written in comments in the following format:

```
// <NAME> <METHOD> <URL>
// <HEADER>
//
// <BODY>
```

Where `<NAME>` is the method name; `<METHOD>` is the HTTP request method (such as `GET`/`POST`); `<URL>` is the request URL; `<HEADER>` includes the request headers, supporting multiple lines; `<BODY>` is the request body. It is important to note, similar to an HTTP message, there must be a `\r\n` separator (i.e., an empty line) between `<BODY>` and `<HEADER>`.

The parts `<URL>`/`<HEADER>`/`<BODY>` also support template syntax. Different from the templates in `sqlx`, in `api` the return value of the `Inner` method is also passed as a template parameter, which can be accessed with `{{ $.Schema }}`, where `Schema` is the name of the defined schema. In this case, it is `Service`, which means you can use `{{ $.Service }}` to access the return value of the `Inner` method. Additionally, for pagination query interfaces, if you use the `--features=api/page` parameter, `api` will provide an additional `page` function (note that it is a function, not a template parameter). You can use `{{ page }}` to access the current page count, which starts from `0`. Each call to the `{{ page }}` function will increment the page count. For example, a paginated query method could be defined as:

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // GetUsers GET {{ $.Service.Host }}/users?name={{ $.name }}&page={{ page }}
  GetUsers(ctx context.Context, name string) ([]*User, error)
}
```

If you do not want to construct the HTTP request body via templates in the comments, `api` also provides another way to pass the body. Specifically, if you do not fill in the body content in the comments (headers can still be included), `defc` will treat the last parameter of the method as the HTTP request body. Note that if this method is used, the last parameter must be of type `io.Reader`. Here is an example (rewritten from the previous one):

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // CreateUser POST {{ $.Service.Host }}/user
  // Content-Type: application/json
  CreateUser(ctx context.Context, name string, age int, body io.Reader) (*User, error)
}
```

Starting from `v1.9.6`, you can add an optional parameter `MANY` between `<MEHOTD>` and `<URL>` (similar to `MANY` in `sqlx` mode), to indicate to `defc` that the method returns a slice type, which will allow `defc` to generate pagination code for the method:

```go
type Users []*User

//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // GetUsers GET MANY {{ $.Service.Host }}/users?name={{ $.name }}&page={{ page }}
  GetUsers(ctx context.Context, name string) (Users, error)
}
```

Starting from `v1.12.2`, for comments of the type `/* */`, you can now use `-` to fully write out Headers (and Body), like so:

```go
/*
	- Content-Type: application/json
	- X-Request-Id: {{ RequestID }}
	
	{{ $.body }}
*/
```

(The role of the `-` is to remove the space characters before and after it, including the `-` itself.)

Starting from `v1.13.0`, you can add an optional parameter `Scan(expr...)` between `<MEHOTD>` and `<URL>`, similar to a function call, to indicate to `defc` that a certain parameter from the method's parameters should be added to the `ScanValues` list (the parameter position is before the return value). A common scenario is when the data type returned by an interface is not in a fixed format, and we are unable to deduce the return type at the schema definition stage, so the final type needs to be passed in through a method parameter (similar to the `Scan` method in `database/sql`), as follows:

```go
type Object interface {
  Type() string
  ID() string
}

//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // GetObject GET Scan(obj) {{ $.Service.Host }}/api/{{ $.obj.Type }}/{{ $.obj.ID }}
  GetObject(ctx context.Context, obj Object) error
}
```

Additional note: From `v1.12.0` onwards, you can use a backslash `\\` to perform a line break in `<URL>`, like so:

```go
/*
  GetUser GET ONE
    {{ $.Service.Host }}/user/{{ $.id }}?\
      field1=value1&\
      field2=value2
*/
```

### Logging

Similar to `sqlx`, if you wish to log request information, please add the `--features=api/log` feature and implement the `Log` interface for the return type of `Inner`. The definition of the `Log` interface is as follows:

```go
type Log interface { 
  Log(ctx context.Context, caller string, method string, url string, elapse time.Duration)
}
```

Here, the `caller` parameter refers to the name of the current method, `method` refers to the request method, `url` is the URL of the request, and `elapse` is the time taken for the current request.

Starting from `v1.14.0`, use `--features=api/logx` to enable the "enhanced" logging feature. This feature, when enabled, will pass `*http.Request`/`*http.Response` to the `Log` interface call, allowing for more detailed logging of each request and response; the detailed definition of the `Log` interface is as follows:

```go
type Log interface {
  Log(ctx context.Context, caller string, request *http.Request, response *http.Response, elapse time.Duration)
}
```

### Using a Custom `http.Client`

If you want to use a custom `http.Client`, please add the `--features=api/client` feature and implement the `Client` interface for the return value of the `Inner` method. The definition of the `Client` interface is as follows:

```go
type Client interface {
  Client() *http.Client
}
```

## ⚠️ Experimental `defc generate` Command

Starting with `v1.2.0`, a new `defc generate` command has been added to generate code from a Schema file. **This is an experimental feature**. The specific usage is as follows:

```shell
go run -mod=mod "github.com/x5iu/defc" generate --mode=sqlx --output=query.go schema.json
```

The content of `schema.json` (example) is:

```json
{
  "package": "main",
  "ident": "Query",
  "imports": [
    "context",
    "database/sql"
  ],
  "schemas": [
    {
      "meta": "GetUserNames QUERY",
      "header": "SELECT `name` FROM `user` WHERE `id` >= ?;",
      "in": [
        {
          "ident": "ctx",
          "type": "context.Context"
        },
        {
          "ident": "id",
          "type": "int64"
        }
      ],
      "out": [
        {
          "ident": "names",
          "type": "[]sql.NullString"
        },
        {
          "ident": "err",
          "type": "error"
        }
      ]
    }
  ]
}
```

You can also use this type of (example) `.toml` Schema file:

```toml
package = "main"
ident = "Service"

imports = [
    "context",
]

[[declare]]
ident = "ResponseOfBaidu"
fields = [
    { ident = "Body", type = "string", tag = "json:\"body\"" },
    { ident = "Javascript", type = "string", tag = "json:\"javascript\"" },
]

[[schemas]]
meta = "PostBaidu POST https://baidu.com?t={{ $.timestamp }}"
header = """
Content-Type: application/json

{{ $.message }}
"""

in = [
    { ident = "ctx", type = "context.Context" },
    { ident = "message", type = "string" },
    { ident = "timestamp", type = "int64" },
]

out = [
    { type = "ResponseOfBaidu" },
    { type = "error" },
]
```

The specific format of the Schema is determined by the following types in `github.com/x5iu/defc/gen/generate.go`:

```go
type (
  Config struct {
    Package  string     `json:"package" toml:"package" yaml:"package"`
    Ident    string     `json:"ident" toml:"ident" yaml:"ident"`
    Features []string   `json:"features" toml:"features" yaml:"features"`
    Imports  []string   `json:"imports" toml:"imports" yaml:"imports"`
    Funcs    []string   `json:"funcs" toml:"funcs" yaml:"funcs"`
    Schemas  []*Schema  `json:"schemas" toml:"schemas" yaml:"schemas"`
    Include  string     `json:"include" toml:"include" yaml:"include"`
    Declare  []*Declare `json:"declare" toml:"declare" yaml:"declare"`
  }

  Schema struct {
    Meta   string   `json:"meta" toml:"meta" yaml:"meta"`
    Header string   `json:"header" toml:"header" yaml:"header"`
    In     []*Param `json:"in" toml:"in" yaml:"in"`
    Out    []*Param `json:"out" toml:"out" yaml:"out"`
  }

  Param struct {
    Ident string `json:"ident" toml:"ident" yaml:"ident"`
    Type  string `json:"type" toml:"type" yaml:"type"`
  }

  Declare struct {
    Ident  string   `json:"ident" toml:"ident" yaml:"ident"`
    Fields []*Field `json:"fields" toml:"fields" yaml:"fields"`
  }

  Field struct {
    Ident string `json:"ident" toml:"ident" yaml:"ident"`
    Type  string `json:"type" toml:"type" yaml:"type"`
    Tag   string `json:"tag" toml:"tag" yaml:"tag"`
  }
)
```

The `defc generate` command works by deserializing the content in the Schema file into `gen.Config`, and then calling the `gen.Generate` function to generate the corresponding code. The currently supported Schema formats are `json`/`toml`/`yaml`.

Additionally, you can directly use the `gen.Generate` function in the code by passing `gen.Mode` and `gen.Config` to manually generate code in the corresponding mode without the need for command line and Schema file.

You can also compile `github.com/x5iu/defc` into a binary file and then use the `defc generate` command to generate code in the corresponding mode.

*Note: Currently, `defc generate` is only an experimental feature, and the stability of its functions and API is not guaranteed. Documentation is also not yet complete, and users may need to figure out how to use various features themselves (such as the formats of various parameters).*

## Additional Features (`--features`) Explanation

### `sqlx/nort` and `api/nort`

Using the `nort` feature tells `defc` not to import the additional `github.com/x5iu/defc/__rt` package. All auxiliary interfaces, types, and functions will be defined within the generated file; the generated file will contain only the standard library (with the addition of `github.com/jmoiron/sqlx` in `sqlx` mode).

### `api/cache`

The `api/cache` feature provides caching functionality for interfaces. This feature must be used in conjunction with the `Inner` method. The return value type of the `Inner` method must implement the following interface:

```go
type Cache interface {
  GetCache(string, ...any) []any
  SetCache(string, []any, ...any)
}
```

In this, the first `string` parameter of the `GetCache` method is the caller information, i.e., the name of the current executing method, and the variable-length `...any` parameters are the current method's arguments. The return value type of the `Inner` method must handle the storage of the cache and the mapping relationship between method arguments and the cache; the `SetCache` method's first `string` parameter is also the caller information, the second `[]any` parameter are the method's arguments, and the third variable-length `...any` parameters are the method's return values (these return values do not include `error`). The return value type of the `Inner` method must complete the storage of the cache and the mapping relationship between the method's arguments and the cache.

### `api/error`

Using the `api/error` feature will return a `ResponseError` when the HTTP response code is not 2xx. The definition of `ResponseError` is as follows:

```go
type ResponseError interface {
  error
  Status() int
  Body() []byte
}
```

The following method can be used to determine if an error occurred at the HTTP layer (and not at the business layer):

```go
if e, ok := err.(__rt.ResponseError); ok {
  status, body := e.Status(), e.Body()
  // error handling
}
```

*Note: `ResponseError` will only be returned when the HTTP response code is not 2xx. Errors in other scenarios will not implement the `ResponseError` interface.*

### `api/future`

*Note: The future feature will be offered as a regular feature in defc v2, containing a series of Breaking Changes.*

Firstly, there is a change in the `__rt.Response` interface. The `future` feature adds a `__rt.FutureResponse` interface, which differs from the `__rt.Response` interface in that it changes the original `FromBytes` method to the `FromResponse` method. The new method signature is as follows:

```go
FromResponse(string, *http.Response) error
```

It replaces the original `[]byte` type with `http.Response`, aiming to give callers the flexibility to handle responses based on the specific Headers and the type of Body. For example, one could choose to deserialize a `Content-Type: application/json` response using JSON deserialization to handle the Body; moreover, handling the Body becomes more flexible. Previously, defc would read out all the Body content (in the form of `[]byte`) before processing, but now you can choose a custom method to read the content of the Body, such as `json.Decoder`. For longer and more extensive content, custom `Read` methods could be more efficient and consume less memory than reading out all bytes at once.

**However, it is important to note that if you use the `FromResponse` method, you must take responsibility for calling the `Close` method on `http.Response.Body`; in other words, you must close the response's Body yourself, as defc will not generate any `Body.Close` related code for you.**

Secondly, since defc no longer reads out the complete Body information in advance, if you use the `api/error` feature, you will face a problem: the original `__rt.ResponseError` interface's Body method will not be able to retrieve the Body of the Response, causing this interface to malfunction. To solve this, we have defined a new interface `__rt.FutureResponseError` with the following definition:

```go
type FutureResponseError interface {
  error
  Response() *http.Response
}
```

Instead of returning `Status` and `Body` separately, it will return the complete `http.Response`, giving error handling more freedom (but also more complexity).

**It is particularly important to note: If you enable both `api/future` and `api/error` features, then the `http.Response` obtained from the request when the HTTP status code is non-2xx will be wrapped in `FutureResponseError` (for error handling). This also means that the caller must be responsible for closing `http.Response.Body` while handling errors. Please be particularly careful with how errors are handled when enabling the aforementioned features, and remember to close `http.Response.Body` promptly to avoid resource leaks.**

### `sqlx/in`

Using the `sqlx/in` feature will result in the following two changes:

1. The `In` function implemented by `defc` will replace the `sqlx.In` function (which is applied when the `NAMED` option is enabled, to recalculate placeholders, generate SQL statements, and bind query parameters). Unlike the `sqlx.In` function, the `In` function implemented by `defc` will not only destructure slice-type parameters but also destruct parameters that implement the `ToArgs` interface. The application scenario here is that when using `NAMED` queries, if the parameter type is a slice or `ToArgs` type, `defc` will recalculate the number of placeholders and bind them with the destructed parameters, which is commonly used for `IN` queries. For instance, in the example below, a single `:ids` placeholder is sufficient for an `IN` query:

    ```sql
    SELECT * FROM `user` WHERE `id` IN (:ids);
    ```

2. For methods where the `NAMED` option is not enabled, `defc`'s `In` function will be used to recalculate placeholders, generate SQL, and bind query parameters before SQL execution. This means that users can complete the binding of multiple parameter placeholders without the help of the `bindvars` function, as illustrated in the following example:

    ```sql
    -- Without enabling the sqlx/in feature
    SELECT * FROM `user` WHERE `id` IN ({{ bindvars $.ids }});
    
    -- With the sqlx/in feature enabled
    SELECT * FROM `user` WHERE `id` IN (?); -- ? placeholder binding the parameters ids
    ```

**Note: Please use the `sqlx/in` feature and the `bindvars` function (in the template) together with caution. Incorrect use of the `bindvars` function under the `sqlx/in` feature can lead to unpredictable consequences. However, since `bindvars` still has its specific use cases (like quickly generating N placeholders), it is not forcibly disabled under the `sqlx/in` feature.**

### sqlx/future

Added in `defc@v1.17.0` as an **experimental feature**, the `sqlx` mode introduces the `github.com/x5iu/sqlx` package as a replacement for the `github.com/jmoiron/sqlx` package, providing the following interfaces:

```go
type IRow interface {
  Columns() ([]string, error)
  Scan(...any) error
}

type FromRow interface {
  FromRow(IRow) error
}
```

For **structures** that implement the `FromRow` interface, the `sqlx.Select`/`sqlx.Get` functions will preferentially use the `FromRow` method to map the query results into the structure, rather than using `sqlx.reflectx` (the default mapping method in the original `sqlx`, which maps database query results through reflection). The `FromRow` is particularly useful for mapping private fields in structures (the original `sqlx` only supports mapping data to exportable fields, i.e., `Exported Field`).

***Note, please implement the `FromRow` interface only for structures.***

### sqlx/callback

Added in `defc@v1.18.0` as an **experimental feature**, in the `sqlx` mode, you can implement the following interface for the return type of methods:

```go
interface {
  Callback(context.Context, Interface) error
}
```

For the return values that implement this interface (the `Interface` in the interface parameters represents the interface for the `defc` Schema, which is marked and code-generated using `go:generate`), `defc` will generate the corresponding callback code. That is, after completing the SQL query defined by the method, it will additionally call the `Callback` method. Generally, if some fields in your structure need to be mapped from other SQL queries (such as Relations/Edges), using `Callback` is an excellent choice. You can define the code to query Relations/Edges in the `Callback` method and map the results to the corresponding fields in the structure.

***Note: If your structure has a circular reference (e.g., a field in your structure A is of type B structure, and the B structure contains a field of type A structure), be careful to prevent StackOverflow caused by the circular calling of Callback. A common solution is to add a corresponding call identifier in the Context. When a specific identifier is detected or satisfies certain conditions, terminate the Callback call.***

***Note 2: The queries within Callback and the main function’s queries by default belong to different transactions. If you want to ensure that Callback and the main function are within the same transaction, please use the `WithTx` method to start a transaction. After starting the transaction, the queries within Callback and the main function will belong to the same transaction.***

### sqlx/any-callback

Added in `defc@v1.19.1` as an **experimental feature**, it is essentially the same as `sqlx/callback`, with the difference that the required interface is changed to:

```go
interface {
  //                        👇 note this
  Callback(context.Context, any) error
}
```

This alteration signifies a more generalized approach in the callback interface, expanding the potential use cases.

## Answers to Common Questions

### How to pass multiple values for the `--features` parameter

To pass multiple features, please use the following method:

```
--features=sqlx/log,sqlx/rebind
```

### About `Query` and `NamedQuery`

In SQL queries, `defc` does not expand `?` parameter placeholders into the number of `?` placeholders corresponding to the parameters (this process would expand slice parameters) using `sqlx.In`, because we believe that the relationship between SQL and parameters should be dominated by SQL. The form of SQL should not be determined by the specific number of parameters. Our recommended approach is to dynamically generate `?` parameter placeholders using the `bindvars` function.

Based on the above logic, if you are using a regular query (i.e., a query without `NAMED` parameters), then you can use a slice as the query parameter. With the `bindvars` function, you can smoothly expand the query parameters and complete an `IN` query. However, if you use `NAMED` for querying, since there is no good way to expand parameters and `bindvars` cannot be used (because the `bindvars` function generates question mark `?` placeholders), its support for `IN` queries is not ideal in most scenarios.

**Starting from `v1.7.0`, you can use slice values as query parameters in the `NAMED` parameter passing method.**

### About the `db` Tag in `MergeNamedArgs`

The reason `MergeNamedArgs` uses the `db` tag, as mentioned above, is because the `sqlx` package uses the tag name `db`. At the same time, since the definition of the `db` tag in `sqlx` is very pure, merely to indicate its mapping to database field names without extra parameters (extra parameters refer to things like the `,omitempty` parameter defined in the `json` tag in the `encoding/json` package), it is not burdensome to use and matches the database fields, which aligns with our expectations for constructing SQL query statements.

### Using types from other packages

**Starting from `v1.15.4`, `defc` supports automatic import of the required packages through static code analysis, so there is no need to manually use the `--import` command line parameter to import external packages. However, the `--import` parameter is still retained, and can still be used when you need to import anonymous packages (such as database drivers), for example:**

```shell
--import "_ https://github.com/mattn/go-sqlite3"
```

*Note: If your code contains unsafe packages or C packages (commonly used with CGO), you still need to manually import the unsafe/C packages using `--import`, as `defc` cannot handle unsafe and C packages.*

*Note 2: In some special cases (e.g., when there are different build tags in some files and there is a package name conflict), automatic import may not work as expected. In such cases, manually importing the required packages is the best choice. Therefore, `defc` provides a `--disable-auto-import` option to disable the automatic import feature. If `--disable-auto-import` is enabled, you will need to import the required packages yourself using the `--import` parameter.*

Since `defc` does not implement a fully accurate recognition of types from other packages, if you want to use types from other packages in your Schema, such as the `url.URL` type, the recommended way is to use a `type alias`:

```go
type (
  URL = url.URL
)
```

From `v1.1.0`, you can use the `--import` command line parameter to import additional packages to use the types (or functions) in that package:

```shell
go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --import "fmt" --import "gohttp net/http"
```

The format for the `--import` parameter can be the package's path, such as `strings`/`fmt`/`github.com/x5iu/defc`, or the package path with an alias, such as `gohttp net/http`/`gofmt fmt`.

### Adding Additional Functions to the Templates in Schema

Starting from `v1.1.0`, you can use the `--func` command line parameter to add additional functions to the SQL or API templates in the Schema, such as `json.Marshal` or `url.QueryEscape`:

```shell
go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --import "encoding/json" --func "marshal=json.Marshal" --func "toUpper: strings.ToUpper"
```

The format for the `--func` parameter is `key=value` or `key: value`, where `key` is the name of the function used in the template, which can also be understood as the `key` in `template.FuncMap`, and `value` is the function to be imported.

### About Generic Support

At present, `defc`'s support for generics is not yet perfect and stable. In practice, there have not been many scenarios where the use of generics is necessary. This is also because Go's current restrictions on the use of generics only allow interface generic parameters to be defined at the outermost layer of the interface, rather than allowing generic parameters to be defined for individual methods within the interface (but Rust can).

Therefore, we will consider improving `defc`'s support for generics when Go supports defining generic parameters for interface methods separately.

### Why Not Generate Global Reusable Prepare Statements to Improve SQL Query Efficiency

On the surface, there are many bizarre reasons, such as:

1. Template statements and multi-statements are not applicable;

2. Prepare Statement can cause multiple TCP connections (Prepare, Exec/Query, Close), while direct Exec/Query can, in most cases, complete a query in one TCP connection (depending on whether the Driver has implemented `Queryer/QueryerContext`), see `database/sql/driver/driver.go:204`;

	> If a Conn implements neither QueryerContext nor Queryer, the sql package's DB.Query will first prepare a query, execute the statement, and then close the statement.

3. Each Stmt occupies a Conn, and in concurrent situations, it can cause a large number of Conn creations and switches, and even lead to resource leaks;

4. The official recommendation is that Stmt should be a local variable rather than a global variable.

The deeper reasons lie in the fact that for Stmt, there are many situations that will trigger RePrepare (RePrepare refers to the behavior where the Stmt needs to be submitted to the database server again for Prepare request when the Stmt is closed, the Conn has become invalid, or it is executed across transactions). For example:

> If the statement has been closed or already belongs to a transaction, we can't reuse it in this connection. Since tx.StmtContext should never need to be called with a Stmt already belonging to tx, we ignore this edge case and re-prepare the statement in this case. No need to add code-complexity for this.

Even with global Stmts, since they are bound to Conns, switching Conns will trigger RePrepare. In concurrent situations, switching Conns is very common, not to mention that every time a transaction is started, a new Conn is obtained (and whether this Conn has cached the Stmt is still unknown). This leads to the situation where even if global Stmts are used,