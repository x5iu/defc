# defc

根据预先定义的 Schema，自动生成代码

## 简介

`defc` 源于在日常工作、生活中写代码时，对重复性地编写”增删改查“、”网络接口对接“代码的厌倦。

例如，对于数据库查询，我们往往需要为一个新的查询：

1. 定义一个新函数、方法；
2. 编写一条新的 SQL；
3. 执行查询，处理错误，将查询结果映射到结构体中；
4. 如果有多条 SQL，需要开启事务，执行提交或回滚；
5. 记录查询日志；
6. ……

又例如，对于网络接口对接，我们往往需要为一个新的接口：

1. 定义一个新函数、方法；
2. 设置接口地址，配置接口参数（例如 HTTP 请求中的 Header、Query、Body 等）；
3. 发起请求，处理错误，将请求结果映射到结构体中；
4. 如果是分页查询，需要将多次查询分页结果拼接成最终结果；
5. 记录请求日志
6. ……

以上这些，在每次写新需求新场景时，都要重复若干次，尤其是其中的查询、请求、错误处理、事务提交/回滚、数据映射、列表拼接、日志记录等，都是逻辑相同的重复性代码，写着非常烦，有些代码还非常长，复制粘贴又需要各种改变量名、方法名、配置信息等，非常影响开发效率；

遗憾的是，Go 语言并没有提供官方宏功能，我们无法像 Rust 一样通过宏来完成这些复杂的重复性代码（当然，宏也有其局限性，在未展开的情况下，对于代码可读性是毁灭性的打击，同时也影响 IDE 的补全）；不过好在 Go 也提供了曲线救国的方式 `go generate`，通过 `go generate` 可以近似地提供宏功能，即代码生成功能。

基于以上背景，我希望实现一个代码生成工具，通过定义查询或请求的 **Schema**，即可以自动生成完成相关功能的 SQL 增删改查或 HTTP 请求，其中包括**参数构建、错误处理、结果映射、日志记录**等逻辑，`defc` 就是我对于这种从 Schema 到代码生成的实验性尝试，“def”即“define”，表示预定 Schema 的行为；目前 `defc` 提供了以下两种场景的代码生成功能：

