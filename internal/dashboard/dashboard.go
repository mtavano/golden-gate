package dashboard

import (
	"net/http"

	"github.com/mtavano/golden-gate/internal/dashboard/views"
	"github.com/mtavano/golden-gate/internal/service"
)

type Handler struct {
	requestSvc *service.RequestSvc
}

func NewHandler(requestSvc *service.RequestSvc) *Handler {
	return &Handler{
		requestSvc: requestSvc,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requests, err := h.requestSvc.GetRequests()
	if err != nil {
		http.Error(w, "Error fetching requests: "+err.Error(), http.StatusInternalServerError)
		return
	}
	views.Dashboard(requests).Render(r.Context(), w)
}
