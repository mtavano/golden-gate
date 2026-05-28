package query

import (
	"encoding/json"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/storage"
)

// InsertRequestLog inserts a RequestLog into the database using sql.Tx
// It handles JSON serialization of headers and query_params, and BLOB storage for body and response_body
func InsertRequestLog(tx storage.Transaction, req *models.RequestLog) error {
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return err
	}

	queryParamsJSON, err := json.Marshal(req.Query)
	if err != nil {
		return err
	}

	stmt := `
		INSERT INTO request_logs (
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var (
		responseStatusCode      *int
		responseBody            []byte
		responseBodyTruncated   bool
		responseHeadersJSON     []byte
	)

	if req.Response != nil {
		responseStatusCode = &req.Response.StatusCode
		responseBody = req.Response.Body
		responseBodyTruncated = req.Response.BodyTruncated

		responseHeadersJSON, err = json.Marshal(req.Response.Headers)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(
		stmt,
		req.ServiceName,
		req.Method,
		req.URL,
		req.Timestamp,
		headersJSON,
		queryParamsJSON,
		req.Body,
		req.BodyTruncated,
		req.DurationMs,
		responseStatusCode,
		responseBody,
		responseBodyTruncated,
		responseHeadersJSON,
		req.CreatedAt,
	)

	return err
}
