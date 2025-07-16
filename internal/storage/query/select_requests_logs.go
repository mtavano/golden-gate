package query

import (
	"encoding/json"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/storage"
)

// RequestLogsQuery represents query parameters for fetching request logs
type SelectRequestLogsQuery struct {
	Limit  int // Default: 100
	Offset int // Default: 0
}

// SelectRequestLogs retrieves request logs from database with pagination
// Default limit is 100 if not specified or if limit is 0
func SelectRequestLogs(db storage.Database, query *SelectRequestLogsQuery) ([]*models.RequestLog, error) {
	// Set default limit if not specified
	if query.Limit <= 0 {
		query.Limit = 100
	}

	sqlQuery := `
		SELECT 
			id,
			method,
			url,
			timestamp,
			headers,
			query_params,
			body,
			response_status_code,
			response_body,
			response_headers,
			created_at
		FROM request_logs 
		ORDER BY timestamp DESC 
		LIMIT ? OFFSET ?
	`

	rows, err := db.Query(sqlQuery, query.Limit, query.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requestLogs []*models.RequestLog

	for rows.Next() {
		var req models.RequestLog
		var headersJSON, queryParamsJSON, responseHeadersJSON []byte
		var responseStatusCode *int
		var responseBody []byte

		err := rows.Scan(
			&req.ID,
			&req.Method,
			&req.URL,
			&req.Timestamp,
			&headersJSON,
			&queryParamsJSON,
			&req.Body,
			&responseStatusCode,
			&responseBody,
			&responseHeadersJSON,
			&req.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Deserialize JSON fields
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

		// Handle response if exists
		if responseStatusCode != nil {
			req.Response = &models.ResponseLog{
				StatusCode: *responseStatusCode,
				Body:       responseBody,
			}

			if len(responseHeadersJSON) > 0 {
				if err := json.Unmarshal(responseHeadersJSON, &req.Response.Headers); err != nil {
					return nil, err
				}
			}
		}

		requestLogs = append(requestLogs, &req)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requestLogs, nil
}