- 基于 [sqlx](https://github.com/jmoiron/sqlx) 的数据库增删改查代码生成
- 基于 Golang 标准库中 `net/http` 包实现的 HTTP 接口请求代码生成

## sqlx mode

*注意：使用此模式需要引入 sqlx 包，请在项目目录下执行：*

```
go get github.com/jmoiron/sqlx
```

首先，我们需要定义 SQL 查询的 Schema，一个基本的 Schema 如下所示：

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

一个 Schema 应是一个接口（Interface），其中的每个方法代表一次 sql 查询，一个 Schema 包含以下三部分：

1. `go generate` 命令；
2. 接口名称定义；
3. 方法定义；

其中方法定义又包含四部分：

1. 方法名及查询方式定义；
2. SQL 语句定义；
3. 查询参数定义；
4. 查询结果及错误定义；

首先是 `go generate` 部分，`go generate` 接收一个 shell 命令，这里我们使用 `go run -mod=mod "github.com/x5iu/defc"` 来执行代码生成命令，当然如果你不想使用 `go run` 命令，或当前环境没有 `go` 编译器，也可以将 `defc` 编译后单独使用，将 `defc` 编译后添加至环境变量 `PATH` 中，再使用命令 `defc --mode=sqlx` 替代上文所说的 `go run -mod=mod` 命令，也是可行的。`defc` 命令接收两个基本参数 `--mode/-m` 和 `--output/-o`，`--mode` 命令指定当前执行的代码生成模式，在本例中应是 `--mode=sqlx`；`--output` 命令则指定生成的代码文件的位置。

*注：额外的，`defc` 命令还支持 `--features` 参数，这在后面的说明中会提及。*

接着是接口名称定义和方法定义，在定义完接口名称后（本例中的接口名称是“Query”），`defc` 会生成这样的若干构造函数：

- `NewQuery`
- `NewQueryFrom*`

方法定义是 Schema 中最重要的部分，其将直接影响到最终生成的代码逻辑，我们将就此部分深入展开介绍。需要注意的是，**Schema 的定义依托于注释内容，因此在 Schema 中，很可能你无法在注释中自由地书写你想表达的内容；并且，请使用单行双斜杠注释`//`，请不要使用多行`/* */` 注释，不然 `defc` 会无法解析注释中定义的 Schema 内容；注释请紧贴方法定义**。

### 方法名及查询方式定义

这是注释的第一行内容，固定为注释的第一行，格式为：方法名、查询方式、及可选的查询参数：

```
// <NAME> <CMD> <ARG>
```

其中 `<NAME>` 为方法名，请与当前接口名称保持一致（这是一种约定，并没有做强制要求，意味着对于 `<NAME>`，你随便打一个单词都没有问题，但一定要有，不能为空；

`<CMD>` 即为当前查询方式的定义，目前支持的命令为（该命令大小写无关）：

- `EXEC`：对应 `sqlx` 中的 `Exec` 函数/方法
- `QUERY`：对应 `sqlx` 中的 `Query`/`Get` 函数/方法

`<ARG>` 为可选参数，仅支持一个值 `NAMED`（同样大小写无关），如果未提供该参数，则使用 `sqlx` 的普通查询方式，即 `Exec`/`Query`/`Get` 函数/方法；若提供 `NAMED` 参数，则使用 `sqlx` 中的 `PrepareNamed` 函数/方法构建 `sqlx.NamedStmt`，再调用 `sqlx.NamedStmt` 中的 `Exec`/`Query`/`Get` 函数/方法。

### SQL 语句定义

从注释的第二行开始，则为 SQL 语句定义，在这里可以书写你要执行的 SQL，支持 SQL 换行（但仍然需要在注释范围内），及多条 SQL（多行 SQL 仅支持在 `EXEC` 中使用，多条 SQL 以分号 `;` 分隔，这些 SQL 会被放在同一个事务中执行）；SQL 中的参数使用问号 `?` 占位符表示，**额外的：如果你使用的是 `PostgreSQL` 或其他不使用 `?` 占位符的数据库，你可以在 `def` 命令中使用 `--features=sqlx/rebind` 特性来将 SQL 中的 `?` 转换为适配对应数据库的占位符，例如 `PostgreSQL` 中的 `$1`。**

### 查询参数定义

那么，如何往这些问号占位符传值呢？`sqlx` 乃至 Go 标准库中 `database/sql` 中都是使用变长参数将值传递给数据库，`defc` 同样采用了这种方式，但是做了一些封装，我们将详细讲述这样的规则：

首先我们描述一般情况下，即不带有 `NAMED` 参数的场景：

1. 方法定义中，方法的入参，除了 `ctx context.Context` 外，其余的参数按入参顺序，传递给底层的 `Exec`/`Query` 函数/方法（**注意，所有的入参，均需要携带参数名称，不能定义只有类型没有参数名称的方法**）；

2. 对于切片（`Slice`）类型的参数，在传参时，会将切片拆分成切片长度的若干个参数，例如参数 `[]int{1, 2, 3}` 将以这样的方式传入 `Query` 方法（`[]byte` 类型除外，`[]byte` 本身是合法的 `driver.Value` 值）：

	```
	sqlx.Query(sql, 1, 2, 3)
	```

3. 但是大部分时候，方法的入参并不是基本类型，深入地说，并不都是 `driver.Value` 类型，如果不是 `driver.Value` 类型，在大多数时候都会导致查询的运行时错误；其次，多数时候我们会使用一个结构体来包装部分参数，避免方法签名过长；对于这两种场景，`defc` 提供了 `ToArgs` 接口，对于实现了 `ToArgs` 接口的类型，在传参时，会调用其 `ToArgs` 方法，并将返回的参数切片拆分后合并入查询参数中，`ToArgs` 接口的定义位于 `github.com/x5iu/defc/__rt` 包中，定义如下：

	```go
	type ToArgs interface {
	  ToArgs() []any
	}
	```

4. 额外的，如果你不想让一个方法入参作为 SQL 查询的参数，可以为这个入参的类型实现 `NoAnArg` 接口，该接口同样位于 `github.com/x5iu/defc/__rt` 包中，其定义为：

	```go
	type NotAnArg interface {
	  NotAnArg()
	}
	```

	

对于带有 `NAMED` 参数的场景：

1. 使用 `:name` 的参数占位符代替问号 `?`，其中 `name` 为参数名称；

2. 方法定义中，方法的入参，除了 `ctx context.Context` 外，其余的参数按 `key:value` 的方式，参数名为 `key`，参数值为 `value`，传递给 `sqlx.PrepareNamed` 函数/方法（**同样的，所有的入参，均需要携带参数名称，不能定义只有类型没有参数名称的方法**）；

3. **注意，使用 `NAMED` 查询方式，将不支持切片参数，`defc` 不会展开 `NAMED` 模式下的切片参数**；

4. 对于定义了 `ToNamedArgs` 的类型，`defc` 将会将 `ToNamedArgs` 返回的 `map` 的内容合并到查询参数中；额外的，如果参数类型是结构体或结构体指针，还会查询结构体字段的 `db` Tag（这也是 `sqlx` 定义的 Tag），对于有 `db` Tag 的字段，会将 `db` Tag 的值作为 `key`，将字段值作为 `value` 加入查询参数中；如果参数类型是 `map` 类型，`defc` 会将该 `map` 合并到查询参数中；`ToNamedArgs` 接口的定义位于 `github.com/x5iu/defc/__rt` 包中，定义如下：

	```go
	type ToNamedArgs interface {
	  ToNamedArgs() map[string]any
	}
	```

5. 同样的，如果你不想让一个方法入参作为 SQL 查询的参数，可以为这个入参的类型实现 `NoAnArg` 接口；

通常情况下，对于复杂的 SQL，我们会使用字符串拼接的方式来构建，但这样既不安全，也不方便，代码还会很混乱；因此，`defc` 默认提供了模板功能来构建 SQL，在注释中的 SQL 语句支持 Go 语言标准库中 `text/template` 的所有模板语法。在 SQL 中可以通过 `{{ $.Arg }}` 来访问方法入参，其中 `Arg` 指方法入参的参数名称，例如：在方法签名 `GetUser(ctx context.Context, id int64)` 中，你可以使用 `{{ $.id }}` 来访问 `id` 参数；同样的，如果这些参数类型有附带方法，你也可以通过模板语法调用这些参数的方法，同样的，判断和循环语句也是支持的。

*注意：请不要使用模板语法来构建 SQL 查询参数，请使用问号 `?` 占位符来传参，以规避 SQL 注入风险。*

额外的，`defc` 提供了一个特殊的模板函数 `bindvars`，`bindvars` 接受一个参数，这个参数可以是任意类型，如果是整数类型（无论是否有符号），则按给定的数值生成对应数量的问号 `?` 占位符，以逗号分隔；如果是切片（`Slice`）类型，则生成切片长度的问号 `?` 占位符，以逗号分隔；如果是其他类型，则生成一个问号 `?` 占位符。`bindvars` 在应对切片类型的 SQL 参数时非常有用，可以快速生成符合条件的查询参数占位符。例如：

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // GetUsers QUERY
  // SELECT * FROM `user` WHERE `id` IN ({{ bindvars $.ids }});
  GetUsers(ctx context.Context, ids []int64) ([]*User, error)
}
```

关于 `ctx context.Context` 参数，这是一个可选参数，但是我们建议在通常情况下的方法定义都带上 `ctx context.Context` 参数，按照 `Go` 语言的规范，我们约定其参数名为 `ctx`（请不要使用其他参数名，因为 `defc` 就只认识 `ctx`）；

### 查询结果及错误定义

首先需要明确的是，**所有的方法都需要携带一个 `error` 返回值**，方法可以不返回结果，但不能不返回错误，如果没有 `error` 返回值，`defc` 会在生成代码时报错，这是一个强约束。

对于 `EXEC` 类型的查询，`defc` 不会调用任何 `Scan` 方法，因为 `EXEC` 执行的是增删改类的操作，其本不应该返回任何数据，所以通常而言，`EXEC` 类的方法应只返回 `error` 一个返回值。

但是，很多时候我们会关心 `INSERT` 语句插入后的 `LastInsertId`，或是 `UPDATE` 语句执行后的 `RowsAffected` 值，因此 `defc` 额外对 `EXEC` 提供了对 `sql.Result` 接口作为返回值的支持，例如，你可以这样定义一个 Schema：

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // INSERT INTO `user` (`name`, `age`) VALUES (?, ?);
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

对于 `QUERY` 类型的查询，你可以使用一个基本类型，或是一个结构体（及结构体指针），或是一个切片来作为返回值，`defc` 会自动将查询结果映射到返回值中。

### 事务支持

如果需要在一个事务中执行查询，可以为 Schema 添加一个特殊的方法 `WithTx`，具体示例如下：

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  WithTx(context.Context, func(Query) error) error
}
```

