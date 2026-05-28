package service

import (
	"context"
	"time"

	"github.com/mtavano/golden-gate/internal/models"
	"github.com/mtavano/golden-gate/internal/storage"
	"github.com/mtavano/golden-gate/internal/storage/query"
)

// RequestSvc manages storage of request logs using database
type RequestSvc struct {
	db storage.Database
}

func NewRequestSvc(db storage.Database) *RequestSvc {
	return &RequestSvc{
		db: db,
	}
}

// AddRequest inserts a request log into the database
func (rs *RequestSvc) AddRequest(req *models.RequestLog) error {
	ctx := context.Background()
	tx, err := rs.db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := query.InsertRequestLog(tx, req); err != nil {
		return err
	}

	return tx.Commit()
}

// GetRequests retrieves request logs from database with default pagination (100 items)
func (rs *RequestSvc) GetRequests() ([]*models.RequestLog, error) {
	return rs.GetRequestsWithPagination(100, 0)
}

// GetRequestsWithPagination retrieves request logs from database with custom pagination
func (rs *RequestSvc) GetRequestsWithPagination(limit, offset int) ([]*models.RequestLog, error) {
	return query.SelectRequestLogs(rs.db, &query.SelectRequestLogsQuery{
		Limit:  limit,
		Offset: offset,
	})
}

// GetRequestsByService retrieves request logs filtered by service and optional time range.
func (rs *RequestSvc) GetRequestsByService(serviceName string, from, to time.Time, limit, offset int) ([]*models.RequestLog, error) {
	return query.SelectRequestLogs(rs.db, &query.SelectRequestLogsQuery{
		ServiceName: serviceName,
		From:        from,
		To:          to,
		Limit:       limit,
		Offset:      offset,
	})
}

// CountByService returns total count and last activity timestamp per service.
func (rs *RequestSvc) CountByService() (map[string]query.ServiceStats, error) {
	return query.CountByService(rs.db)
}

// CountRequestsByService returns the number of requests for a service within an optional range.
func (rs *RequestSvc) CountRequestsByService(serviceName string, from, to time.Time) (int64, error) {
	return query.CountRequestLogs(rs.db, &query.CountRequestLogsQuery{
		ServiceName: serviceName,
		From:        from,
		To:          to,
	})
}
