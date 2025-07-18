package interceptors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userCtxKey string

const (
	UserIDKey userCtxKey = "userID"
)

func authenticate(ctx context.Context, jwtSecret string) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 {
		return 0, status.Error(codes.Unauthenticated, "authorization header is not provided")
	}

	if !strings.HasPrefix(authHeader[0], "Bearer ") {
		return 0, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}
	tokenString := strings.TrimPrefix(authHeader[0], "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, status.Error(codes.Unauthenticated, "invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "invalid token claims")
	}

	userIDStr, ok := claims["sub"].(string)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "invalid user id in token")
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0, status.Error(codes.Unauthenticated, "invalid user id format in token")
	}

	return userID, nil
}

func AuthUnaryInterceptor(jwtSecret string, publicMethods map[string]bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if _, ok := publicMethods[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		userID, err := authenticate(ctx, jwtSecret)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, UserIDKey, userID)
		return handler(newCtx, req)
	}
}

func AuthStreamInterceptor(jwtSecret string, publicMethods map[string]bool) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, ok := publicMethods[info.FullMethod]; ok {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		userID, err := authenticate(ctx, jwtSecret)
		if err != nil {
			return err
		}

		wrapped := newWrappedServerStream(ss, context.WithValue(ctx, UserIDKey, userID))
		return handler(srv, wrapped)
	}
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func newWrappedServerStream(ss grpc.ServerStream, ctx context.Context) *wrappedServerStream {
	return &wrappedServerStream{ServerStream: ss, ctx: ctx}
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
