package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddServiceNameAndFlags, downAddServiceNameAndFlags)
}

func upAddServiceNameAndFlags(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE request_logs ADD COLUMN service_name TEXT`,
		`ALTER TABLE request_logs ADD COLUMN duration_ms INTEGER`,
		`ALTER TABLE request_logs ADD COLUMN request_body_truncated INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE request_logs ADD COLUMN response_body_truncated INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX idx_request_logs_service_timestamp ON request_logs(service_name, timestamp DESC)`,
	}

	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func downAddServiceNameAndFlags(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`DROP INDEX IF EXISTS idx_request_logs_service_timestamp`,
		`ALTER TABLE request_logs DROP COLUMN response_body_truncated`,
		`ALTER TABLE request_logs DROP COLUMN request_body_truncated`,
		`ALTER TABLE request_logs DROP COLUMN duration_ms`,
		`ALTER TABLE request_logs DROP COLUMN service_name`,
	}

	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
