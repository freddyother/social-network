package realtime

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxMessageSize = 4 << 10
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
)

type Hub struct {
	mu                 sync.RWMutex
	userClients        map[string]map[*Client]struct{}
	postClients        map[string]map[*Client]struct{}
	postSubscriptionOK func(ctx context.Context, userID, postID string) bool
	userConnected      func(ctx context.Context, userID string)
	messageDelivered   func(ctx context.Context, userID, messageID string)
	conversationRead   func(ctx context.Context, userID, conversationUserID string)
	chatHistory        func(ctx context.Context, userID, conversationUserID, beforeMessageID string, limit int) (any, error)
}

type Client struct {
	hub               *Hub
	conn              *websocket.Conn
	userID            string
	send              chan []byte
	postSubscriptions map[string]struct{}
	activeChatUserID  string
	mu                sync.RWMutex
	closeOnce         sync.Once
	closed            bool
}

type inboundCommand struct {
	Type               string `json:"type"`
	PostID             string `json:"postId,omitempty"`
	MessageID          string `json:"messageId,omitempty"`
	ConversationUserID string `json:"conversationUserId,omitempty"`
	BeforeMessageID    string `json:"beforeMessageId,omitempty"`
	RequestID          string `json:"requestId,omitempty"`
	Limit              int    `json:"limit,omitempty"`
}

type outboundMessage struct {
	Type      string    `json:"type"`
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func NewHub() *Hub {
	return &Hub{
		userClients: make(map[string]map[*Client]struct{}),
		postClients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) SetPostSubscriptionAuthorizer(authorizer func(ctx context.Context, userID, postID string) bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.postSubscriptionOK = authorizer
}

func (h *Hub) SetUserConnectedHandler(handler func(ctx context.Context, userID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.userConnected = handler
}

func (h *Hub) SetMessageDeliveredHandler(handler func(ctx context.Context, userID, messageID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messageDelivered = handler
}

func (h *Hub) SetConversationReadHandler(handler func(ctx context.Context, userID, conversationUserID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.conversationRead = handler
}

func (h *Hub) SetChatHistoryHandler(handler func(ctx context.Context, userID, conversationUserID, beforeMessageID string, limit int) (any, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.chatHistory = handler
}

func (h *Hub) ServeConn(userID string, conn *websocket.Conn) {
	client := &Client{
		hub:               h,
		conn:              conn,
		userID:            strings.TrimSpace(userID),
		send:              make(chan []byte, 32),
		postSubscriptions: make(map[string]struct{}),
	}

	h.registerClient(client)
	client.enqueueMessage("ws.ready", map[string]any{
		"userId": client.userID,
	})

	go client.writePump()
	go client.readPump()
	h.handleUserConnected(client.userID)
}

func (h *Hub) PublishToUser(userID, eventType string, payload any) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}

	h.mu.RLock()
	clients := h.snapshotClients(h.userClients[trimmedUserID])
	h.mu.RUnlock()

	h.publish(clients, eventType, payload)
}

func (h *Hub) PublishToPost(postID, eventType string, payload any) {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return
	}

	h.mu.RLock()
	clients := h.snapshotClients(h.postClients[trimmedPostID])
	h.mu.RUnlock()

	h.publish(clients, eventType, payload)
}

func (h *Hub) publish(clients []*Client, eventType string, payload any) {
	if len(clients) == 0 || strings.TrimSpace(eventType) == "" {
		return
	}

	message, err := json.Marshal(outboundMessage{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return
	}

	for _, client := range clients {
		if !client.enqueue(message) {
			client.shutdown()
		}
	}
}

func (h *Hub) snapshotClients(source map[*Client]struct{}) []*Client {
	if len(source) == 0 {
		return nil
	}

	clients := make([]*Client, 0, len(source))
	for client := range source {
		clients = append(clients, client)
	}

	return clients
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.userClients[client.userID]
	if clients == nil {
		clients = make(map[*Client]struct{})
		h.userClients[client.userID] = clients
	}

	clients[client] = struct{}{}
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients := h.userClients[client.userID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.userClients, client.userID)
		}
	}

	for postID := range client.postSubscriptions {
		if clients := h.postClients[postID]; clients != nil {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.postClients, postID)
			}
		}
	}
}

func (h *Hub) subscribeClientToPost(client *Client, postID string) {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := client.postSubscriptions[trimmedPostID]; exists {
		return
	}

	clients := h.postClients[trimmedPostID]
	if clients == nil {
		clients = make(map[*Client]struct{})
		h.postClients[trimmedPostID] = clients
	}

	clients[client] = struct{}{}
	client.postSubscriptions[trimmedPostID] = struct{}{}
}

