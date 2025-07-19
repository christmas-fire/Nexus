// Файл: internal/app/gateway/websocket.go

package gateway

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
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

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, jwtSecret string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}

	client := &Client{Conn: conn, hub: hub, send: make(chan []byte, 256)}
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

		if msg.Type == "auth" {
			c.handleAuth(msg.Payload, jwtSecret)
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
