package query

import (
	"encoding/json"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/storage"
)

// InsertRequestLog inserts a RequestLog into the database using sql.Tx
// It handles JSON serialization of headers and query_params, and BLOB storage for body and response_body
func InsertRequestLog(tx storage.Transaction, req *models.RequestLog) error {
	// Serialize headers to JSON
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return err
	}

	// Serialize query parameters to JSON
	queryParamsJSON, err := json.Marshal(req.Query)
	if err != nil {
		return err
	}

	// Prepare the SQL statement
	query := `
		INSERT INTO request_logs (
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
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Prepare response data
	var responseStatusCode *int
	var responseBody []byte
	var responseHeadersJSON []byte

	if req.Response != nil {
		responseStatusCode = &req.Response.StatusCode
		responseBody = req.Response.Body

		// Serialize response headers to JSON
		responseHeadersJSON, err = json.Marshal(req.Response.Headers)
		if err != nil {
			return err
		}
	}

	// Execute the insert
	_, err = tx.Exec(
		query,
		req.Method,
		req.URL,
		req.Timestamp,
		headersJSON,
		queryParamsJSON,
		req.Body,
		responseStatusCode,
		responseBody,
		responseHeadersJSON,
		req.CreatedAt,
	)

	return err
}
