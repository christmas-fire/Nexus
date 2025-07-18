// Файл: internal/service/auth/service.go

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/christmas-fire/nexus/internal/repository/user"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailRequired      = errors.New("email is required")
	ErrUsernameRequired   = errors.New("username is required")
	ErrPasswordRequired   = errors.New("password is required")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters long")
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService struct {
	userRepo    user.UserRepository
	tokenSecret string
	tokenTTL    time.Duration
}

func NewAuthService(userRepo user.UserRepository, secret string, ttl time.Duration) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		tokenSecret: secret,
		tokenTTL:    ttl,
	}
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (int64, error) {
	if email == "" {
		return 0, ErrEmailRequired
	}
	if username == "" {
		return 0, ErrUsernameRequired
	}
	if password == "" {
		return 0, ErrPasswordRequired
	}
	if len(password) < 8 {
		return 0, ErrPasswordTooShort
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	userID, err := s.userRepo.Create(ctx, email, username, passHash)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	if email == "" {
		return "", ErrEmailRequired
	}
	if password == "" {
		return "", ErrPasswordRequired
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", user.ID),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
	})

	signedToken, err := token.SignedString([]byte(s.tokenSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}
