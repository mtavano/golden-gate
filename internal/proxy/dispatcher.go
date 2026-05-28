package proxy

import (
	"net/http"
	"sort"
	"strings"
	"sync/atomic"

	internalConfig "github.com/mtavano/golden-gate/internal/config"
	"github.com/mtavano/golden-gate/internal/service"
	"go.uber.org/zap"
)

// Dispatcher is the catch-all HTTP handler that routes incoming requests to a
// per-service Proxy based on path prefix. The active set of proxies is held
// behind an atomic pointer so updates are non-blocking against readers.
type Dispatcher struct {
	requestSvc   *service.RequestSvc
	maxBodyBytes int
	logger       *zap.Logger

	snapshot atomic.Pointer[snapshot]
}

// snapshot holds the immutable routing table for one configuration version.
// Entries are sorted by descending BasePrefix length so the longest prefix
// wins (e.g. /api/v1/buda beats /api/v1).
type snapshot struct {
	entries []entry
}

type entry struct {
	prefix  string
	handler *Proxy
}

// NewDispatcher returns a dispatcher with an empty routing table. Call Update
// (or wire it through ConfigManager.Subscribe) to populate routes.
func NewDispatcher(requestSvc *service.RequestSvc, maxBodyBytes int, logger *zap.Logger) *Dispatcher {
	d := &Dispatcher{
		requestSvc:   requestSvc,
		maxBodyBytes: maxBodyBytes,
		logger:       logger,
	}
	d.snapshot.Store(&snapshot{})
	return d
}

// Update rebuilds the routing snapshot from the given service configuration.
// Validation already happened in config.Validate, so this is a pure
// transformation. Old proxies remain reachable by any in-flight goroutine
// that captured them; new requests use the freshly built snapshot.
func (d *Dispatcher) Update(cfg *internalConfig.Config) {
	if cfg == nil {
		d.snapshot.Store(&snapshot{})
		return
	}

	entries := make([]entry, 0, len(cfg.Services))
	for name, sc := range cfg.Services {
		p := NewProxy(&Config{
			ServiceName:  name,
			BasePrefix:   sc.BasePrefix,
			Target:       sc.Target,
			MaxBodyBytes: d.maxBodyBytes,
		}, d.requestSvc)
		entries = append(entries, entry{
			prefix:  sc.BasePrefix,
			handler: p,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		// Longest prefix first so /a/b matches before /a.
		return len(entries[i].prefix) > len(entries[j].prefix)
	})

	d.snapshot.Store(&snapshot{entries: entries})
	d.logger.Info("dispatcher snapshot updated", zap.Int("services", len(entries)))
}

// ServeHTTP walks the current snapshot looking for the longest matching
// BasePrefix. If none match, it returns 404.
func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snap := d.snapshot.Load()
	for _, e := range snap.entries {
		if matchesPrefix(r.URL.Path, e.prefix) {
			e.handler.ServeHTTP(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

// matchesPrefix returns true if path == prefix or path == prefix + "/...".
// This avoids /budget matching /buda when a user mis-orders prefixes.
func matchesPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if len(path) == len(prefix) {
		return true
	}
	return path[len(prefix)] == '/'
}
