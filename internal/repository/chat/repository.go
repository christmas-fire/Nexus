package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	ID       string
	ChatID   string
	SenderID int64
	Text     string
	SentAt   time.Time
}

type ChatRepository interface {
	CreateChat(ctx context.Context, name *string, memberIDs []int64) (string, error)
	IsMember(ctx context.Context, chatID string, userID int64) (bool, error)
	SendMessage(ctx context.Context, chatID string, senderID int64, text string) (messageID string, sentAt time.Time, err error)
	GetHistory(ctx context.Context, chatID string, limit int) ([]Message, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) ChatRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateChat(ctx context.Context, name *string, memberIDs []int64) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var chatID string
	createChatQuery := "INSERT INTO chats (name) VALUES ($1) RETURNING id"
	err = tx.QueryRow(ctx, createChatQuery, name).Scan(&chatID)
	if err != nil {
		return "", fmt.Errorf("failed to create chat: %w", err)
	}

	addMembersQuery := "INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2)"
	for _, memberID := range memberIDs {
		_, err = tx.Exec(ctx, addMembersQuery, chatID, memberID)
		if err != nil {
			return "", fmt.Errorf("failed to add member %d to chat: %w", memberID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return chatID, nil
}

func (r *postgresRepository) IsMember(ctx context.Context, chatID string, userID int64) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)"
	var isMember bool
	err := r.db.QueryRow(ctx, query, chatID, userID).Scan(&isMember)
	if err != nil {
		return false, fmt.Errorf("failed to check chat membership: %w", err)
	}
	return isMember, nil
}

func (r *postgresRepository) SendMessage(ctx context.Context, chatID string, senderID int64, text string) (string, time.Time, error) {
	query := "INSERT INTO messages (chat_id, sender_id, text) VALUES ($1, $2, $3) RETURNING id, sent_at"
	var messageID string
	var sentAt time.Time
	err := r.db.QueryRow(ctx, query, chatID, senderID, text).Scan(&messageID, &sentAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to send message: %w", err)
	}
	return messageID, sentAt, nil
}

func (r *postgresRepository) GetHistory(ctx context.Context, chatID string, limit int) ([]Message, error) {
	query := `
		SELECT id, chat_id, sender_id, text, sent_at 
		FROM messages 
		WHERE chat_id = $1 
		ORDER BY sent_at DESC 
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat history: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Text, &msg.SentAt); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
