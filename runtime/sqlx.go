package defc

import "github.com/x5iu/defc/sqlx"

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
	DB  = sqlx.DB
	Tx  = sqlx.Tx
	Row = sqlx.IRow
)
