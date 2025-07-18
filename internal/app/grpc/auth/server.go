package auth

import (
	"context"
	"errors"

	"github.com/christmas-fire/nexus/internal/service/auth"
	authv1 "github.com/christmas-fire/nexus/pkg/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	authv1.UnimplementedAuthServiceServer
	authService *auth.AuthService
}

func NewServer(authService *auth.AuthService) *server {
	return &server{authService: authService}
}

func (s *server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	userID, err := s.authService.Register(ctx, req.GetEmail(), req.GetUsername(), req.GetPassword())
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &authv1.RegisterResponse{
		UserId: userID,
	}, nil
}

func (s *server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	token, err := s.authService.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to login")
	}

	return &authv1.LoginResponse{
		AccessToken: token,
	}, nil
}
