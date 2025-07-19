package models

import "time"

type Message struct {
	ID       string    `json:"id"`
	ChatID   string    `json:"chat_id"`
	SenderID int64     `json:"sender_id"`
	Text     string    `json:"text"`
	SentAt   time.Time `json:"sent_at"`
}
