package chat

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mygo/internal/platform/auth"
	"mygo/internal/platform/httpx"
)

// HTTPHandler 暴露聊天模块的 REST API。
type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Get("/conversations", h.ListConversations)
	r.Post("/conversations", h.CreateConversation)
	r.Get("/conversations/{conversationID}/members", h.ListMembers)
	r.Post("/conversations/{conversationID}/members", h.AddMembers)
	r.Get("/conversations/{conversationID}/messages", h.ListMessages)
	r.Post("/conversations/{conversationID}/messages", h.SendMessage)
	r.Post("/conversations/{conversationID}/read", h.MarkRead)
}

func (h *HTTPHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	conversations, err := h.service.ListConversations(r.Context(), user.ID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": conversations,
	})
}

func (h *HTTPHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	var req struct {
		Name      string   `json:"name"`
		Kind      string   `json:"kind"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	conversation, err := h.service.CreateConversation(r.Context(), CreateConversationInput{
		Name:      req.Name,
		Kind:      req.Kind,
		MemberIDs: req.MemberIDs,
		OwnerID:   user.ID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": conversation,
	})
}

func (h *HTTPHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			httpx.Error(w, http.StatusBadRequest, "limit 必须是整数")
			return
		}
		limit = parsed
	}

	messages, err := h.service.ListMessages(r.Context(), ListMessagesQuery{
		ConversationID: conversationID,
		UserID:         user.ID,
		Limit:          limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": messages,
	})
}

func (h *HTTPHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	var req struct {
		Kind     string          `json:"kind"`
		Content  string          `json:"content"`
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	message, err := h.service.SendMessage(r.Context(), SendMessageInput{
		ConversationID: conversationID,
		SenderID:       user.ID,
		Kind:           req.Kind,
		Content:        req.Content,
		Metadata:       req.Metadata,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": message,
	})
}

func (h *HTTPHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	members, err := h.service.ListConversationMembers(r.Context(), conversationID, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": members,
	})
}

func (h *HTTPHandler) AddMembers(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	var req struct {
		Members []ConversationMemberInput `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	members, err := h.service.AddConversationMembers(r.Context(), AddConversationMembersInput{
		ConversationID: conversationID,
		OperatorID:     user.ID,
		Members:        req.Members,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": members,
	})
}

func (h *HTTPHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	var req struct {
		LastReadMessageID string `json:"last_read_message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	var messageID *uuid.UUID
	if req.LastReadMessageID != "" {
		parsed, err := uuid.Parse(req.LastReadMessageID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "last_read_message_id 无效")
			return
		}
		messageID = &parsed
	}

	state, err := h.service.MarkConversationRead(r.Context(), MarkConversationReadInput{
		ConversationID:    conversationID,
		UserID:            user.ID,
		LastReadMessageID: messageID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": state,
	})
}
