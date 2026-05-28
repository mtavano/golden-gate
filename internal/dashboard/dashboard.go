package dashboard

import (
	"net/http"
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