`WithTx` 方法第一个参数为 `context.Context`（同样为可选参数，但通常建议带上它），第二个参数为一个函数，函数的入参为 Schema 定义的接口，在函数中，你可以通过这个入参来执行查询，这些查询会被放在同一个事务中执行，事务会自动提交，遇到错误则会自动回滚。

### 日志记录

如果需要记录查询日志，请添加 `--features=sqlx/log` 特性并使用 `New*FromCore` 构造方法，并对传入的 `core` 参数的类型实现 `Log` 接口，`Log` 接口的定义如下：

```go
type Log interface { 
  Log(ctx context.Context, caller string, query string, args any, elapse time.Duration) 
}
```

其中，`caller` 参数为当前方法的方法名，`query` 为当前查询的 SQL 语句，`args` 为查询参数，`elapse` 为查询所花费的时间。

### 嵌入复杂 SQL

~~很多~~少数情况下，我们会编写非常复杂非常长的 SQL 查询语句，将这些 SQL 写在注释中显然非常影响代码的可读性，也不利于 SQL 的维护，因此 `defc` 提供了在编译期嵌入 `.sql` 文件的功能（事实上所有文件类型都支持，例如 `.tmpl` 文件），只需要使用 `#include` 语句引入文件即可（这看起来像 C 语言的 `include`）：

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --output=query.go
type Query interface {
  // CreateUser EXEC
  // #include insert.sql
  CreateUser(ctx context.Context, user *User) (sql.Result, error)
}
```

*注：`#include` 语句应存在于单独一行的注释中，并且该行只应该包含 `#include` 及文件名，每次只支持引入一个文件，但你可以多次使用 `#include` 来引入不同文件。*

