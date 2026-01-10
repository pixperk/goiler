package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512 KB
)

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	UserID string
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	rooms  map[string]bool
	logger *slog.Logger
}

// NewClient creates a new client instance
func NewClient(hub *Hub, conn *websocket.Conn, userID string, logger *slog.Logger) *Client {
	return &Client{
		ID:     uuid.New().String(),
		UserID: userID,
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		logger: logger,
	}
}

// Message represents a WebSocket message
type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Encode encodes the message to JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage decodes a JSON message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("websocket read error",
					slog.String("client_id", c.ID),
					slog.String("error", err.Error()),
				)
			}
			break
		}

		message, err := DecodeMessage(data)
		if err != nil {
			c.logger.Warn("invalid message format",
				slog.String("client_id", c.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		c.handleMessage(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(message *Message) {
	switch message.Type {
	case "join":
		var payload struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err == nil && payload.Room != "" {
			c.hub.joinRoom <- &RoomRequest{Client: c, Room: payload.Room}
		}

	case "leave":
		var payload struct {
			Room string `json:"room"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err == nil && payload.Room != "" {
			c.hub.leaveRoom <- &RoomRequest{Client: c, Room: payload.Room}
		}

	case "broadcast":
		// Broadcast to all clients
		c.hub.broadcast <- message

	case "room":
		// Broadcast to room
		if message.Room != "" {
			c.hub.BroadcastToRoom(message.Room, message)
		}

	case "ping":
		// Respond with pong
		response := &Message{Type: "pong"}
		if data, err := response.Encode(); err == nil {
			c.send <- data
		}

	default:
		c.logger.Debug("unknown message type",
			slog.String("type", message.Type),
			slog.String("client_id", c.ID),
		)
	}
}

// Send sends a message to the client
func (c *Client) Send(message *Message) error {
	data, err := message.Encode()
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return ErrBufferFull
	}
}

// JoinRoom joins a room
func (c *Client) JoinRoom(room string) {
	c.hub.joinRoom <- &RoomRequest{Client: c, Room: room}
}

// LeaveRoom leaves a room
func (c *Client) LeaveRoom(room string) {
	c.hub.leaveRoom <- &RoomRequest{Client: c, Room: room}
}

// GetRooms returns the rooms the client is in
func (c *Client) GetRooms() []string {
	rooms := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}
