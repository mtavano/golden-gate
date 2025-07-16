package models

import (
	"time"
)

// RequestLog represents a logged HTTP request with its response
type RequestLog struct {
	ID        int                 `db:"id" json:"id"`
	Method    string              `db:"method" json:"method"`
	URL       string              `db:"url" json:"url"`
	Timestamp time.Time           `db:"timestamp" json:"timestamp"`
	Headers   map[string][]string `db:"headers" json:"headers"`           // JSON serialized
	Query     map[string][]string `db:"query_params" json:"query_params"` // JSON serialized
	Body      []byte              `db:"body" json:"body"`
	Response  *ResponseLog        `json:"response,omitempty"`
	CreatedAt time.Time           `db:"created_at" json:"created_at"`
}

// ResponseLog represents the HTTP response
type ResponseLog struct {
	StatusCode int                 `db:"response_status_code" json:"status_code"`
	Headers    map[string][]string `db:"response_headers" json:"headers"` // JSON serialized
	Body       []byte              `db:"response_body" json:"body"`
}
