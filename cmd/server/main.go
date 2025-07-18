package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	grpcAuth "github.com/christmas-fire/nexus/internal/app/grpc/auth"
	userRepo "github.com/christmas-fire/nexus/internal/repository/user"
	authService "github.com/christmas-fire/nexus/internal/service/auth"
	"github.com/christmas-fire/nexus/internal/storage/postgres"
	authv1 "github.com/christmas-fire/nexus/pkg/auth/v1"
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

	userRepository := userRepo.NewPostgresRepository(dbPool)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
	tokenTTL := 1 * time.Hour

	authenticationService := authService.NewAuthService(userRepository, jwtSecret, tokenTTL)

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	grpcAuthServer := grpcAuth.NewServer(authenticationService)

	authv1.RegisterAuthServiceServer(grpcServer, grpcAuthServer)

	log.Println("gRPC server is listening on", listener.Addr())
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
