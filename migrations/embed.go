package migrations

import "embed"

// EmbeddedFS bundles the migrations package contents so goose has a base FS
// regardless of the runtime CWD. The actual migration logic is registered via
// goose.AddMigrationContext in each migration's init(); the embedded files
// are only used to satisfy goose's directory check.
//
//go:embed *.go
var EmbeddedFS embed.FS
