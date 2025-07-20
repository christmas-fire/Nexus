package ws

import (
	"encoding/json"
	"time"

	"github.com/christmas-fire/nexus/internal/models"
)

type WsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type AuthRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type NewMessage struct {
	ID       string    `json:"id"`
	ChatID   string    `json:"chat_id"`
	SenderID int64     `json:"sender_id"`
	Text     string    `json:"text"`
	SentAt   time.Time `json:"sent_at"`
}

type SendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func NewWsMessage(typ string, payload interface{}) ([]byte, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	msg := WsMessage{
		Type:    typ,
		Payload: p,
	}

	return json.Marshal(msg)
}

type GetMyChatsRequest struct{}

type MyChatsResponse struct {
	Chats []ChatInfo `json:"chats"`
}

type ChatInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetChatHistoryRequest struct {
	ChatID string `json:"chat_id"`
}

type ChatHistoryResponse struct {
	Messages []models.Message `json:"messages"`
}

type CreateChatRequest struct {
	MemberIDs []int64 `json:"member_ids"`
	Name      string  `json:"name,omitempty"`
}
