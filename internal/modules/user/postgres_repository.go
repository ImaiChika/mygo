package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository 使用 PostgreSQL 持久化用户数据。
type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, input RegisterInput, passwordHash string) (User, error) {
	now := time.Now()
	created := User{
		ID:          uuid.NewString(),
		Email:       strings.ToLower(strings.TrimSpace(input.Email)),
		Username:    strings.ToLower(strings.TrimSpace(input.Username)),
		DisplayName: strings.TrimSpace(input.DisplayName),
		AvatarURL:   "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := r.db.QueryRow(ctx, `
		INSERT INTO users (id, email, username, display_name, avatar_url, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, email, username, display_name, avatar_url, created_at, updated_at
	`, created.ID, created.Email, created.Username, created.DisplayName, created.AvatarURL, passwordHash, created.CreatedAt, created.UpdatedAt).Scan(
		&created.ID,
		&created.Email,
		&created.Username,
		&created.DisplayName,
		&created.AvatarURL,
		&created.CreatedAt,
		&created.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, errors.New("邮箱或用户名已存在")
		}
		return User{}, fmt.Errorf("创建用户失败: %w", err)
	}

	return created, nil
}

func (r *PostgresRepository) GetUserByEmailOrUsername(ctx context.Context, identity string) (UserCredentials, error) {
	var credentials UserCredentials
	err := r.db.QueryRow(ctx, `
		SELECT id, email, username, display_name, avatar_url, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1 OR username = $1
	`, strings.ToLower(strings.TrimSpace(identity))).Scan(
		&credentials.ID,
		&credentials.Email,
		&credentials.Username,
		&credentials.DisplayName,
		&credentials.AvatarURL,
		&credentials.PasswordHash,
		&credentials.CreatedAt,
		&credentials.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserCredentials{}, errors.New("用户不存在")
	}
	if err != nil {
		return UserCredentials{}, fmt.Errorf("查询用户失败: %w", err)
	}

	return credentials, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (User, error) {
	var found User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, username, display_name, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`, strings.TrimSpace(userID)).Scan(
		&found.ID,
		&found.Email,
		&found.Username,
		&found.DisplayName,
		&found.AvatarURL,
		&found.CreatedAt,
		&found.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errors.New("用户不存在")
	}
	if err != nil {
		return User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	return found, nil
}

func (r *PostgresRepository) SearchUsers(ctx context.Context, query string, excludeUserID string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, email, username, display_name, avatar_url, created_at, updated_at
		FROM users
		WHERE id <> $2
		  AND (
			username ILIKE '%' || $1 || '%'
			OR display_name ILIKE '%' || $1 || '%'
			OR email ILIKE '%' || $1 || '%'
		  )
		ORDER BY created_at DESC
		LIMIT $3
	`, strings.TrimSpace(query), strings.TrimSpace(excludeUserID), limit)
	if err != nil {
		return nil, fmt.Errorf("搜索用户失败: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var found User
		if err := rows.Scan(
			&found.ID,
			&found.Email,
			&found.Username,
			&found.DisplayName,
			&found.AvatarURL,
			&found.CreatedAt,
			&found.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描用户失败: %w", err)
		}
		users = append(users, found)
	}

	return users, rows.Err()
}
