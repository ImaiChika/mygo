package user

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	platformauth "mygo/internal/platform/auth"
	"mygo/internal/platform/httpx"
)

// HTTPHandler 暴露用户与登录接口。
type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
}

func (h *HTTPHandler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", h.Me)
	r.Get("/users/search", h.Search)
}

func (h *HTTPHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	created, token, err := h.service.Register(r.Context(), req)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"user":  created,
			"token": token,
		},
	})
}

func (h *HTTPHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	found, token, err := h.service.Login(r.Context(), req)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user":  found,
			"token": token,
		},
	})
}

func (h *HTTPHandler) Me(w http.ResponseWriter, r *http.Request) {
	current := platformauth.MustUser(r.Context())
	profile, err := h.service.GetProfile(r.Context(), current.ID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": profile,
	})
}

func (h *HTTPHandler) Search(w http.ResponseWriter, r *http.Request) {
	current := platformauth.MustUser(r.Context())
	query := r.URL.Query().Get("q")

	users, err := h.service.SearchUsers(r.Context(), query, current.ID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": users,
	})
}
