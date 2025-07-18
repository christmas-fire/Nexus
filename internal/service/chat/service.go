package chat

import (
	"context"
	"errors"
	"time"

	"github.com/christmas-fire/nexus/internal/repository/chat"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
)

type ChatService struct {
	chatRepo chat.ChatRepository
}

func NewChatService(chatRepo chat.ChatRepository) *ChatService {
	return &ChatService{chatRepo: chatRepo}
}

func (s *ChatService) CreateChat(ctx context.Context, name *string, memberIDs []int64) (string, error) {
	return s.chatRepo.CreateChat(ctx, name, memberIDs)
}

func (s *ChatService) SendMessage(ctx context.Context, chatID string, senderID int64, text string) (string, time.Time, error) {
	isMember, err := s.chatRepo.IsMember(ctx, chatID, senderID)
	if err != nil {
		return "", time.Time{}, err
	}
	if !isMember {
		return "", time.Time{}, ErrPermissionDenied
	}

	if text == "" {
		return "", time.Time{}, errors.New("message text cannot be empty")
	}

	return s.chatRepo.SendMessage(ctx, chatID, senderID, text)
}

func (s *ChatService) GetChatHistory(ctx context.Context, chatID string, userID int64) ([]chat.Message, error) {
	isMember, err := s.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	const messageLimit = 50
	return s.chatRepo.GetHistory(ctx, chatID, messageLimit)
}
