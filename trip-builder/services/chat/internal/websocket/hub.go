package websocket

import (
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	ID     string
	UserID uuid.UUID
	RoomID uuid.UUID
	Send   chan []byte
	Hub    *Hub
	closed bool       // ← ДОБАВЛЯЕМ
	mu     sync.Mutex // ← ДОБАВЛЯЕМ
}

type Hub struct {
	rooms      map[uuid.UUID]map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage
	mu         sync.RWMutex
}

type BroadcastMessage struct {
	EventID uuid.UUID
	Data    []byte
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.rooms[client.RoomID]; !ok {
				h.rooms[client.RoomID] = make(map[string]*Client)
			}
			h.rooms[client.RoomID][client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.rooms[client.RoomID]; ok {
				delete(clients, client.ID)
				if len(clients) == 0 {
					delete(h.rooms, client.RoomID)
				}
			}
			// Безопасное закрытие канала
			client.mu.Lock()
			if !client.closed {
				client.closed = true
				close(client.Send)
			}
			client.mu.Unlock()
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients, ok := h.rooms[msg.EventID]
			if !ok {
				h.mu.RUnlock()
				continue
			}
			for _, client := range clients {
				select {
				case client.Send <- msg.Data:
				default:
					// Не закрываем канал здесь — Hub.Run сам это сделает при unregister
				}
			}
			h.mu.RUnlock()
		}
	}
}