`#include` 的另一个最佳实践为，如果你的 SQL 总是包含一些共用的查询条件，或是查询字段，你可以将他们写在一个单独的 `.sql` 文件中，并使用 `#include` 在不同方法中引入，这样你就不用重复编写这些通用的查询语句，并且只需要更新 `.sql` 文件即可更新所有引入了该 `.sql` 文件的方法 Schema。

## api mode

`api` 模式下，Schema 的定义与 `sqlx` 模式大体相同，一个基本的 Schema 定义如下所示：

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

有了 `sqlx` 模式的基础，理解 `api` 模式下的 Schema 就容易得多了，这里我们略过与 `sqlx` 重复的概念，直接介绍其中的不同点。

### `Inner` 和 `Response` 方法

与 `sqlx` 的 Schema 的第一个不同点在于，`api` 的 Schema 需要定义两个辅助方法，分别是 `Inner` 和 `Response`。

`Inner` 方法返回的类型，将作为 Schema 的依赖，并且 Schema 的构造方法 `New*` 将会把 `Inner` 方法返回值的类型作为入参来完成依赖注入；通常而言，我们会把接口调用中使用的 host、key、secret、token 等信息放置在 `Inner` 方法的返回值中。

`Response` 方法返回的类型，将用于构造 HTTP 请求的返回值，`Response` 方法的返回值必须实现以下接口：

```go
type Response interface {
   Err() error
   ScanValues(...any) error
   FromBytes(string, []byte) error
   Break() bool
}
```

`Err` 方法表示本次请求产生的与网络层无关，与业务层有关的错误，例如我们请求一个接口，网络请求成功，返回状态值 200，但在业务上产生了错误，例如请求参数错误，业务返回了 400 业务响应码，那么在获得业务返回的响应信息后，我们应将错误通过 `Err` 方法暴露给调用者。

`ScanValues` 方法用于将响应的结果映射到方法的返回值中，注意到这里的参数是一个变长参数，意味着方法的返回值可以包括不止一个参数（但这其中不包括返回的错误信息 `error` 返回值）；在我们的例子中，返回值为 `*User`，这个返回值会先分配内存，随后调用 `ScanValues` 方法将响应信息写入 `*User` 中，如果遇到错误，则会返回错误。

`FromBytes` 方法用于构造这个 `Response`，http 请求的响应为 `[]byte` 类型，完整的响应值将会传入 `FromBytes` 方法中，`Response` 类型需要自己处理响应的反序列化工作，并在遇到错误时返回错误；`FromBytes` 的第一个参数为当前方法名，适用于对不同接口执行不同的反序列化程序。

`Break` 方法仅适用于分页查询中，即当方法返回值为切片类型时，将会生成适用于分页查询的请求代码，分页查询将会使用 `Break` 方法的返回值作为是否已达到查询目的的判断标准，当 `Break` 方法返回真值时，分页查询结束，返回构建的切片结果；通常而言，对于分页查询，推荐的方式是，`Response` 存储当前的查询进度，例如已查询并存储的元素数量，并将其与期望的数量相比较（期望的数量可从响应中获取），并将比较结果作为 `Break` 方法的返回值。

