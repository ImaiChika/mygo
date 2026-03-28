package attachment

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mygo/internal/platform/auth"
	"mygo/internal/platform/httpx"
)

// HTTPHandler 暴露附件上传接口。
type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Post("/conversations/{conversationID}/attachments", h.Upload)
}

func (h *HTTPHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())
	conversationID, err := uuid.Parse(chi.URLParam(r, "conversationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "无效的 conversationID")
		return
	}

	// 使用 MaxBytesReader 保护上传接口，避免大文件直接撑爆服务内存。
	r.Body = http.MaxBytesReader(w, r.Body, h.service.maxFileSize+1024)
	if err := r.ParseMultipartForm(h.service.maxFileSize + 1024); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "上传文件过大，不能超过 10MB")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "解析上传表单失败")
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "缺少 file 文件字段")
		return
	}
	defer file.Close()

	uploaded, err := h.service.Upload(r.Context(), UploadInput{
		ConversationID: conversationID,
		UserID:         user.ID,
		Filename:       fileHeader.Filename,
		ContentType:    fileHeader.Header.Get("Content-Type"),
		SizeBytes:      fileHeader.Size,
		Reader:         file,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"data": uploaded,
	})
}
