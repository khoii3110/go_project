package user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUserNotFound       = errors.New("user not found")
)

type CreateUserParams struct {
	Username     string
	Email        string
	Role         string
	PasswordHash string
}

type Repository interface {
	CreateUser(ctx context.Context, params CreateUserParams) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	CreateSession(ctx context.Context, sessionID, userID string, expiresAt time.Time) error
	RevokeSession(ctx context.Context, sessionID string) error
	IsSessionActive(ctx context.Context, sessionID string) (bool, error)
	ListUsers(ctx context.Context) ([]User, error)
}

type PGRepository struct {
	db *pgxpool.Pool
}

func NewPGRepository(db *pgxpool.Pool) *PGRepository {
	return &PGRepository{db: db}
}

func (r *PGRepository) CreateUser(ctx context.Context, params CreateUserParams) (*User, error) {
	const query = `
		INSERT INTO users (username, email, role, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING user_id, username, email, role, password_hash, created_at, updated_at
	`

	var u User
	err := r.db.QueryRow(ctx, query,
		strings.TrimSpace(params.Username),
		strings.ToLower(strings.TrimSpace(params.Email)),
		params.Role,
		params.PasswordHash,
	).Scan(
		&u.UserID,
		&u.Username,
		&u.Email,
		&u.Role,
		&u.PasswordHash,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return &u, nil
}

func (r *PGRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT user_id, username, email, role, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`

	var u User
	err := r.db.QueryRow(ctx, query, strings.ToLower(strings.TrimSpace(email))).Scan(
		&u.UserID,
		&u.Username,
		&u.Email,
		&u.Role,
		&u.PasswordHash,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}

func (r *PGRepository) CreateSession(ctx context.Context, sessionID, userID string, expiresAt time.Time) error {
	const query = `
		INSERT INTO user_sessions (session_id, user_id, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.Exec(ctx, query, sessionID, userID, expiresAt)
	return err
}

func (r *PGRepository) RevokeSession(ctx context.Context, sessionID string) error {
	const query = `
		UPDATE user_sessions
		SET revoked_at = NOW()
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, sessionID)
	return err
}

func (r *PGRepository) IsSessionActive(ctx context.Context, sessionID string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM user_sessions
			WHERE session_id = $1
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
		)
	`

	var exists bool
	if err := r.db.QueryRow(ctx, query, sessionID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PGRepository) ListUsers(ctx context.Context) ([]User, error) {
	const query = `
		SELECT user_id, username, email, role, password_hash, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.UserID,
			&u.Username,
			&u.Email,
			&u.Role,
			&u.PasswordHash,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
