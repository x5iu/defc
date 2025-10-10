# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`defc` is a Go code generation tool that automatically generates CRUD operations for databases, HTTP client code, and net/rpc client/server wrappers based on predefined schemas. The project focuses on eliminating repetitive boilerplate code by defining interfaces and automatically generating implementation code.

## Development Commands

### Testing

```bash
# Run all tests with coverage (primary test command)
./test.sh

# Run specific module tests
go test -cover ./gen
go test -cover ./runtime
go test -cover ./sqlx
go test -tags=test ./gen/integration

# Run individual test functions
go test -run TestRunCommand ./gen
go test -run TestSplitArgs ./gen
# RPC integration single test
go test -run TestRpc -tags=test ./gen/integration

# Code quality checks
# Prefer go vet to validate code instead of go build
go vet ./...
go vet ./gen
```

### Building

```bash
# Build the binary
go build

# Install globally
go install

# Run with go run
go run -mod=mod github.com/x5iu/defc@latest --help
```

### Code Generation

```bash
# Generate code using the tool itself
go generate

# Basic usage examples
defc generate schema.go
defc --mode=sqlx --output=query.go
defc --mode=api  --output=service.go
# RPC mode example
defc --mode=rpc  --output=client.go
```

## Architecture Overview

### Core Components

1. **Main CLI (`main.go`)**: Command-line interface built with Cobra, handles argument parsing and delegates to generation logic. Uses `goimports` for automatic import formatting unless disabled with `--disable-auto-import` (see main.go)
2. **Generator (`gen/` package)**: Core code generation engine with three modes (gen/builder.go:10-16):
   - `sqlx` mode: Database CRUD operations using enhanced sqlx
   - `api` mode: HTTP client code generation
   - `rpc` mode: net/rpc client and server wrappers
3. **Runtime (`runtime/` package)**: Runtime utilities and helpers for generated code
4. **Templates (`gen/template/`)**: Go templates for code generation
   - `sqlx.tmpl`: Template for database operations
   - `api.tmpl`: Template for HTTP client operations
   - `rpc.tmpl`: Template for net/rpc clients and servers

### Code Generation Flow

1. Parse Go interface definitions from source files using `go/ast` and `go/parser`
2. Extract method signatures and comments containing SQL/HTTP specifications where applicable
3. Use `DetectTargetDecl` to automatically detect generation mode (gen/tools.go:360-407)
   - `sqlx`: Detected by `EXEC`/`QUERY` comments or presence of `WithTx`
   - `api`: Detected by HTTP methods or presence of `Options()`/`ResponseHandler()`
   - `rpc`: Detected when interface methods have exactly 1 input and 2 outputs with the second being `error`
4. Build context with `CliBuilder` pattern containing imports, features, functions, and templates
5. Use Go templates (`sqlx.tmpl`, `api.tmpl`, `rpc.tmpl`) to generate implementation code
6. Apply Go formatting with `go/format` and import management with `goimports`
7. Write generated code to output file

### Key Features

- **Auto-detection**: Automatically detects generation mode from interface patterns
- **Template engine**: Built-in template support for dynamic SQL and HTTP requests
- **Feature flags**: Modular features like logging, caching, pagination
- **File inclusion**: Support for external SQL files and script execution
- **Transaction support**: Automatic transaction handling for database operations
- **RPC generation**: Generate net/rpc client/Server wrappers, with optional no-runtime mode via `rpc/nort` (gen/rpc.go:14-16; gen/template/rpc.tmpl)

## Important File Patterns

- Schema definitions: Go interface files with special method comments (sqlx/api) or RPC-style signatures (rpc)
- Generated files: Typically named `*.gen.go`
- Templates: Located in `gen/template/` with `.tmpl` extension
- Integration tests: Located in `gen/integration/` (includes `rpc/`)
- Test data: Located in `gen/testdata/`

## Development Notes

- The project uses Go 1.19+ features
- Enhanced sqlx fork provides additional interfaces over standard sqlx
- Since v1.37.0, `future` features are enabled by default
- Legacy mode available with `legacy` build tag
- Code generation is deterministic and includes proper error handling
- The `generate` command only accepts `.go` files as input (configuration file support was removed)

### Key Development Patterns

- **Builder Pattern**: The `CliBuilder` struct chains method calls like `WithFeats()`, `WithImports()`, `WithFuncs()`
- **Mode Detection**: Uses `DetectTargetDecl` to automatically determine if an interface is for `sqlx`, `api`, or `rpc` mode
- **Template Processing**: Comments in interface methods are parsed as templates for SQL queries or HTTP requests (sqlx/api)
- **AST Analysis**: Heavy use of Go's AST parsing to inspect interface definitions and method signatures
- **Feature Flags**: Modular architecture where features are enabled/disabled via string flags
- **Test Integration**: Integration tests in `gen/integration/` use real database connections, HTTP servers, and net/rpc loops

### Code Structure Patterns

- Generated code follows naming convention: `New{Interface}()` and `New{Interface}FromCore()` (sqlx/api)
- Interface methods with special names have semantic meaning:
  - `WithTx()`: Automatic transaction support in sqlx mode
  - `Options()`: Configuration provider in api mode
  - `ResponseHandler()`: Response processing in api mode
- Template functions like `bind`, `bindvars`, `getRepr` are available in SQL templates
- Test files use `runTest()` helper function for consistent integration testing
- RPC specifics:
  - Methods must have exactly 1 input parameter and 2 outputs, with the second being `error` (gen/rpc.go:35-49)
  - Generated client constructor: `New{Interface}(*rpc.Client) {Interface}`
  - Generated server wrapper: `New{Interface}Server(impl {Interface}) *{Interface}Server`
  - Feature `rpc/nort`: generate helpers to avoid depending on `runtime.New[T]` (see gen/template/rpc.tmpl)
