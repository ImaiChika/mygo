package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SaveInput 是上传对象需要的元信息。
type SaveInput struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

// Object 是落盘后的对象描述。
type Object struct {
	StorageKey  string
	PublicURL   string
	ContentType string
}

// Store 代表可替换的附件存储实现。
type Store interface {
	Save(ctx context.Context, input SaveInput) (Object, error)
}

// LocalStore 以本地文件系统模拟对象存储，便于开发期快速落地。
type LocalStore struct {
	rootDir    string
	publicBase string
}

func NewLocalStore(rootDir string, publicBase string) *LocalStore {
	return &LocalStore{
		rootDir:    rootDir,
		publicBase: strings.TrimRight(publicBase, "/"),
	}
}

func (s *LocalStore) Save(_ context.Context, input SaveInput) (Object, error) {
	now := time.Now()
	ext := filepath.Ext(input.Filename)
	if ext == "" {
		if guessed, _ := mime.ExtensionsByType(input.ContentType); len(guessed) > 0 {
			ext = guessed[0]
		}
	}

	key := path.Join(
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%02d", now.Day()),
		uuid.NewString()+ext,
	)

	fullPath := filepath.Join(s.rootDir, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return Object{}, fmt.Errorf("创建上传目录失败: %w", err)
	}

	file, err := os.Create(filepath.Clean(fullPath))
	if err != nil {
		return Object{}, fmt.Errorf("创建上传文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, input.Reader); err != nil {
		return Object{}, fmt.Errorf("写入上传文件失败: %w", err)
	}

	publicURL := "/uploads/" + strings.ReplaceAll(key, string(filepath.Separator), "/")
	if s.publicBase != "" {
		publicURL = s.publicBase + publicURL
	}

	return Object{
		StorageKey:  key,
		PublicURL:   publicURL,
		ContentType: input.ContentType,
	}, nil
}
