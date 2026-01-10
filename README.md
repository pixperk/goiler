# Goiler

Production-ready Go backend boilerplate with Echo, sqlc, WebSocket, async workers, and OpenTelemetry.

## Quick Start

```bash
# 1. Clone and setup
git clone https://github.com/yourusername/goiler.git
cd goiler
make setup

# 2. Configure environment
make env
# Edit .env with your database/redis credentials

# 3. Start Postgres + Redis
docker-compose up -d

# 4. Run migrations and generate code
make migrate-up
make generate

# 5. Start the API
make run

# 6. Start the worker (separate terminal)
make run-worker
```

API runs at `http://localhost:8080`. Swagger docs at `/swagger/index.html`.

---

## Guide 1: Adding a CRUD Resource

Example: Adding a `Product` resource.

### Step 1: Create migration

```bash
make migrate-create name=create_products_table
```

Edit `db/migrations/XXXXXX_create_products_table.up.sql`:
```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

Run: `make migrate-up`

### Step 2: Write sqlc queries

Create `db/queries/product.sql`:
```sql
-- name: CreateProduct :one
INSERT INTO products (name, price) VALUES ($1, $2) RETURNING *;

-- name: GetProduct :one
SELECT * FROM products WHERE id = $1;

-- name: ListProducts :many
SELECT * FROM products ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: UpdateProduct :one
UPDATE products SET name = $2, price = $3, updated_at = NOW() WHERE id = $1 RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products WHERE id = $1;
```

Run: `make sqlc-generate`

### Step 3: Create domain layer

Create `internal/product/` directory with:

**model.go**
```go
package product

import (
    "time"
    "github.com/google/uuid"
)

