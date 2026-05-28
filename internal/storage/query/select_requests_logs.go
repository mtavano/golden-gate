package query

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/storage"
)

// SelectRequestLogsQuery represents query parameters for fetching request logs
type SelectRequestLogsQuery struct {
	ServiceName string    // Empty = all services
	From        time.Time // Zero = no lower bound
	To          time.Time // Zero = no upper bound
	Limit       int       // Default: 100
	Offset      int       // Default: 0
}

const selectRequestLogsColumns = `
	id,
	service_name,
	method,
	url,
	timestamp,
	headers,
	query_params,
	body,
	request_body_truncated,
	duration_ms,
	response_status_code,
	response_body,
	response_body_truncated,
	response_headers,
	created_at
`

// SelectRequestLogs retrieves request logs from database with pagination and optional filters.
// Default limit is 100 if not specified or if limit is 0.
func SelectRequestLogs(db storage.Database, q *SelectRequestLogsQuery) ([]*models.RequestLog, error) {
	if q.Limit <= 0 {
		q.Limit = 100
	}

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
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	sqlQuery := `SELECT ` + selectRequestLogsColumns + ` FROM request_logs ` + where + `
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?`

	args = append(args, q.Limit, q.Offset)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.RequestLog
	for rows.Next() {
		req, err := scanRequestLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanRequestLog(rows *sql.Rows) (*models.RequestLog, error) {
	var (
		req                     models.RequestLog
		serviceName             sql.NullString
		headersJSON             []byte
		queryParamsJSON         []byte
		responseHeadersJSON     []byte
		responseStatusCode      sql.NullInt64
		responseBody            []byte
		responseBodyTruncated   sql.NullBool
		requestBodyTruncated    sql.NullBool
		durationMs              sql.NullInt64
	)

	err := rows.Scan(
		&req.ID,
		&serviceName,
		&req.Method,
		&req.URL,
		&req.Timestamp,
		&headersJSON,
		&queryParamsJSON,
		&req.Body,
		&requestBodyTruncated,
		&durationMs,
		&responseStatusCode,
		&responseBody,
		&responseBodyTruncated,
		&responseHeadersJSON,
		&req.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if serviceName.Valid && serviceName.String != "" {
		req.ServiceName = serviceName.String
	} else {
		req.ServiceName = "unknown"
	}
	req.BodyTruncated = requestBodyTruncated.Bool
	req.DurationMs = durationMs.Int64

	if len(headersJSON) > 0 {
		if err := json.Unmarshal(headersJSON, &req.Headers); err != nil {
			return nil, err
		}
	}
	if len(queryParamsJSON) > 0 {
		if err := json.Unmarshal(queryParamsJSON, &req.Query); err != nil {
			return nil, err
		}
	}

	if responseStatusCode.Valid {
		req.Response = &models.ResponseLog{
			StatusCode:    int(responseStatusCode.Int64),
			Body:          responseBody,
			BodyTruncated: responseBodyTruncated.Bool,
		}
		if len(responseHeadersJSON) > 0 {
			if err := json.Unmarshal(responseHeadersJSON, &req.Response.Headers); err != nil {
				return nil, err
			}
		}
	}

	return &req, nil
}
