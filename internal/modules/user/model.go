package user

import "time"

// User 是系统中的基础用户模型。
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserCredentials 用于登录时读取密码散列。
type UserCredentials struct {
	User
	PasswordHash string
}

type RegisterInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type LoginInput struct {
	EmailOrUsername string `json:"email_or_username"`
	Password        string `json:"password"`
}
