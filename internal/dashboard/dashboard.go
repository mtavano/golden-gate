package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	internalConfig "github.com/mtavano/golden-gate/internal/config"
	"github.com/mtavano/golden-gate/internal/dashboard/views"
	"github.com/mtavano/golden-gate/internal/service"
)

const (
	defaultPageSize    = 50
	defaultRangeHours  = 24
	defaultDateLayout  = "2006-01-02T15:04"
	fallbackTimeZone   = "America/Santiago"
)

type Handler struct {
	requestSvc *service.RequestSvc
	cfgMgr     *internalConfig.Manager
	loc        *time.Location
	configPath string
}

func NewHandler(requestSvc *service.RequestSvc, cfgMgr *internalConfig.Manager) *Handler {
	loc, err := time.LoadLocation(fallbackTimeZone)
	if err != nil {
		loc = time.UTC
	}
	return &Handler{
		requestSvc: requestSvc,
		cfgMgr:     cfgMgr,
		loc:        loc,
	}
}

// SetLocation overrides the timezone used for display. Returns the handler to
// allow chaining from main.
func (h *Handler) SetLocation(tz string) *Handler {
	if tz == "" {
		return h
	}
	loc, err := time.LoadLocation(tz)
	if err == nil {
		h.loc = loc
	}
	return h
}

// WithConfigPath enables the browser editor by pointing it at the absolute
// path of the service.json file on disk. Returns the handler to allow
// chaining from main.
func (h *Handler) WithConfigPath(path string) *Handler {
	h.configPath = path
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stats, err := h.requestSvc.CountByService()
	if err != nil {
		http.Error(w, "Error fetching stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cards := make([]views.ServiceCard, 0, len(h.cfgMgr.Current().Services)+len(stats))
	for name, svc := range h.cfgMgr.Current().Services {
		s := stats[name]
		cards = append(cards, views.ServiceCard{
			Name:          name,
			BasePrefix:    svc.BasePrefix,
			Target:        svc.Target,
			Count:         s.Count,
			LastRequestAt: s.LastRequestAt,
		})
		delete(stats, name)
	}

	// Anything left in stats has data but no matching service in the current
	// configs/service.json — show as an orphan card so the count is visible.
	for name, s := range stats {
		cards = append(cards, views.ServiceCard{
			Name:          name,
			Count:         s.Count,
			LastRequestAt: s.LastRequestAt,
			Orphan:        true,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Orphan != cards[j].Orphan {
			return !cards[i].Orphan
		}
		return cards[i].Name < cards[j].Name
	})

	views.Dashboard(cards, h.loc).Render(r.Context(), w)
}

// ExploreHandler returns the handler for /dashboard/services/{name}.
func (h *Handler) ExploreHandler() http.Handler {
	return http.HandlerFunc(h.explore)
}

func (h *Handler) explore(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	svc, configured := h.cfgMgr.Current().Services[name]
	if !configured && name != "unknown" {
		http.NotFound(w, r)
		return
	}

	now := time.Now().In(h.loc)
	to := h.parseLocalTime(r.URL.Query().Get("to"), now)
	from := h.parseLocalTime(r.URL.Query().Get("from"), to.Add(-defaultRangeHours*time.Hour))

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	fromUTC := from.UTC()
	toUTC := to.UTC()
	offset := (page - 1) * defaultPageSize

	requests, err := h.requestSvc.GetRequestsByService(name, fromUTC, toUTC, defaultPageSize+1, offset)
	if err != nil {
		http.Error(w, "Error fetching requests: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasNext := len(requests) > defaultPageSize
	if hasNext {
		requests = requests[:defaultPageSize]
	}

	count, err := h.requestSvc.CountRequestsByService(name, fromUTC, toUTC)
	if err != nil {
		http.Error(w, "Error counting requests: "+err.Error(), http.StatusInternalServerError)
		return
	}

	view := views.ExploreView{
		ServiceName: name,
		BasePrefix:  svc.BasePrefix,
		Target:      svc.Target,
		Requests:    requests,
		Count:       count,
		From:        from,
		To:          to,
		Page:        page,
		PageSize:    defaultPageSize,
		HasNext:     hasNext,
		Location:    h.loc,
	}
	views.Explore(view).Render(r.Context(), w)
}

// parseLocalTime parses a "datetime-local" input string in the handler's
// timezone. Returns fallback if parsing fails.
func (h *Handler) parseLocalTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.ParseInLocation(defaultDateLayout, s, h.loc)
	if err != nil {
		return fallback
	}
	return t
}

// EditorGetHandler renders the browser editor for service.json at
// /dashboard/config. Returns 503 if the editor was not enabled via
// WithConfigPath.
func (h *Handler) EditorGetHandler() http.Handler {
	return http.HandlerFunc(h.editorGet)
}

// EditorPostHandler accepts a POST to /dashboard/config with the raw JSON
// content in the "content" form field, validates it, and writes it
// atomically. Hot reload is triggered by the existing fsnotify watcher.
func (h *Handler) EditorPostHandler() http.Handler {
	return http.HandlerFunc(h.editorPost)
}

func (h *Handler) editorGet(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		http.Error(w, "editor disabled (config path not set)", http.StatusServiceUnavailable)
		return
	}

	var content string
	raw, err := os.ReadFile(h.configPath)
	switch {
	case err == nil:
		var pretty bytes.Buffer
		if jerr := json.Indent(&pretty, raw, "", "  "); jerr == nil {
			content = pretty.String()
		} else {
			content = string(raw)
		}
	case os.IsNotExist(err):
		content = ""
	default:
		http.Error(w, "Error reading config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	view := views.EditorView{
		Path:         h.configPath,
		Content:      content,
		Success:      r.URL.Query().Get("ok") == "1",
		ServiceCount: len(h.cfgMgr.Current().Services),
	}
	views.Editor(view).Render(r.Context(), w)
}

func (h *Handler) editorPost(w http.ResponseWriter, r *http.Request) {
	if h.configPath == "" {
		http.Error(w, "editor disabled (config path not set)", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}
	content := r.FormValue("content")

	renderError := func(msg string) {
		view := views.EditorView{
			Path:         h.configPath,
			Content:      content,
			Error:        msg,
			ServiceCount: len(h.cfgMgr.Current().Services),
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.Editor(view).Render(r.Context(), w)
	}

	var services map[string]internalConfig.ServiceConfig
	if err := json.Unmarshal([]byte(content), &services); err != nil {
		renderError("JSON inválido: " + err.Error())
		return
	}

	cfg := &internalConfig.Config{Services: services}
	if err := internalConfig.Validate(cfg); err != nil {
		renderError("Configuración inválida: " + err.Error())
		return
	}

	dir := filepath.Dir(h.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "Error creating directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".service.json.tmp.%d", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		os.Remove(tmp)
		http.Error(w, "Error writing temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp, h.configPath); err != nil {
		os.Remove(tmp)
		http.Error(w, "Error replacing config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/config?ok=1", http.StatusSeeOther)
}
