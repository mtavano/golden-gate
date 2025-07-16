package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInitialize, downInitialize)
}

func upInitialize(ctx context.Context, tx *sql.Tx) error {
	// Create request_logs table
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			method VARCHAR(10) NOT NULL,
			url TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			headers TEXT,
			query_params TEXT,
			body BLOB,
			response_status_code INTEGER,
			response_body BLOB,
			response_headers TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes for better query performance
	_, err = tx.ExecContext(ctx, `CREATE INDEX idx_request_logs_timestamp ON request_logs(timestamp)`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `CREATE INDEX idx_request_logs_method ON request_logs(method)`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `CREATE INDEX idx_request_logs_url ON request_logs(url)`)
	if err != nil {
		return err
	}

	return nil
}

func downInitialize(ctx context.Context, tx *sql.Tx) error {
	// Drop indexes first
	_, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_request_logs_url`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_request_logs_method`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_request_logs_timestamp`)
	if err != nil {
		return err
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS request_logs`)
	return err
}
