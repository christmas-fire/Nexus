package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/christmas-fire/nexus/internal/app/gateway"
	grpcAuth "github.com/christmas-fire/nexus/internal/app/grpc/auth"
	grpcChat "github.com/christmas-fire/nexus/internal/app/grpc/chat"
	"github.com/christmas-fire/nexus/internal/app/grpc/interceptors"
	"github.com/christmas-fire/nexus/internal/app/rest"

	"github.com/christmas-fire/nexus/internal/repository/chat"
	userRepo "github.com/christmas-fire/nexus/internal/repository/user"

	authService "github.com/christmas-fire/nexus/internal/service/auth"
	chatService "github.com/christmas-fire/nexus/internal/service/chat"

	"github.com/christmas-fire/nexus/internal/storage/postgres"
	"github.com/christmas-fire/nexus/internal/storage/redis"

	authv1 "github.com/christmas-fire/nexus/pkg/auth/v1"
	chatv1 "github.com/christmas-fire/nexus/pkg/chat/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()

	connString := os.Getenv("POSTGRES_DSN")
	dbPool, err := postgres.NewStorage(ctx, connString)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	redisAddr := os.Getenv("REDIS_ADDR")

	redisClient, err := redis.NewClient(ctx, redisAddr)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}
	tokenTTL := 72 * time.Hour

	publicMethods := map[string]bool{
		"/nexus.auth.v1.AuthService/Login":    true,
		"/nexus.auth.v1.AuthService/Register": true,
	}

	userRepository := userRepo.NewPostgresRepository(dbPool)
	chRepository := chat.NewPostgresRepository(dbPool)

	authenticationService := authService.NewAuthService(userRepository, jwtSecret, tokenTTL)
	chService := chatService.NewChatService(chRepository, redisClient)

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

	grpcConn, err := grpc.DialContext(
		ctx,
		"localhost:8080",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to create gRPC client connection: %v", err)
	}
	defer grpcConn.Close()

	chatGrpcClient := chatv1.NewChatServiceClient(grpcConn)

	hub := gateway.NewHub(redisClient, chRepository)
	go hub.Run()
	go hub.SubscribeToMessages(ctx)

	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		gateway.ServeWs(hub, w, r, jwtSecret, chatGrpcClient)
	})

	authRestHandler := rest.NewAuthHandler(authenticationService)

	httpMux.HandleFunc("/api/v1/register", authRestHandler.Register)
	httpMux.HandleFunc("/api/v1/login", authRestHandler.Login)

	fileServer := http.FileServer(http.Dir("./web"))
	httpMux.Handle("/", fileServer)

	httpServer := &http.Server{
		Addr:    ":8081",
		Handler: httpMux,
	}

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

	go func() {
		log.Printf("WebSocket gateway is listening on :8081")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server failed: %w", err)
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}