### 请求 Method、URL、Header 及 Body

`api` 方法中的 Schema 定义与 `sqlx` 大同小异，都是将定义写在注释中，其格式为：

```
// <NAME> <METHOD> <URL>
// <HEADER>
//
// <BODY>
```

其中 `<NAME>` 为方法名；`<METHOD>` 为 http 请求的方法（例如 `GET`/`POST`）；`<URL>` 为请求的 URL 地址；`<HEADER>` 为请求 Headers，支持多行；`<BODY>` 为请求 Body，需要注意的是，同 http 报文一样，`<BODY>` 与 `<HEADER>` 之间必须包含一个 `\r\n` 分隔符（即空行）。

其中，`<URL>`/`<HEADER>`/`<BODY>` 部分同样支持模板语法。而与 `sqlx` 中的模板不同的是，`api` 额外将 `Inner` 方法的返回值也作为模板参数传入模板中，访问它的方式是 `{{ $.Schema }}`，其中，`Schema` 为定义的 Schema 名称，在本例中为 `Service`，即是说，你可以在本例中使用 `{{ $.Service }}` 来访问 `Inner` 方法的返回值。额外的，对于分页查询接口，如果你使用了 `--features=api/page` 参数，`api` 将提供一个额外的 `page` 函数（注意是函数而不是模板参数），使用 `{{ page }}` 来访问当前执行的分页计数，分页计数从 `0` 开始计数，每当调用一次 `{{ page }}` 函数，分页计数则会加一，例如，我们可以这样定义一次分页查询方法：

```go
//go:generate go run -mod=mod "github.com/x5iu/defc" --mode=api --output=service.go
type Service interface {
  Inner() *Inner
  Response() *Response
  
  // GetUsers GET {{ $.Service.Host }}/users?name={{ $.name }}&page={{ page }}
  GetUsers(ctx context.Context, name string) ([]*User, error)
}
```

如果你不想通过注释中的模板来构建 HTTP 请求 Body，`api` 还提供了另一种传入 Body 的方式，具体的方式为，在注释中不填入 Body 内容（仍然可以填入 Header），此时，`defc` 将会把方法最后一个参数视作 HTTP 请求的 Body，需要注意的是，如果使用这种方式来传入 Body，则方法最后一个参数必须是 `io.Reader` 类型，具体示例如下（改写上面的例子）：

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



### 日志记录

与 `sqlx` 一样，如果你想要记录请求日志，那么请添加 `--features=api/log` 特性并为 `Inner` 的返回值类型实现 `Log` 接口，`Log` 接口的定义如下：

```go
type Log interface { 
  Log(ctx context.Context, caller string, method string, url string, elapse time.Duration)
}
```

其中，`caller` 参数为当前方法的方法名，`method` 为请求方式，`url` 为请求的 URL，`elapse` 为当前请求所花费的时间。

### 使用自定义的 `http.Client`

如果你想使用自定义的 `http.Client`，那么请添加 `--features=api/client` 为 `Inner` 方法的返回值实现 `Client` 接口，`Client` 接口的定义如下：

```go
type Client interface {
  Client() *http.Client
}
```

## ⚠️ 实验性的 `defc generate` 命令

从 `v1.2.0` 开始，新增了 `defc generate` 命令用于根据 Schema 文件生成代码，**这是一项实验性的功能**，具体的使用方式为：

```shell
go run -mod=mod "github.com/x5iu/defc" generate --mode=sqlx --output=query.go schema.json
```

其中，`schema.json` 的（示例）内容为：

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

Schema 的具体格式，是根据 `github.com/x5iu/defc/gen/generate.go` 中的以下类型决定的：

```go
type (
  Config struct {
    Package  string    `json:"package"`
    Ident    string    `json:"ident"`
    Features []string  `json:"features"`
    Imports  []string  `json:"imports"`
    Funcs    []string  `json:"funcs"`
    Schemas  []*Schema `json:"schemas"`
  }

  Schema struct {
    Meta   string   `json:"meta"`
    Header string   `json:"header"`
    In     []*Param `json:"in"`
    Out    []*Param `json:"out"`
  }

  Param struct {
    Ident string `json:"ident"`
    Type  string `json:"type"`
  }
)
```

