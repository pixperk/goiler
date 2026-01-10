package websocket

import (
	"log/slog"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Clients organized by room
	rooms map[string]map[*Client]bool

	// Inbound messages from clients
	broadcast chan *Message

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Join room requests
	joinRoom chan *RoomRequest

	// Leave room requests
	leaveRoom chan *RoomRequest

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Logger
	logger *slog.Logger
}

// RoomRequest represents a request to join or leave a room
type RoomRequest struct {
	Client *Client
	Room   string
}

// NewHub creates a new Hub instance
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		joinRoom:   make(chan *RoomRequest),
		leaveRoom:  make(chan *RoomRequest),
		logger:     logger,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case request := <-h.joinRoom:
			h.addClientToRoom(request.Client, request.Room)

		case request := <-h.leaveRoom:
			h.removeClientFromRoom(request.Client, request.Room)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	h.logger.Info("client registered",
		slog.String("client_id", client.ID),
		slog.String("user_id", client.UserID),
	)
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		// Remove from all rooms
		for room, clients := range h.rooms {
			if _, ok := clients[client]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.rooms, room)
				}
			}
		}

		h.logger.Info("client unregistered",
			slog.String("client_id", client.ID),
			slog.String("user_id", client.UserID),
		)
	}
}

// addClientToRoom adds a client to a room
func (h *Hub) addClientToRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.rooms[room] = true

	h.logger.Info("client joined room",
		slog.String("client_id", client.ID),
		slog.String("room", room),
	)
}

// removeClientFromRoom removes a client from a room
func (h *Hub) removeClientFromRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		delete(client.rooms, room)

		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}

	h.logger.Info("client left room",
		slog.String("client_id", client.ID),
		slog.String("room", room),
	)
}

// broadcastMessage sends a message to appropriate clients
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := message.Encode()
	if err != nil {
		h.logger.Error("failed to encode message", slog.String("error", err.Error()))
		return
	}

	// If room is specified, only send to clients in that room
	if message.Room != "" {
		if clients, ok := h.rooms[message.Room]; ok {
			for client := range clients {
				select {
				case client.send <- data:
				default:
					// Client's send buffer is full, skip
					h.logger.Warn("client buffer full, dropping message",
						slog.String("client_id", client.ID),
					)
				}
			}
		}
		return
	}

	// Broadcast to all clients
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			// Client's send buffer is full, skip
		}
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *Hub) BroadcastToAll(message *Message) {
	h.broadcast <- message
}

// BroadcastToRoom sends a message to all clients in a room
func (h *Hub) BroadcastToRoom(room string, message *Message) {
	message.Room = room
	h.broadcast <- message
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID string, message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := message.Encode()
	if err != nil {
		return
	}

	for client := range h.clients {
		if client.UserID == userID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// GetConnectedClients returns the number of connected clients
func (h *Hub) GetConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetRoomClients returns the number of clients in a room
func (h *Hub) GetRoomClients(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.rooms[room]; ok {
		return len(clients)
	}
	return 0
}
