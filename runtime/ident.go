package defc

import (
	"fmt"
	"strings"
)

// SQLDialect selects the identifier-quoting convention for
// [QuoteIdentifier].
type SQLDialect int

const (
	// DialectMySQL quotes identifiers with backticks and escapes a
	// backtick inside the value by doubling it. Interior NUL is
	// rejected.
	DialectMySQL SQLDialect = iota
	// DialectPostgres quotes identifiers with double quotes and
	// escapes an embedded double quote by doubling it.
	DialectPostgres
	// DialectSQLite follows the Postgres convention.
	DialectSQLite
)

// InvalidIdentifierError signals that a runtime value is not safe to
// inline as a SQL identifier under the selected dialect.
type InvalidIdentifierError struct {
	Dialect SQLDialect
	Detail  string
}

func (e *InvalidIdentifierError) Error() string {
	return fmt.Sprintf("defc: invalid SQL identifier: %s", e.Detail)
}

func (e *InvalidIdentifierError) Unwrap() error { return ErrUnsafeInterpolation }

// QuoteIdentifier returns the dialect-escaped, quoted form of v. The
// result is suitable for direct inlining into a SQL string.
func QuoteIdentifier(v string, dialect SQLDialect) (string, error) {
	if strings.ContainsRune(v, 0) {
		return "", &InvalidIdentifierError{Dialect: dialect, Detail: "contains NUL"}
	}
	switch dialect {
	case DialectMySQL:
		return "`" + strings.ReplaceAll(v, "`", "``") + "`", nil
	case DialectPostgres, DialectSQLite:
		return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`, nil
	default:
		return "", &InvalidIdentifierError{Dialect: dialect, Detail: fmt.Sprintf("unknown dialect %d", int(dialect))}
	}
}
