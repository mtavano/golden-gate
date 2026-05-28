package query

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/storage"
)

// CountRequestLogsQuery filters the count.
type CountRequestLogsQuery struct {
	ServiceName string    // Empty = all services
	From        time.Time // Zero = no lower bound
	To          time.Time // Zero = no upper bound
}

// CountRequestLogs returns the number of request logs matching the filters.
func CountRequestLogs(db storage.Database, q *CountRequestLogsQuery) (int64, error) {
	var conds []string
	var args []any

	if q.ServiceName != "" {
		conds = append(conds, "COALESCE(service_name, 'unknown') = ?")
		args = append(args, q.ServiceName)
	}
	if !q.From.IsZero() {
		conds = append(conds, "timestamp >= ?")
		args = append(args, q.From)
	}
	if !q.To.IsZero() {
		conds = append(conds, "timestamp < ?")
		args = append(args, q.To)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var n int64
	if err := db.Get(&n, "SELECT COUNT(*) FROM request_logs"+where, args...); err != nil {
		return 0, err
	}
	return n, nil
}

// ServiceStats summarizes activity for a single service.
type ServiceStats struct {
	ServiceName   string
	Count         int64
	LastRequestAt *time.Time
}

// CountByService returns aggregated stats keyed by service_name.
// Rows with NULL service_name are coalesced into "unknown".
func CountByService(db storage.Database) (map[string]ServiceStats, error) {
	rows, err := db.Query(`
		SELECT COALESCE(service_name, 'unknown') AS svc,
		       COUNT(*) AS count,
		       MAX(timestamp) AS last_at
		FROM request_logs
		GROUP BY svc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]ServiceStats)
	for rows.Next() {
		var s ServiceStats
		var lastAtRaw sql.NullString
		if err := rows.Scan(&s.ServiceName, &s.Count, &lastAtRaw); err != nil {
			return nil, err
		}
		if lastAtRaw.Valid && lastAtRaw.String != "" {
			if t, perr := parseSQLiteTime(lastAtRaw.String); perr == nil {
				s.LastRequestAt = &t
			}
		}
		out[s.ServiceName] = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseSQLiteTime parses the timestamp formats returned by modernc.org/sqlite.
func parseSQLiteTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized sqlite time format: %q", s)
}