func (h *Hub) unsubscribeClientFromPost(client *Client, postID string) {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(client.postSubscriptions, trimmedPostID)

	if clients := h.postClients[trimmedPostID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.postClients, trimmedPostID)
		}
	}
}

func (c *Client) readPump() {
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	defer c.shutdown()

	for {
		var command inboundCommand
		if err := c.conn.ReadJSON(&command); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}

			return
		}

		switch command.Type {
		case "subscribe.post":
			if !c.hub.canSubscribeToPost(c.userID, command.PostID) {
				continue
			}
			c.hub.subscribeClientToPost(c, command.PostID)
		case "unsubscribe.post":
			c.hub.unsubscribeClientFromPost(c, command.PostID)
		case "ack.chat.delivered":
			c.hub.handleDeliveredAck(c.userID, command.MessageID)
		case "ack.chat.read":
			c.hub.handleConversationRead(c.userID, command.ConversationUserID)
		case "chat.view":
			c.setActiveChatUser(command.ConversationUserID)
		case "chat.leave":
			c.setActiveChatUser("")
		case "chat.history":
			c.hub.handleChatHistory(c, command)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.shutdown()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) enqueueMessage(eventType string, payload any) bool {
	message, err := json.Marshal(outboundMessage{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return false
	}

	return c.enqueue(message)
}

func (c *Client) enqueue(message []byte) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false
	}

	select {
	case c.send <- message:
		return true
	default:
		return false
	}
}

func (c *Client) shutdown() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()

		c.hub.unregisterClient(c)
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *Client) setActiveChatUser(conversationUserID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeChatUserID = strings.TrimSpace(conversationUserID)
}

func (c *Client) activeChatUser() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.activeChatUserID
}

func (h *Hub) canSubscribeToPost(userID, postID string) bool {
	h.mu.RLock()
	authorizer := h.postSubscriptionOK
	h.mu.RUnlock()

	if authorizer == nil {
		return true
	}

	return authorizer(context.Background(), strings.TrimSpace(userID), strings.TrimSpace(postID))
}

func (h *Hub) HasUserConnection(userID string) bool {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return false
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.userClients[trimmedUserID]) > 0
}

func (h *Hub) IsViewingConversation(userID, conversationUserID string) bool {
	trimmedUserID := strings.TrimSpace(userID)
	trimmedConversationUserID := strings.TrimSpace(conversationUserID)
	if trimmedUserID == "" || trimmedConversationUserID == "" {
		return false
	}

	h.mu.RLock()
	clients := h.snapshotClients(h.userClients[trimmedUserID])
	h.mu.RUnlock()

	for _, client := range clients {
		if client.activeChatUser() == trimmedConversationUserID {
			return true
		}
	}

	return false
}

func (h *Hub) handleDeliveredAck(userID, messageID string) {
	h.mu.RLock()
	handler := h.messageDelivered
	h.mu.RUnlock()

	if handler == nil {
		return
	}

	go handler(context.Background(), strings.TrimSpace(userID), strings.TrimSpace(messageID))
}

func (h *Hub) handleUserConnected(userID string) {
	h.mu.RLock()
	handler := h.userConnected
	h.mu.RUnlock()

	if handler == nil {
		return
	}

	go handler(context.Background(), strings.TrimSpace(userID))
}

func (h *Hub) handleConversationRead(userID, conversationUserID string) {
	h.mu.RLock()
	handler := h.conversationRead
	h.mu.RUnlock()

	if handler == nil {
		return
	}

	go handler(context.Background(), strings.TrimSpace(userID), strings.TrimSpace(conversationUserID))
}

func (h *Hub) handleChatHistory(client *Client, command inboundCommand) {
	h.mu.RLock()
	handler := h.chatHistory
	h.mu.RUnlock()

	if handler == nil {
		return
	}

	go func() {
		payload, err := handler(
			context.Background(),
			strings.TrimSpace(client.userID),
			strings.TrimSpace(command.ConversationUserID),
			strings.TrimSpace(command.BeforeMessageID),
			command.Limit,
		)
		if err != nil {
			_ = client.enqueueMessage("chat.history.error", map[string]any{
				"requestId":          strings.TrimSpace(command.RequestID),
				"conversationUserId": strings.TrimSpace(command.ConversationUserID),
				"message":            "Could not load chat history.",
			})
			return
		}

		_ = client.enqueueMessage("chat.history.loaded", map[string]any{
			"requestId": strings.TrimSpace(command.RequestID),
			"history":   payload,
		})
	}()
}
