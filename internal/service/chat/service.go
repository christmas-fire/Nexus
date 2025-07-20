package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/christmas-fire/nexus/internal/models"
	"github.com/christmas-fire/nexus/internal/repository/chat"
	"github.com/redis/go-redis/v9"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
)

const (
	messagesChannel   = "messages"
	chatEventsChannel = "chat_events"
	messageLimit      = 50
)

type ChatService struct {
	chatRepo chat.ChatRepository
	redis    *redis.Client
}

func NewChatService(chatRepo chat.ChatRepository, redisClient *redis.Client) *ChatService {
	return &ChatService{chatRepo: chatRepo, redis: redisClient}
}

type ChatCreatedEvent struct {
	ChatID    string  `json:"chat_id"`
	MemberIDs []int64 `json:"member_ids"`
}

func (s *ChatService) CreateChat(ctx context.Context, name *string, memberIDs []int64) (string, error) {
	chatID, err := s.chatRepo.CreateChat(ctx, name, memberIDs)
	if err != nil {
		return "", err
	}

	event := ChatCreatedEvent{
		ChatID:    chatID,
		MemberIDs: memberIDs,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal chat created event: %v", err)
		return chatID, nil
	}

	if err := s.redis.Publish(ctx, chatEventsChannel, eventBytes).Err(); err != nil {
		log.Printf("failed to publish chat created event: %v", err)
	}

	return chatID, nil
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

	messageID, sentAt, err := s.chatRepo.SendMessage(ctx, chatID, senderID, text)
	if err != nil {
		return "", time.Time{}, err
	}

	msg := models.Message{
		ID:       messageID,
		ChatID:   chatID,
		SenderID: senderID,
		Text:     text,
		SentAt:   sentAt,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message for redis: %v", err)
		return messageID, sentAt, nil
	}

	if err := s.redis.Publish(ctx, messagesChannel, msgBytes).Err(); err != nil {
		log.Printf("failed to publish message to redis: %v", err)
	}

	return messageID, sentAt, nil
}

func (s *ChatService) GetChatHistory(ctx context.Context, chatID string, userID int64) ([]models.Message, error) {
	isMember, err := s.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrPermissionDenied
	}

	return s.chatRepo.GetHistory(ctx, chatID, messageLimit)
}

func (s *ChatService) GetChatsByUserID(ctx context.Context, userID int64) ([]chat.ChatInfo, error) {
	return s.chatRepo.GetChatsByUserID(ctx, userID)
}
