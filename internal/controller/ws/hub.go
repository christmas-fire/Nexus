package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/christmas-fire/nexus/internal/models"
	"github.com/christmas-fire/nexus/internal/repository/chat"
	"github.com/redis/go-redis/v9"
)

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	redis      *redis.Client
	chatRepo   chat.ChatRepository
}

func NewHub(redisClient *redis.Client, chatRepo chat.ChatRepository) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		redis:      redisClient,
		chatRepo:   chatRepo,
	}
}

type ChatCreatedEvent struct {
	MemberIDs []int64 `json:"member_ids"`
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("client registered: %p", client.Conn)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				log.Printf("client unregistered: %p", client.Conn)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) SubscribeToMessages(ctx context.Context) {
	pubsub := h.redis.Subscribe(ctx, "messages")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for redisMsg := range ch {
		var msg models.Message
		if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
			log.Printf("failed to unmarshal message from redis: %v", err)
			continue
		}

		memberIDs, err := h.chatRepo.GetChatMemberIDs(ctx, msg.ChatID)
		if err != nil {
			log.Printf("failed to get chat members for broadcast: %v", err)
			continue
		}

		wsMsgBytes, err := NewWsMessage("new_message", msg)
		if err != nil {
			log.Printf("failed to create ws message for broadcast: %v", err)
			continue
		}

		h.mu.RLock()
		for client := range h.clients {
			for _, memberID := range memberIDs {
				if client.UserID == memberID {
					select {
					case client.send <- wsMsgBytes:
					default:
						log.Printf("client send channel full, dropping message for user %d", client.UserID)
					}
					break
				}
			}
		}
		h.mu.RUnlock()
	}
}

func (h *Hub) SubscribeToChatEvents(ctx context.Context) {
	pubsub := h.redis.Subscribe(ctx, "chat_events")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for redisMsg := range ch {
		var event ChatCreatedEvent
		if err := json.Unmarshal([]byte(redisMsg.Payload), &event); err != nil {
			log.Printf("failed to unmarshal chat event from redis: %v", err)
			continue
		}

		wsMsgBytes, _ := NewWsMessage("chat_list_updated", nil)

		h.mu.RLock()
		for client := range h.clients {
			for _, memberID := range event.MemberIDs {
				if client.UserID == memberID {
					select {
					case client.send <- wsMsgBytes:
					default:
						log.Printf("client send channel full, dropping chat update for user %d", client.UserID)
					}
					break
				}
			}
		}
		h.mu.RUnlock()
	}
}
