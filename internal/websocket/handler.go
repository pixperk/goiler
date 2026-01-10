package websocket

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/pixperk/goiler/internal/auth"
)

var (
	ErrBufferFull       = errors.New("client buffer full")
	ErrConnectionClosed = errors.New("connection closed")
)

// Handler handles WebSocket connections
type Handler struct {
	hub      *Hub
	upgrader websocket.Upgrader
	logger   *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, logger *slog.Logger) *Handler {
	return &Handler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
		},
		logger: logger,
	}
}

// HandleConnection handles WebSocket connection upgrades
// @Summary WebSocket connection
// @Description Upgrade to WebSocket connection
// @Tags WebSocket
// @Produce json
// @Success 101 "Switching Protocols"
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /api/v1/ws [get]
func (h *Handler) HandleConnection(c echo.Context) error {
	// Get user ID from auth context (optional - can be anonymous)
	userID := ""
	if payload := auth.GetCurrentUser(c); payload != nil {
		userID = payload.UserID.String()
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.String("error", err.Error()))
		return err
	}

	// Create new client
	client := NewClient(h.hub, conn, userID, h.logger)

	// Register client with hub
	h.hub.register <- client

	// Send welcome message
	welcome := &Message{
		Type: "connected",
		Payload: []byte(`{"message": "Connected to WebSocket server", "client_id": "` + client.ID + `"}`),
	}
	if data, err := welcome.Encode(); err == nil {
		client.send <- data
	}

	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()

	return nil
}

// HandleAuthenticatedConnection handles WebSocket connections requiring authentication
func (h *Handler) HandleAuthenticatedConnection(c echo.Context) error {
	payload := auth.GetCurrentUser(c)
	if payload == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.String("error", err.Error()))
		return err
	}

	client := NewClient(h.hub, conn, payload.UserID.String(), h.logger)
	h.hub.register <- client

	welcome := &Message{
		Type: "connected",
		Payload: []byte(`{"message": "Connected to WebSocket server", "client_id": "` + client.ID + `", "user_id": "` + payload.UserID.String() + `"}`),
	}
	if data, err := welcome.Encode(); err == nil {
		client.send <- data
	}

	go client.WritePump()
	go client.ReadPump()

	return nil
}

// BroadcastToAll broadcasts a message to all connected clients
func (h *Handler) BroadcastToAll(messageType string, payload interface{}) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}

	h.hub.BroadcastToAll(&Message{
		Type:    messageType,
		Payload: data,
	})
	return nil
}

// BroadcastToRoom broadcasts a message to all clients in a room
func (h *Handler) BroadcastToRoom(room, messageType string, payload interface{}) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}

	h.hub.BroadcastToRoom(room, &Message{
		Type:    messageType,
		Payload: data,
	})
	return nil
}

// BroadcastToUser broadcasts a message to a specific user
func (h *Handler) BroadcastToUser(userID, messageType string, payload interface{}) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}

	h.hub.BroadcastToUser(userID, &Message{
		Type:    messageType,
		Payload: data,
	})
	return nil
}

// GetStats returns WebSocket statistics
func (h *Handler) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"connected_clients": h.hub.GetConnectedClients(),
	}
}

// encodePayload encodes a payload to JSON
func encodePayload(payload interface{}) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	switch p := payload.(type) {
	case []byte:
		return p, nil
	case string:
		return []byte(p), nil
	default:
		return nil, errors.New("unsupported payload type")
	}
}
