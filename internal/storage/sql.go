package storage

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	_ "modernc.org/sqlite"
)

// SqlStore is the database wrapper
type SqlStore struct {
	*sqlx.DB
}

var _ Database = &SqlStore{}

func NewSqlStore(driver, dsn string) (*SqlStore, error) {
	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	// SQLite supports a single writer; force a serial pool so PRAGMAs apply
	// consistently and writers don't fight for the same file.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if driver == "sqlite" || driver == "sqlite3" {
		pragmas := []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=5000",
			"PRAGMA synchronous=NORMAL",
			"PRAGMA foreign_keys=ON",
		}
		for _, p := range pragmas {
			if _, err := db.Exec(p); err != nil {
				return nil, errors.Wrapf(err, "storage: failed to apply %q", p)
			}
		}
	}

	return &SqlStore{db}, nil
}

func (st *SqlStore) BeginTx(ctx context.Context) (Transactioner, error) {
	tx, err := st.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "storage: SqlStore.BeginTx error")
	}
	return tx, nil
}
