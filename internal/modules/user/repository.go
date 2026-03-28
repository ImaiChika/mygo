package user

import "context"

// Repository 定义用户模块的数据访问契约。
type Repository interface {
	CreateUser(ctx context.Context, input RegisterInput, passwordHash string) (User, error)
	GetUserByEmailOrUsername(ctx context.Context, identity string) (UserCredentials, error)
	GetUserByID(ctx context.Context, userID string) (User, error)
	SearchUsers(ctx context.Context, query string, excludeUserID string, limit int) ([]User, error)
}
