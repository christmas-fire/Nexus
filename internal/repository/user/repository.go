package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type User struct {
	ID           int64
	PasswordHash []byte
}

type UserRepository interface {
	Create(ctx context.Context, email, username string, passHash []byte) (int64, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) UserRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, email, username string, passHash []byte) (int64, error) {
	query := "INSERT INTO users (email, username, password_hash) VALUES ($1, $2, $3) RETURNING id"

	var userID int64
	err := r.db.QueryRow(ctx, query, email, username, passHash).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert user: %w", err)
	}

	return userID, nil
}

func (r *postgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := "SELECT id, password_hash FROM users WHERE email = $1"

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}
