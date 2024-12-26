package defc

import (
	"context"
	"database/sql"

	"github.com/x5iu/defc/sqlx"
)

var (
	Open           = sqlx.Open
	MustOpen       = sqlx.MustOpen
	Connect        = sqlx.Connect
	ConnectContext = sqlx.ConnectContext
	MustConnect    = sqlx.MustConnect
	NewDB          = sqlx.NewDB

	// There is no need to import In from sqlx package, since sqlx.In references defc.In
	/*
		In = sqlx.In
	*/

	Named      = sqlx.Named
	StructScan = sqlx.StructScan
	ScanStruct = sqlx.StructScan
	ScanRow    = sqlx.ScanRow
)

type (
	DB       = sqlx.DB
	Tx       = sqlx.Tx
	Row      = sqlx.IRow
	Rows     = sqlx.IRows
	FromRow  = sqlx.FromRow
	FromRows = sqlx.FromRows
)

type TxInterface interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	Rollback() error
	Commit() error
}

type TxRebindInterface interface {
	TxInterface
	Rebind(query string) string
}
