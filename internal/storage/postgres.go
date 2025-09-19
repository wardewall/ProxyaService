package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Role string

const (
	RoleFree    Role = "free"
	RolePremium Role = "premium"
	RoleAdmin   Role = "admin"
)

type User struct {
	ID        int64
	Role      Role
	IsAuthed  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS users (
	telegram_id BIGINT PRIMARY KEY,
	role TEXT NOT NULL,
	is_authed BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rate_events (
	id BIGSERIAL PRIMARY KEY,
	telegram_id BIGINT NOT NULL,
	kind TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rate_events_user_time ON rate_events(telegram_id, created_at);

CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    role TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    issued_by BIGINT,
    issued_to BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tokens_expires ON tokens(expires_at);
	`)
	return err
}

func (s *Store) UpsertUser(ctx context.Context, user User) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO users (telegram_id, role, is_authed)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_id) DO UPDATE SET
	role = EXCLUDED.role,
	is_authed = EXCLUDED.is_authed,
	updated_at = now();
`, user.ID, string(user.Role), user.IsAuthed)
	return err
}

func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	var u User
	row := s.pool.QueryRow(ctx, `SELECT telegram_id, role, is_authed, created_at, updated_at FROM users WHERE telegram_id=$1`, id)
	if err := row.Scan(&u.ID, &u.Role, &u.IsAuthed, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) InsertRateEvent(ctx context.Context, telegramID int64, kind string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO rate_events (telegram_id, kind) VALUES ($1, $2)`, telegramID, kind)
	return err
}

func (s *Store) CountEventsSince(ctx context.Context, telegramID int64, since time.Time) (int, error) {
	var cnt int
	row := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rate_events WHERE telegram_id=$1 AND created_at >= $2`, telegramID, since)
	if err := row.Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

// Tokens
func (s *Store) CreateToken(ctx context.Context, token string, role Role, expiresAt *time.Time, issuedBy int64, issuedTo *int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO tokens (token, role, expires_at, issued_by, issued_to) VALUES ($1,$2,$3,$4,$5)`, token, string(role), expiresAt, issuedBy, issuedTo)
	return err
}

func (s *Store) ConsumeToken(ctx context.Context, token string, consumeBy int64) (Role, error) {
	// Try to consume if valid and not expired/consumed
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var role Role
	var expiresAt *time.Time
	var consumedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT role, expires_at, consumed_at FROM tokens WHERE token=$1`, token).Scan(&role, &expiresAt, &consumedAt)
	if err != nil {
		return "", err
	}
	if consumedAt != nil {
		return "", ErrNotFound
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return "", ErrNotFound
	}

	if _, err := tx.Exec(ctx, `UPDATE tokens SET consumed_at=now(), issued_to=$2 WHERE token=$1`, token, consumeBy); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return role, nil
}
