package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"mygo/internal/platform/httpx"
)

type contextKey string

const userContextKey contextKey = "user"

// User 是当前鉴权通过后的用户模型。
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// Token 代表发给客户端的访问令牌。
type Token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Claims 是 JWT 内部的业务声明。
type Claims struct {
	UserID      string `json:"uid"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// Issuer 负责签发与解析 JWT。
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (i *Issuer) Issue(user User) (Token, error) {
	expiresAt := time.Now().Add(i.ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:      user.ID,
		Email:       user.Email,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	signed, err := token.SignedString(i.secret)
	if err != nil {
		return Token{}, fmt.Errorf("签发 JWT 失败: %w", err)
	}

	return Token{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}, nil
}

func (i *Issuer) Parse(rawToken string) (User, error) {
	parsedToken, err := jwt.ParseWithClaims(rawToken, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("不支持的 JWT 签名算法: %s", token.Method.Alg())
		}
		return i.secret, nil
	})
	if err != nil {
		return User{}, fmt.Errorf("解析 JWT 失败: %w", err)
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return User{}, errors.New("JWT 无效")
	}

	return User{
		ID:          claims.UserID,
		Email:       claims.Email,
		Username:    claims.Username,
		DisplayName: claims.DisplayName,
	}, nil
}

// Authenticator 负责从请求中抽取并验证访问令牌。
type Authenticator struct {
	issuer *Issuer
}

func NewAuthenticator(issuer *Issuer) *Authenticator {
	return &Authenticator{issuer: issuer}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.AuthenticateRequest(r)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Authenticator) AuthenticateRequest(r *http.Request) (User, error) {
	rawToken := strings.TrimSpace(extractBearerToken(r.Header.Get("Authorization")))
	if rawToken == "" {
		rawToken = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if rawToken == "" {
		return User{}, errors.New("缺少 Bearer Token")
	}

	user, err := a.issuer.Parse(rawToken)
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(user.ID) == "" {
		return User{}, errors.New("JWT 中缺少用户标识")
	}

	return user, nil
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// MustUser 从上下文中读取用户信息。
func MustUser(ctx context.Context) User {
	user, _ := ctx.Value(userContextKey).(User)
	return user
}
