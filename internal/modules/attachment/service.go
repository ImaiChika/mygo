package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"mygo/internal/platform/storage"
)

// ConversationAccessChecker 抽象出聊天权限判断能力，避免与 chat 模块形成强耦合。
type ConversationAccessChecker interface {
	CanAccessConversation(ctx context.Context, conversationID uuid.UUID, userID string) (bool, error)
}

// Service 负责附件上传流程编排。
type Service struct {
	repo          Repository
	store         storage.Store
	accessChecker ConversationAccessChecker
	maxFileSize   int64
}

func NewService(repo Repository, store storage.Store, accessChecker ConversationAccessChecker, maxFileSize int64) *Service {
	return &Service{
		repo:          repo,
		store:         store,
		accessChecker: accessChecker,
		maxFileSize:   maxFileSize,
	}
}

type UploadInput struct {
	ConversationID uuid.UUID
	UserID         string
	Filename       string
	ContentType    string
	SizeBytes      int64
	Reader         io.Reader
}

func (s *Service) Upload(ctx context.Context, input UploadInput) (Attachment, error) {
	if input.ConversationID == uuid.Nil {
		return Attachment{}, errors.New("会话 ID 不能为空")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.Filename = strings.TrimSpace(input.Filename)
	input.ContentType = strings.TrimSpace(input.ContentType)

	if input.UserID == "" {
		return Attachment{}, errors.New("用户 ID 不能为空")
	}
	if input.Filename == "" {
		return Attachment{}, errors.New("文件名不能为空")
	}
	if input.Reader == nil {
		return Attachment{}, errors.New("上传内容不能为空")
	}
	if input.SizeBytes <= 0 {
		return Attachment{}, errors.New("文件大小必须大于 0")
	}
	if s.maxFileSize > 0 && input.SizeBytes > s.maxFileSize {
		return Attachment{}, fmt.Errorf("文件不能超过 %d 字节", s.maxFileSize)
	}

	allowed, err := s.accessChecker.CanAccessConversation(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return Attachment{}, fmt.Errorf("校验会话权限失败: %w", err)
	}
	if !allowed {
		return Attachment{}, errors.New("当前用户不属于该会话")
	}

	saved, err := s.store.Save(ctx, storage.SaveInput{
		Filename:    input.Filename,
		ContentType: input.ContentType,
		Reader:      input.Reader,
	})
	if err != nil {
		return Attachment{}, err
	}

	return s.repo.Create(ctx, Attachment{
		ID:             uuid.New(),
		ConversationID: input.ConversationID,
		UploadedBy:     input.UserID,
		OriginalName:   input.Filename,
		ContentType:    saved.ContentType,
		SizeBytes:      input.SizeBytes,
		StorageKey:     saved.StorageKey,
		PublicURL:      saved.PublicURL,
		CreatedAt:      time.Now(),
	})
}
