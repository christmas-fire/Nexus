package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcAuth "github.com/christmas-fire/nexus/internal/app/grpc/auth"
	grpcChat "github.com/christmas-fire/nexus/internal/app/grpc/chat"
	"github.com/christmas-fire/nexus/internal/app/grpc/interceptors"

	"github.com/christmas-fire/nexus/internal/repository/chat"
	userRepo "github.com/christmas-fire/nexus/internal/repository/user"

	authService "github.com/christmas-fire/nexus/internal/service/auth"
	chatService "github.com/christmas-fire/nexus/internal/service/chat"

	"github.com/christmas-fire/nexus/internal/storage/postgres"

	authv1 "github.com/christmas-fire/nexus/pkg/auth/v1"
	chatv1 "github.com/christmas-fire/nexus/pkg/chat/v1"

	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	connString := os.Getenv("POSTGRES_DSN")
	dbPool, err := postgres.NewStorage(ctx, connString)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
	tokenTTL := 1 * time.Hour

	publicMethods := map[string]bool{
		"/nexus.auth.v1.AuthService/Login":    true,
		"/nexus.auth.v1.AuthService/Register": true,
	}

	userRepository := userRepo.NewPostgresRepository(dbPool)
	chRepository := chat.NewPostgresRepository(dbPool)

	authenticationService := authService.NewAuthService(userRepository, jwtSecret, tokenTTL)
	chService := chatService.NewChatService(chRepository)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.AuthUnaryInterceptor(jwtSecret, publicMethods),
		),
		grpc.ChainStreamInterceptor(
			interceptors.AuthStreamInterceptor(jwtSecret, publicMethods),
		),
	)

	grpcAuthServer := grpcAuth.NewServer(authenticationService)
	grpcChatServer := grpcChat.NewServer(chService)

	authv1.RegisterAuthServiceServer(grpcServer, grpcAuthServer)
	chatv1.RegisterChatServiceServer(grpcServer, grpcChatServer)

	errChan := make(chan error, 1)

	go func() {
		listener, err := net.Listen("tcp", ":8080")
		if err != nil {
			errChan <- fmt.Errorf("failed to listen: %v", err)
		}

		log.Println("gRPC server is listening on", listener.Addr())
		if err := grpcServer.Serve(listener); err != nil {
			errChan <- fmt.Errorf("failed to serve: %v", err)
		}
	}()

	quitChan := make(chan os.Signal, 1)
	signal.Notify(quitChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quitChan:
		log.Printf("Shutdown signal received: %s", sig)
	case err := <-errChan:
		log.Printf("Server error, initiating shutdown: %v", err)
	}

	grpcServer.GracefulStop()

}
