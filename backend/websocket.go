package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow cross-origin WebSocket connections; adjust if you lock down origins.
		return true
	},
}

// Hub tracks connected clients by user id.
type Hub struct {
	mu      sync.RWMutex
	clients map[int]map[*Client]struct{}
}

// NewHub constructs an empty Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[int]map[*Client]struct{}),
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.userID]; !ok {
		h.clients[c.userID] = make(map[*Client]struct{})
	}
	h.clients[c.userID][c] = struct{}{}

	log.Printf("ws connect user=%d remote=%s", c.userID, c.conn.RemoteAddr())
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.clients[c.userID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.clients, c.userID)
		}
	}
}

// deliver routes a message to receiver (and echo to sender).
func (h *Hub) deliver(msg ChatMessage) {
	targets := []int{msg.ReceiverID}
	if msg.SenderID != msg.ReceiverID {
		targets = append(targets, msg.SenderID)
	}

	for _, uid := range targets {
		for _, client := range h.clientsForUser(uid) {
			client.enqueue(msg)
		}
	}
}

func (h *Hub) clientsForUser(userID int) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	conns := h.clients[userID]
	result := make([]*Client, 0, len(conns))
	for c := range conns {
		result = append(result, c)
	}
	return result
}

// Client represents a live WebSocket connection.
type Client struct {
	conn      *websocket.Conn
	send      chan ChatMessage
	userID    int
	hub       *Hub
	closeOnce sync.Once
	done      chan struct{}
}

func newClient(h *Hub, conn *websocket.Conn, userID int) *Client {
	return &Client{
		conn:   conn,
		send:   make(chan ChatMessage, 8),
		userID: userID,
		hub:    h,
		done:   make(chan struct{}),
	}
}

func (c *Client) enqueue(msg ChatMessage) {
	select {
	case <-c.done:
		return
	case c.send <- msg:
	default:
		// Drop the client if it cannot keep up.
		go c.close("send buffer full")
	}
}

func (c *Client) close(reason string) {
	c.closeOnce.Do(func() {
		c.hub.removeClient(c)
		close(c.done)
		_ = c.conn.Close()
		log.Printf("ws disconnect user=%d reason=%s", c.userID, reason)
	})
}

func (c *Client) readPump() {
	defer c.close("read loop ended")

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var incoming ChatMessage
		if err := c.conn.ReadJSON(&incoming); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws read error user=%d: %v", c.userID, err)
			}
			return
		}

		if incoming.Type == "" {
			incoming.Type = defaultMessageType
		}
		incoming.SenderID = c.userID
		incoming.Content = strings.TrimSpace(incoming.Content)
		incoming.CreatedAt = time.Now()

		if incoming.ReceiverID == 0 || incoming.Content == "" {
			log.Printf("ws drop message user=%d: missing receiver or content", c.userID)
			continue
		}

		if err := saveMessage(incoming); err != nil {
			log.Printf("persist message failed: %v", err)
			continue
		}

		c.hub.deliver(incoming)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.close("write loop ended")
	}()

	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("ws write error user=%d: %v", c.userID, err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func websocketHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := authenticateWebsocket(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		client := newClient(hub, conn, userID)
		hub.addClient(client)

		go client.writePump()
		client.readPump()
	}
}

// authenticateWebsocket validates JWT from query ?token= or Authorization header.
func authenticateWebsocket(r *http.Request) (int, error) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		tokenString = tokenFromHeader(r.Header.Get("Authorization"))
	}

	if tokenString == "" {
		return 0, errors.New("token required")
	}

	claims, err := parseToken(tokenString)
	if err != nil {
		return 0, errors.New("invalid or expired token")
	}
	return claims.UserID, nil
}

func tokenFromHeader(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
