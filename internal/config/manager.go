package config

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Manager owns the live service configuration. It watches the source file for
// changes and atomically swaps a validated snapshot in front of readers.
// Subscribers are notified after every successful reload.
type Manager struct {
	path    string
	logger  *zap.Logger
	current atomic.Pointer[Config]

	mu          sync.Mutex
	subscribers []func(*Config)
}

// NewManager loads the config from path (bootstrapping the embedded default
// if missing) and returns a Manager seeded with that snapshot.
func NewManager(path string, logger *zap.Logger) (*Manager, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		path:   path,
		logger: logger,
	}
	m.current.Store(cfg)
	return m, nil
}

// Current returns the latest validated snapshot. It is safe for concurrent
// reads with zero contention (atomic load).
func (m *Manager) Current() *Config {
	return m.current.Load()
}

// Subscribe registers a callback invoked after every successful reload. The
// callback runs on the watcher goroutine and must not block long. Subscribers
// added after Watch starts also fire on subsequent reloads.
func (m *Manager) Subscribe(fn func(*Config)) {
	m.mu.Lock()
	m.subscribers = append(m.subscribers, fn)
	m.mu.Unlock()
	// Fire once with the current snapshot so the subscriber initializes its
	// own derived state without a duplicate code path in callers.
	fn(m.Current())
}

// Watch blocks until ctx is cancelled, watching the directory that contains
// the config file. Atomic editor saves (rename) and direct writes both
// trigger a debounced reload.
func (m *Manager) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	dir := filepath.Dir(m.path)
	base := filepath.Base(m.path)
	if err := watcher.Add(dir); err != nil {
		return err
	}

	m.logger.Info("config watcher started",
		zap.String("path", m.path),
		zap.String("watch_dir", dir),
	)

	var (
		debounce  *time.Timer
		debounceM sync.Mutex
	)
	trigger := func() {
		debounceM.Lock()
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(300*time.Millisecond, m.reload)
		debounceM.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Base(ev.Name) != base {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				trigger()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			m.logger.Warn("config watcher error", zap.Error(err))
		}
	}
}

// reload re-reads the config file, validates it, and swaps the snapshot. On
// failure the previous snapshot is kept and the error is logged — the app
// keeps serving with the last-known-good config.
func (m *Manager) reload() {
	cfg, err := LoadConfig(m.path)
	if err != nil {
		m.logger.Error("config reload failed; keeping previous snapshot",
			zap.String("path", m.path),
			zap.Error(err),
		)
		return
	}
	old := m.current.Swap(cfg)
	m.logger.Info("config reloaded",
		zap.Int("previous_services", countServices(old)),
		zap.Int("new_services", len(cfg.Services)),
	)

	m.mu.Lock()
	subs := append([]func(*Config){}, m.subscribers...)
	m.mu.Unlock()
	for _, fn := range subs {
		fn(cfg)
	}
}

func countServices(c *Config) int {
	if c == nil {
		return 0
	}
	return len(c.Services)
}