`defc generate` 的工作方式为，将 Schema 文件中的内容反序列化为 `gen.Config`，随后调用 `gen.Generate` 函数生成对应的代码，目前支持的 Schema 格式为 `json`/`toml` 格式，未来将支持更多 Schema 格式，例如 `yaml` 等（看起来都非常简单，只需要引入相应的反序列化库并加上对应的 Tag 即可）。

另外，你也可以在代码中直接使用 `gen.Generate` 函数，通过传入 `gen.Mode` 及 `gen.Config` 来手动生成相应模式下的代码，而无需使用命令行及 Schema 文件。

你也可以将 `github.com/x5iu/defc` 编译成二进制文件后，再使用 `defc generate` 命令来生成相应模式下的代码。

*注：目前 `defc generate` 仅为实验性功能，不保证功能和 API 的稳定性，文档也尚未补全，各项功能可能需要使用者自己摸索着使用（例如各类参数的格式）*

## 对一些常见问题的解答

### `--features` 参数如何实现传递多个值

请使用以下方式传递多个 features：

```
--features=sqlx/log,sqlx/rebind
```



### 关于 `Query` 与 `NamedQuery`

在 SQL 查询中，`defc` 不会使用 `sqlx.In` 来将 `?` 参数占位符扩展成对应参数（此过程会展开切片参数）数量的 `?` 参数占位符，因为我们认为 SQL 与参数的关系应是以 SQL 为主导地位，不应当根据具体参数数量来决定 SQL 形式，我们推荐的方式是使用 `bindvars` 函数来动态生成 `?` 参数占位符。

基于以上逻辑，如果你使用的是普通的查询（即不带 `NAMED` 参数的查询），那么你可以使用切片作为查询参数，配合 `bindvars` 函数可以顺利地将查询参数展开，完成一次 `IN` 查询；但如果你使用 `NAMED` 进行查询，那么由于没有什么很好的参数展开方式，也无法使用 `bindvars`（因为 `bindvars` 函数生成的是问号 `?` 占位符），所以在大多数场景下，其对 `IN` 查询的支持不是很理想。

### 关于 `MergeNamedArgs` 中的 `db` Tag

为什么 `MergeNamedArgs` 使用了 `db` Tag，上文中有提到，因为 `sqlx` 包使用的是 Tag 名称是 `db`。同时，由于 `sqlx` 对于 `db` Tag 的定义很纯粹，只为指明其与数据库字段名的映射，没有额外的参数（额外的参数指的是类似于 `encoding/json` 包中，在 `json` Tag 中定义 `,omitempty` 参数），所以使用起来没有负担，也能与数据库字段相匹配，符合我们对于 SQL 查询语句构建方式的期望。

### 使用其他包中的类型

由于 `defc` 没有实现对其他包类型的完全准确地识别，因此如果你想在 Schema 中使用其他包中的类型，例如 `url.URL` 类型，推荐的方式是使用 `type alias`：

```go
type (
  URL = url.URL
)
```

从 `v1.1.0` 开始，你可以使用 `--import` 命令行参数导入额外的包，以使用该包中的类型（或函数）：

```shell
go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --import "fmt" --import "gohttp net/http"
```

`--import` 参数的格式可以为包的路径地址，例如 `strings`/`fmt`/`github.com/x5iu/defc`，或是带有 alias 的包路径，例如 `gohttp net/http`/`gofmt fmt`。

### 为 Schema 中的模板添加额外的函数

从 `v1.1.0` 开始，你可以使用 `--func` 命令行参数为 Schema 中的 SQL 或 API 模板添加额外的函数，例如 `json.Marshal` 或 `url.QueryEscape` 等：

```shell
go run -mod=mod "github.com/x5iu/defc" --mode=sqlx --import "encoding/json" --func "marshal=json.Marshal" --func "toUpper: strings.ToUpper"
```

`--func` 参数的格式为 `key=value` 或 `key: value`，其中 `key` 为该函数在模板中使用的名称，也可以理解为 `template.FuncMap` 中的 `key`，`value` 则为需要导入的函数。

### 关于泛型支持

目前 `defc` 对泛型的支持尚未完善和稳定，实际的使用过程中，也并未发现过多必须使用泛型的场景，这也是因为 Go 语言目前对于泛型使用的限制，接口泛型参数，只能在接口的最外层定义，而无法为接口中的方法单独定义泛型参数（但是 Rust 可以）。

所以，待 Go 支持对接口方法单独定义泛型参数时，再考虑完善 `defc`对泛型的支持。