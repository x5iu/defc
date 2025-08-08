# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`defc` is a Go code generation tool that automatically generates CRUD operations for databases and HTTP client code
based on predefined schemas. The project focuses on eliminating repetitive boilerplate code by defining interfaces and
automatically generating implementation code.

## Development Commands

### Testing

```bash
# Run all tests with coverage
./test.sh

# Run specific module tests
go test -cover ./gen
go test -cover ./runtime  
go test -cover ./sqlx
go test -tags=test ./gen/integration

# Code quality checks
go vet ./...
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
defc --mode=api --output=service.go
```

## Architecture Overview

### Core Components

1. **Main CLI (`main.go`)**: Command-line interface built with Cobra, handles argument parsing and delegates to
   generation logic
2. **Generator (`gen/` package)**: Core code generation engine with two main modes:
    - `sqlx` mode: Database CRUD operations using enhanced sqlx
    - `api` mode: HTTP client code generation
3. **Runtime (`runtime/` package)**: Runtime utilities and helpers for generated code
4. **Templates (`gen/template/`)**: Go templates for code generation

### Code Generation Flow

1. Parse Go interface definitions from source files
2. Extract method signatures and comments containing SQL or HTTP specifications
3. Use templates to generate implementation code
4. Apply Go formatting and import management
5. Write generated code to output file

### Key Features

- **Auto-detection**: Automatically detects generation mode from interface patterns
- **Template engine**: Built-in template support for dynamic SQL and HTTP requests
- **Feature flags**: Modular features like logging, caching, pagination
- **File inclusion**: Support for external SQL files and script execution
- **Transaction support**: Automatic transaction handling for database operations

## Important File Patterns

- Schema definitions: Go interface files with special method comments
- Generated files: Typically named `*.gen.go`
- Templates: Located in `gen/template/` with `.tmpl` extension
- Integration tests: Located in `gen/integration/`
- Test data: Located in `gen/testdata/`

## Development Notes

- The project uses Go 1.19+ features
- Enhanced sqlx fork provides additional interfaces over standard sqlx
- Since v1.37.0, `future` features are enabled by default
- Legacy mode available with `legacy` build tag
- Code generation is deterministic and includes proper error handling
- The `generate` command only accepts `.go` files as input (configuration file support was removed)