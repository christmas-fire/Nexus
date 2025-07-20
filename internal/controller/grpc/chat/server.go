package chat

import (
	"context"
	"errors"
	"log"

	"github.com/christmas-fire/nexus/internal/controller/grpc/interceptors"
	chat "github.com/christmas-fire/nexus/internal/service/chat"
	chatv1 "github.com/christmas-fire/nexus/pkg/chat/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	chatv1.UnimplementedChatServiceServer
	chatService *chat.ChatService
}

func NewServer(chatService *chat.ChatService) *server {
	return &server{chatService: chatService}
}

func (s *server) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	creatorID, ok := ctx.Value(interceptors.UserIDKey).(int64)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to get user id from context")
	}

	memberIDs := append(req.GetMemberIds(), creatorID)
	memberIDs = uniqueInt64(memberIDs)

	var chatName *string
	if req.GetName() != "" {
		chatName = &req.Name
	}

	chatID, err := s.chatService.CreateChat(ctx, chatName, memberIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create chat")
	}

	return &chatv1.CreateChatResponse{ChatId: chatID}, nil
}

func uniqueInt64(slice []int64) []int64 {
	keys := make(map[int64]bool)
	var list []int64
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func (s *server) SendMessage(ctx context.Context, req *chatv1.SendMessageRequest) (*chatv1.SendMessageResponse, error) {
	senderID, ok := ctx.Value(interceptors.UserIDKey).(int64)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to get user id from context")
	}

	msgID, sentAt, err := s.chatService.SendMessage(ctx, req.GetChatId(), senderID, req.GetText())
	if err != nil {
		if errors.Is(err, chat.ErrPermissionDenied) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to send message")
	}

	return &chatv1.SendMessageResponse{
		MessageId: msgID,
		SentAt:    timestamppb.New(sentAt),
	}, nil
}

func (s *server) GetChatHistory(req *chatv1.GetChatHistoryRequest, stream chatv1.ChatService_GetChatHistoryServer) error {
	ctx := stream.Context()

	userID, ok := ctx.Value(interceptors.UserIDKey).(int64)
	if !ok {
		return status.Error(codes.Internal, "failed to get user id from context")
	}

	messages, err := s.chatService.GetChatHistory(ctx, req.GetChatId(), userID)
	if err != nil {
		if errors.Is(err, chat.ErrPermissionDenied) {
			return status.Error(codes.PermissionDenied, err.Error())
		}
		return status.Error(codes.Internal, "failed to get chat history")
	}

	for _, msg := range messages {
		grpcMsg := &chatv1.Message{
			Id:       msg.ID,
			ChatId:   msg.ChatID,
			SenderId: msg.SenderID,
			Text:     msg.Text,
			SentAt:   timestamppb.New(msg.SentAt),
		}

		if err := stream.Send(grpcMsg); err != nil {
			log.Printf("failed to send message to stream: %v", err)
			return status.Error(codes.Internal, "failed to send message stream")
		}
	}

	return nil
}

func (s *server) GetMyChats(ctx context.Context, req *chatv1.GetMyChatsRequest) (*chatv1.GetMyChatsResponse, error) {
	userID, ok := ctx.Value(interceptors.UserIDKey).(int64)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to get user id from context")
	}

	chats, err := s.chatService.GetChatsByUserID(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get user chats")
	}

	grpcChats := make([]*chatv1.ChatInfo, 0, len(chats))
	for _, chatInfo := range chats {
		grpcChats = append(grpcChats, &chatv1.ChatInfo{
			Id:   chatInfo.ID,
			Name: chatInfo.Name,
		})
	}

	return &chatv1.GetMyChatsResponse{
		Chats: grpcChats,
	}, nil
}
