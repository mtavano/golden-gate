package models

import (
	"time"
)

type RequestLog struct {
	ID        string
	Timestamp time.Time
	Method    string
	URL       string
	Headers   map[string][]string
	Query     map[string][]string
	Body      []byte
	Response  *ResponseLog
}

type ResponseLog struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type RequestStore struct {
	requests []*RequestLog
	maxSize  int
}

func NewRequestStore(maxSize int) *RequestStore {
	return &RequestStore{
		requests: make([]*RequestLog, 0),
		maxSize:  maxSize,
	}
}

func (rs *RequestStore) AddRequest(req *RequestLog) {
	if len(rs.requests) >= rs.maxSize {
		rs.requests = rs.requests[1:]
	}
	rs.requests = append(rs.requests, req)
}

func (rs *RequestStore) GetRequests() []*RequestLog {
	return rs.requests
} 