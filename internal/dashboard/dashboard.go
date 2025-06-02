package dashboard

import (
	"net/http"

	"github.com/mtavano/golden-gate/internal/dashboard/views"
	"github.com/mtavano/golden-gate/internal/types"
)

type Handler struct {
	requestStore *types.RequestStore
}

func NewHandler(requestStore *types.RequestStore) *Handler {
	return &Handler{
		requestStore: requestStore,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requests := h.requestStore.GetRequests()
	views.Dashboard(requests).Render(r.Context(), w)
} 