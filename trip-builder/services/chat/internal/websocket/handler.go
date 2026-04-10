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
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	// Сначала пробуем из заголовка (от gateway)
	userIDStr := r.Header.Get("X-User-ID")

	// Если нет, пробуем из query-параметра (от браузера)
	if userIDStr == "" {
		userIDStr = r.URL.Query().Get("user_id")
	}

	if userIDStr == "" {
		h.log.Error("WebSocket: user_id не найден ни в заголовке, ни в query")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.log.Error("WebSocket: неверный user_id", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Получаем event_id из URL
	vars := mux.Vars(r)
	eventIDStr := vars["event_id"]
	if eventIDStr == "" {
		h.log.Error("WebSocket: event_id не указан")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		h.log.Error("WebSocket: неверный event_id", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Проверяем, что пользователь участник события
	isParticipant, err := h.chatService.IsEventParticipant(r.Context(), eventID, userID)
	if err != nil {
		h.log.Error("WebSocket: ошибка проверки участника", "error", err)
		return
	}
	if !isParticipant {
		h.log.Warn("WebSocket: пользователь не участник события", "user_id", userID, "event_id", eventID)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("WebSocket: ошибка апгрейда", "error", err)
		return
	}
	defer func() {
		h.log.Info("WebSocket: соединение закрыто", "user_id", userID, "event_id", eventID)
		conn.Close()
	}()

	clientID := uuid.New().String()
	client := &Client{
		ID:     clientID,
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    h.hub,
		RoomID: eventID,
	}

	// Сохраняем комнату клиента
	h.clientsMu.Lock()
	h.clientRoom[clientID] = eventID
	h.clientsMu.Unlock()

	// Регистрируем клиента в Hub
	h.hub.register <- client

	// Отправляем последние 50 сообщений
	go h.sendRecentMessages(conn, eventID)

	// Подписываемся на Redis канал комнаты для получения новых сообщений
	ctx := context.Background()
	pubsub, err := h.redisPubSub.Subscribe(ctx, eventID)
	if err != nil {
		h.log.Error("WebSocket: ошибка подписки на Redis", "error", err)
		return
	}
	defer pubsub.Close()

	// Канал для получения сообщений из Redis
	redisCh := pubsub.Channel()

	// Горoutine для отправки сообщений из Redis клиенту
	go func() {
		for msg := range redisCh {
			select {
			case client.Send <- []byte(msg.Payload):
			default:
				h.log.Warn("WebSocket: канал клиента переполнен", "client_id", clientID)
				return
			}
		}
	}()

	// Запускаем чтение сообщений (от клиента)
	go h.readPump(conn, client, eventID, userID)

	// Запускаем запись сообщений (клиенту) – этот метод читает из client.Send
	h.writePump(conn, client)
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
		conn.Close()
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
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
