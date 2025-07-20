package ws

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/christmas-fire/nexus/internal/models"
	chatv1 "github.com/christmas-fire/nexus/pkg/chat/v1"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/metadata"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	writeWait      = 10 * time.Second
	maxMessageSize = 512
	pingPeriod     = (pongWait * 9) / 10
	pongWait       = 60 * time.Second
)

type Client struct {
	UserID     int64
	Token      string
	Conn       *websocket.Conn
	hub        *Hub
	send       chan []byte
	chatClient chatv1.ChatServiceClient
	ctx        context.Context
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, jwtSecret string, chatClient chatv1.ChatServiceClient) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		Conn:       conn,
		hub:        hub,
		send:       make(chan []byte, 256),
		chatClient: chatClient,
		ctx:        r.Context()}
	client.hub.register <- client

	log.Printf("client connected: %p", client.Conn)

	defer func() {
		client.hub.unregister <- client
		client.Conn.Close()
	}()

	go client.writePump()
	client.readPump(jwtSecret)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(jwtSecret string) {
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected websocket close error: %v", err)
			}
			break
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("failed to unmarshal message: %v", err)
			continue
		}

		switch msg.Type {
		case "auth":
			c.handleAuth(msg.Payload, jwtSecret)

		case "send_message":
			c.handleSendMessage(msg.Payload)

		case "get_my_chats":
			c.handleGetMyChats()

		case "get_chat_history":
			c.handleGetChatHistory(msg.Payload)
		}

	}
}

func (c *Client) handleAuth(payload []byte, jwtSecret string) {
	var authReq AuthRequest
	if err := json.Unmarshal(payload, &authReq); err != nil {
		log.Printf("failed to unmarshal auth payload: %v", err)
		return
	}

	token, err := jwt.Parse(authReq.Token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	authResp := AuthResponse{Success: false}
	if err != nil || !token.Valid {
		authResp.Message = "Invalid token"
	} else if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if userIDStr, ok := claims["sub"].(string); ok {
			userID, _ := strconv.ParseInt(userIDStr, 10, 64)
			c.UserID = userID
			c.Token = authReq.Token
			authResp.Success = true
			authResp.Message = "Authentication successful"
			log.Printf("client authenticated: UserID=%d", c.UserID)
		} else {
			authResp.Message = "Invalid token claims"
		}
	} else {
		authResp.Message = "Invalid token claims"
	}

	respBytes, err := NewWsMessage("auth_status", authResp)
	if err != nil {
		log.Printf("failed to create auth response message: %v", err)
		return
	}

	c.Conn.WriteMessage(websocket.TextMessage, respBytes)
}

func (c *Client) handleSendMessage(payload json.RawMessage) {
	if c.Token == "" { /* ... */
		return
	}
	var req SendMessageRequest
	if err := json.Unmarshal(payload, &req); err != nil { /* ... */
		return
	}

	_, err := c.chatClient.SendMessage(c.createAuthContext(), &chatv1.SendMessageRequest{
		ChatId: req.ChatID,
		Text:   req.Text,
	})

	if err != nil {
		log.Printf("failed to send message via gRPC: %v", err)
	}

}

func (c *Client) createAuthContext() context.Context {
	md := metadata.New(map[string]string{"authorization": "Bearer " + c.Token})
	return metadata.NewOutgoingContext(c.ctx, md)
}

func (c *Client) handleGetMyChats() {
	if c.UserID == 0 {
		return
	}

	grpcResp, err := c.chatClient.GetMyChats(c.createAuthContext(), &chatv1.GetMyChatsRequest{})
	if err != nil {
		log.Printf("failed to get chats via gRPC for user %d: %v", c.UserID, err)
		return
	}

	wsChats := make([]ChatInfo, 0, len(grpcResp.GetChats()))
	for _, grpcChat := range grpcResp.GetChats() {
		wsChats = append(wsChats, ChatInfo{
			ID:   grpcChat.GetId(),
			Name: grpcChat.GetName(),
		})
	}

	wsResp := MyChatsResponse{
		Chats: wsChats,
	}

	wsMsg, err := NewWsMessage("my_chats_list", wsResp)
	if err != nil {
		log.Printf("failed to create my_chats_list message: %v", err)
		return
	}

	c.send <- wsMsg
}

func (c *Client) handleGetChatHistory(payload json.RawMessage) {
	if c.UserID == 0 {
		return
	}

	var req GetChatHistoryRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal get_chat_history payload: %v", err)
		return
	}

	stream, err := c.chatClient.GetChatHistory(c.createAuthContext(), &chatv1.GetChatHistoryRequest{ChatId: req.ChatID})
	if err != nil {
		log.Printf("failed to call GetChatHistory gRPC: %v", err)
		return
	}

	var history []models.Message
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("error receiving from GetChatHistory stream: %v", err)
			return
		}
		history = append(history, models.Message{
			ID:       msg.Id,
			ChatID:   msg.ChatId,
			SenderID: msg.SenderId,
			Text:     msg.Text,
			SentAt:   msg.SentAt.AsTime(),
		})
	}

	wsResp := ChatHistoryResponse{Messages: history}
	wsMsg, _ := NewWsMessage("chat_history", wsResp)
	c.send <- wsMsg
}
