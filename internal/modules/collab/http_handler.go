package collab

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mygo/internal/platform/auth"
	"mygo/internal/platform/httpx"
)

// HTTPHandler 暴露协作模块接口。
type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Post("/collab/sessions", h.CreateSession)
	r.Get("/collab/rooms/{roomID}", h.DescribeRoom)
}

func (h *HTTPHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	var req struct {
		RoomID      string `json:"room_id"`
		ProviderURL string `json:"provider_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	roomID, err := uuid.Parse(req.RoomID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 room_id")
		return
	}

	ticket, err := h.service.IssueAccessTicket(r.Context(), CreateSessionInput{
		RoomID:      roomID,
		UserID:      user.ID,
		ProviderURL: req.ProviderURL,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": ticket,
	})
}

func (h *HTTPHandler) DescribeRoom(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 roomID")
		return
	}

	room := h.service.DescribeRoom(r.Context(), roomID)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": room,
	})
}
