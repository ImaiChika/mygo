package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	platformauth "mygo/internal/platform/auth"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]{3,32}$`)

// Service 负责用户注册、登录与资料查询。
type Service struct {
	repo   Repository
	issuer *platformauth.Issuer
}

func NewService(repo Repository, issuer *platformauth.Issuer) *Service {
	return &Service{
		repo:   repo,
		issuer: issuer,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, platformauth.Token, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)

	if !strings.Contains(input.Email, "@") {
		return User{}, platformauth.Token{}, errors.New("邮箱格式不正确")
	}
	if !usernamePattern.MatchString(input.Username) {
		return User{}, platformauth.Token{}, errors.New("用户名仅支持 3-32 位字母、数字、下划线或中划线")
	}
	if len(input.Password) < 6 {
		return User{}, platformauth.Token{}, errors.New("密码至少需要 6 位")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, platformauth.Token{}, fmt.Errorf("生成密码散列失败: %w", err)
	}

	created, err := s.repo.CreateUser(ctx, input, string(passwordHash))
	if err != nil {
		return User{}, platformauth.Token{}, err
	}

	token, err := s.issuer.Issue(platformauth.User{
		ID:          created.ID,
		Email:       created.Email,
		Username:    created.Username,
		DisplayName: created.DisplayName,
	})
	if err != nil {
		return User{}, platformauth.Token{}, err
	}

	return created, token, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, platformauth.Token, error) {
	input.EmailOrUsername = strings.ToLower(strings.TrimSpace(input.EmailOrUsername))

	if input.EmailOrUsername == "" || strings.TrimSpace(input.Password) == "" {
		return User{}, platformauth.Token{}, errors.New("账号和密码不能为空")
	}

	credentials, err := s.repo.GetUserByEmailOrUsername(ctx, input.EmailOrUsername)
	if err != nil {
		return User{}, platformauth.Token{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(input.Password)); err != nil {
		return User{}, platformauth.Token{}, errors.New("账号或密码错误")
	}

	token, err := s.issuer.Issue(platformauth.User{
		ID:          credentials.ID,
		Email:       credentials.Email,
		Username:    credentials.Username,
		DisplayName: credentials.DisplayName,
	})
	if err != nil {
		return User{}, platformauth.Token{}, err
	}

	return credentials.User, token, nil
}

func (s *Service) GetProfile(ctx context.Context, userID string) (User, error) {
	if strings.TrimSpace(userID) == "" {
		return User{}, errors.New("用户 ID 不能为空")
	}
	return s.repo.GetUserByID(ctx, userID)
}

func (s *Service) SearchUsers(ctx context.Context, query string, excludeUserID string) ([]User, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return nil, errors.New("搜索关键字至少需要 2 个字符")
	}
	return s.repo.SearchUsers(ctx, query, excludeUserID, 10)
}
