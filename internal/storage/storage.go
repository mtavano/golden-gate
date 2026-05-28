package storage

import (
	"context"
	"database/sql"
)

// Minimal interfaces to work with sqlx-backed databases.

type QueryContext interface {
	Exec(query string, params ...any) (sql.Result, error)
	Query(query string, params ...any) (*sql.Rows, error)
}

type Transaction interface {
	Get(dest any, query string, args ...any) error
	Select(dest any, query string, args ...any) error
	QueryContext
}

type Transactioner interface {
	Transaction
	Commit() error
	Rollback() error
}

type Database interface {
	BeginTx(context.Context) (Transactioner, error)
	Transaction
}
