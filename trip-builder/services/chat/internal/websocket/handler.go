package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/redis"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/chat/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var chatMetrics = metrics.NewBusinessMetrics("chat")

var (
	activeConnections = chatMetrics.NewGauge("active_connections", "Current active WebSocket connections")
)

type WSHandler struct {
	hub         *Hub
	chatService *service.ChatService
	redisPubSub *redis.PubSub
	log         *logger.Logger
	clientsMu   sync.RWMutex
	clientRoom  map[string]uuid.UUID
	upgrader    websocket.Upgrader
}

func NewWSHandler(hub *Hub, chatService *service.ChatService, redisPubSub *redis.PubSub, log *logger.Logger, allowedOrigins []string) *WSHandler {
	return &WSHandler{
		hub:         hub,
		chatService: chatService,
		redisPubSub: redisPubSub,
		log:         log,
		clientRoom:  make(map[string]uuid.UUID),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, allowed := range allowedOrigins {
					if allowed == "*" || allowed == origin {
						return true
					}
				}
				return false
			},
		},
	}
}

// ServeWS обрабатывает WebSocket подключение
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		userIDStr = r.URL.Query().Get("user_id")
	}
	if userIDStr == "" {
		activeConnections.Dec()
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		activeConnections.Dec()
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	eventIDStr := vars["event_id"]
	if eventIDStr == "" {
		activeConnections.Dec()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		activeConnections.Dec()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isParticipant, err := h.chatService.IsEventParticipant(r.Context(), eventID, userID)
	if err != nil || !isParticipant {
		activeConnections.Dec()
		w.WriteHeader(http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		activeConnections.Dec()
		return
	}
	activeConnections.Inc()
	h.log.Debug("WebSocket: соединение установлено", "user_id", userID)

	clientID := uuid.New().String()
	client := &Client{
		ID:     clientID,
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
		RoomID: eventID,
	}

	h.clientsMu.Lock()
	h.clientRoom[clientID] = eventID
	h.clientsMu.Unlock()

	h.hub.register <- client
	defer func() {
		h.hub.unregister <- client
	}()

	go h.sendRecentMessages(conn, eventID)

	ctx := context.Background()
	pubsub, err := h.redisPubSub.Subscribe(ctx, eventID)
	if err != nil {
		return
	}
	defer pubsub.Close()

	redisCh := pubsub.Channel()
	go func() {
		for msg := range redisCh {
			select {
			case client.Send <- []byte(msg.Payload):
			default:
				return
			}
		}
	}()

	// Запускаем readPump и writePump в горутинах
	go h.readPump(conn, client, eventID, userID)

	done := make(chan struct{})
	go func() {
		h.writePump(conn, client)
		close(done)
	}()

	// Ждем завершения writePump
	<-done

	conn.Close()
	activeConnections.Dec()
	h.log.Debug("WebSocket: соединение закрыто", "user_id", userID, "event_id", eventID)
}

// sendRecentMessages отправляет последние сообщения
func (h *WSHandler) sendRecentMessages(conn *websocket.Conn, eventID uuid.UUID) {
	messages, err := h.chatService.GetRecentMessages(context.Background(), eventID, 50)
	if err != nil {
		h.log.Error("WebSocket: ошибка получения последних сообщений", "error", err)
		return
	}

	response := map[string]interface{}{
		"type":     "history",
		"messages": messages,
	}
	data, _ := json.Marshal(response)
	conn.WriteMessage(websocket.TextMessage, data)
}

// readPump читает сообщения от клиента
func (h *WSHandler) readPump(conn *websocket.Conn, client *Client, eventID, userID uuid.UUID) {
	defer func() {
		h.hub.unregister <- client
	}()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}

		switch msgType {
		case "message":
			text, ok := msg["text"].(string)
			if !ok || text == "" {
				continue
			}

			// Сохраняем сообщение в БД
			savedMsg, err := h.chatService.SaveMessage(context.Background(), eventID, userID, text)
			if err != nil {
				h.log.Error("WebSocket: ошибка сохранения сообщения", "error", err)
				continue
			}

			// Публикуем в Redis для рассылки всем клиентам в комнате
			h.chatService.PublishMessage(context.Background(), eventID, savedMsg)

		case "ping":
			conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))

		case "read_receipt":
			h.chatService.MarkMessagesAsRead(context.Background(), eventID, userID)
		}
	}
}

// writePump отправляет сообщения клиенту
func (h *WSHandler) writePump(conn *websocket.Conn, client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				// Канал закрыт хабом — просто выходим, не шлем CloseMessage
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
