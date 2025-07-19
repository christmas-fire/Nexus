package gateway

import (
	"encoding/json"
	"time"
)

// WsMessage - это общая обертка для всех сообщений.
// Мы используем json.RawMessage, чтобы отложить парсинг `payload`,
// пока мы не узнаем тип сообщения.
type WsMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// AuthRequest - это структура для payload сообщения типа "auth".
type AuthRequest struct {
	Token string `json:"token"`
}

// AuthResponse - это структура для payload сообщения типа "auth_status".
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