type Product struct {
    ID        uuid.UUID
    Name      string
    Price     float64
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**repository.go**
```go
package product

import (
    "context"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/pixperk/goiler/db/sqlc"
)

type Repository struct {
    queries *sqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{queries: sqlc.New(db)}
}

func (r *Repository) Create(ctx context.Context, name string, price float64) (*Product, error) {
    p, err := r.queries.CreateProduct(ctx, sqlc.CreateProductParams{
        Name:  name,
        Price: price,
    })
    if err != nil {
        return nil, err
    }
    return &Product{ID: p.ID, Name: p.Name, Price: p.Price}, nil
}

// Implement GetByID, List, Update, Delete similarly...
```

**handler.go**
```go
package product

import (
    "net/http"
    "github.com/labstack/echo/v4"
    "github.com/pixperk/goiler/pkg/response"
)

type Handler struct {
    repo *Repository
}

func NewHandler(repo *Repository) *Handler {
    return &Handler{repo: repo}
}

func (h *Handler) Create(c echo.Context) error {
    var req struct {
        Name  string  `json:"name" validate:"required"`
        Price float64 `json:"price" validate:"required,gt=0"`
    }
    if err := c.Bind(&req); err != nil {
        return response.BadRequest(c, "Invalid request")
    }

    product, err := h.repo.Create(c.Request().Context(), req.Name, req.Price)
    if err != nil {
        return response.InternalError(c, "Failed to create product")
    }
    return response.Created(c, "Product created", product)
}

// Implement Get, List, Update, Delete handlers...
```

### Step 4: Register routes

In `cmd/api/main.go` or your routes file:
```go
productRepo := product.NewRepository(db)
productHandler := product.NewHandler(productRepo)

api := e.Group("/api/v1")
products := api.Group("/products")
products.POST("", productHandler.Create)
products.GET("/:id", productHandler.Get)
products.GET("", productHandler.List)
products.PUT("/:id", productHandler.Update)
products.DELETE("/:id", productHandler.Delete)
```

---

## Guide 2: WebSocket Real-time Features

### Basic Setup

WebSocket is already configured. Connect from client:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onopen = () => console.log('Connected');
ws.onmessage = (e) => console.log('Message:', JSON.parse(e.data));
ws.onclose = () => console.log('Disconnected');
```

### Join/Leave Rooms

```javascript
// Join a room
ws.send(JSON.stringify({ type: 'join', payload: { room: 'chat:general' } }));

// Leave a room
ws.send(JSON.stringify({ type: 'leave', payload: { room: 'chat:general' } }));
```

### Send Messages from Server

In any handler or service:

```go
// Broadcast to everyone
wsHandler.BroadcastToAll("notification", map[string]string{"msg": "Server update"})

// Broadcast to a room
wsHandler.BroadcastToRoom("chat:general", "message", map[string]any{
    "user": "john",
    "text": "Hello!",
})

// Send to specific user (by user ID from auth)
wsHandler.BroadcastToUser(userID, "private", map[string]string{"alert": "New follower"})
```

### Handle Custom Message Types

Edit `internal/websocket/handler.go`:

```go
func (h *Handler) handleMessage(client *Client, msg Message) {
    switch msg.Type {
    case "join":
        // existing join logic
    case "leave":
        // existing leave logic
    case "chat":  // Add custom type
        h.handleChat(client, msg.Payload)
    }
}

func (h *Handler) handleChat(client *Client, payload json.RawMessage) {
    var chat struct {
        Room string `json:"room"`
        Text string `json:"text"`
    }
    json.Unmarshal(payload, &chat)

    // Broadcast to room
    h.hub.BroadcastToRoom(chat.Room, "chat", map[string]any{
        "user": client.UserID,
        "text": chat.Text,
        "time": time.Now(),
    })
}
```

---

## Guide 2.5: WebSocket + PubSub (Multi-Instance)

For horizontal scaling where multiple API instances need to share WebSocket events.

### Architecture

```
Client A ──> API Instance 1 ──┐
                              ├──> Redis PubSub ──> All Instances
Client B ──> API Instance 2 ──┘
```

### Step 1: Create Redis PubSub Bridge

Create `internal/websocket/pubsub.go`:

```go
package websocket

import (
    "context"
    "encoding/json"
    "github.com/redis/go-redis/v9"
)

type PubSubBridge struct {
    redis  *redis.Client
    hub    *Hub
    channel string
}

func NewPubSubBridge(redisClient *redis.Client, hub *Hub) *PubSubBridge {
    return &PubSubBridge{
        redis:   redisClient,
        hub:     hub,
        channel: "ws:broadcast",
    }
}

type BroadcastMessage struct {
    Target  string          `json:"target"`  // "all", "room", "user"
    Room    string          `json:"room,omitempty"`
    UserID  string          `json:"user_id,omitempty"`
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// Publish broadcasts via Redis so all instances receive it
func (p *PubSubBridge) Publish(ctx context.Context, msg BroadcastMessage) error {
    data, _ := json.Marshal(msg)
    return p.redis.Publish(ctx, p.channel, data).Err()
}

// Subscribe listens for broadcasts from other instances
func (p *PubSubBridge) Subscribe(ctx context.Context) {
    sub := p.redis.Subscribe(ctx, p.channel)
    ch := sub.Channel()

    for msg := range ch {
        var broadcast BroadcastMessage
        if err := json.Unmarshal([]byte(msg.Payload), &broadcast); err != nil {
            continue
        }

        switch broadcast.Target {
        case "all":
            p.hub.BroadcastToAll(broadcast.Type, broadcast.Payload)
        case "room":
            p.hub.BroadcastToRoom(broadcast.Room, broadcast.Type, broadcast.Payload)
        case "user":
            p.hub.BroadcastToUser(broadcast.UserID, broadcast.Type, broadcast.Payload)
        }
    }
}
```

### Step 2: Initialize in main.go

```go
// After creating hub and redis client
pubsubBridge := websocket.NewPubSubBridge(redisClient, hub)

// Start subscriber in background
go pubsubBridge.Subscribe(context.Background())
```

### Step 3: Use PubSub for Cross-Instance Broadcasts

```go
// Instead of direct hub broadcast, use pubsub bridge
pubsubBridge.Publish(ctx, websocket.BroadcastMessage{
    Target:  "room",
    Room:    "chat:general",
    Type:    "message",
    Payload: json.RawMessage(`{"user":"john","text":"Hello!"}`),
})
```

### Using Go Channels PubSub (Single Instance)

For in-process pub/sub without Redis:

```go
import "github.com/pixperk/goiler/internal/channel"

// Create pubsub
pubsub := channel.NewPubSub(logger, 100)

// Subscribe to topics
sub := pubsub.Subscribe(ctx, "handler-1", "user.created", "order.placed")

// Listen in goroutine
go func() {
    for event := range sub.Channel {
        switch event.Topic {
        case "user.created":
            // notify connected WebSocket clients
            hub.BroadcastToAll("user_joined", event.Payload)
        case "order.placed":
            // notify specific user
            order := event.Payload.(Order)
            hub.BroadcastToUser(order.UserID, "order_update", order)
        }
    }
}()

// Publish from anywhere
pubsub.Publish("user.created", userData)
```

---

## Project Structure

```
goiler/
├── cmd/api/           # API entrypoint
├── cmd/worker/        # Async worker entrypoint
├── internal/
│   ├── auth/          # JWT/PASETO auth, password hashing
│   ├── channel/       # Go channels pub/sub
│   ├── config/        # Environment config
│   ├── server/        # Echo setup, middleware
│   ├── user/          # User domain example
│   ├── websocket/     # WebSocket hub & handlers
│   └── worker/        # Asynq task handlers
├── pkg/
│   ├── otel/          # OpenTelemetry setup
│   ├── response/      # API response helpers
│   └── validator/     # Request validation
├── db/
│   ├── migrations/    # SQL migrations
│   ├── queries/       # sqlc query files
│   └── sqlc/          # Generated code
└── docs/swagger/      # API documentation
```

## Common Commands

```bash
make run              # Run API server
make run-worker       # Run async worker
make dev              # Run with hot reload
make test             # Run tests
make lint             # Run linter
make generate         # Generate sqlc + swagger
make migrate-up       # Apply migrations
make migrate-down-one # Rollback one migration
make migrate-create name=xxx  # Create migration
make docker-up        # Start Postgres + Redis
make fresh            # Clean slate (reset DB)
```

## Auth Endpoints

```
POST /api/v1/auth/register  - Register user
POST /api/v1/auth/login     - Login, get tokens
POST /api/v1/auth/refresh   - Refresh access token
POST /api/v1/auth/logout    - Invalidate session
```

Protect routes:
```go
protected := api.Group("")
protected.Use(authHandler.AuthMiddleware())

// In handler, get current user:
user := auth.GetCurrentUser(c)
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `APP_PORT` | Server port (default: 8080) |
| `DATABASE_URL` | Postgres connection string |
| `REDIS_ADDR` | Redis address |
| `AUTH_TYPE` | `jwt` or `paseto` |
| `JWT_SECRET` | JWT signing key (32+ chars) |
| `OTEL_ENABLED` | Enable tracing (true/false) |

See `.env.example` for full list.

## License

MIT
