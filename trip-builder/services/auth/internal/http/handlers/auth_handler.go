package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/client/user"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/ratelimit"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/utils"
	authdto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/auth"
	commondto "github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/dto/common"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/kafka"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type AuthHandler struct {
	authService        *service.AuthService
	rateLimiter        *ratelimit.RateLimiter
	kafkaProducer      *kafka.Producer
	userClient         *user.Client
	passwordResetTopic string // ← ДОБАВЛЯЕМ
	log                *logger.Logger
}

func NewAuthHandler(
	authService *service.AuthService,
	rateLimiter *ratelimit.RateLimiter,
	kafkaProducer *kafka.Producer,
	userClient *user.Client,
	passwordResetTopic string, // ← ДОБАВЛЯЕМ
	log *logger.Logger,
) *AuthHandler {
	return &AuthHandler{
		authService:        authService,
		rateLimiter:        rateLimiter,
		kafkaProducer:      kafkaProducer,
		userClient:         userClient,
		passwordResetTopic: passwordResetTopic, // ← ДОБАВЛЯЕМ
		log:                log,
	}
}

func (h *AuthHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(commondto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *AuthHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Register регистрация нового пользователя
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authdto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Валидация...
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		h.writeError(w, http.StatusBadRequest, "email, password, firstName and lastName are required", "")
		return
	}
	if len(req.Password) < 6 {
		h.writeError(w, http.StatusBadRequest, "password must be at least 6 characters", "")
		return
	}

	// Создаем пользователя
	user, verificationToken, err := h.authService.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailExists):
			h.writeError(w, http.StatusConflict, "email already exists", "")
		default:
			h.log.Error("Ошибка регистрации", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// Отправляем событие в Kafka
	go func() {
		eventData := authdto.UserCreatedData{
			UserID:            user.ID,
			Email:             user.Email,
			FirstName:         req.FirstName,
			LastName:          req.LastName,
			VerificationToken: verificationToken,
		}

		event := kafka.Event{
			EventID:   uuid.New().String(),
			EventType: "user.created",
			Timestamp: time.Now(),
			Data:      eventData,
		}

		if err := h.kafkaProducer.Publish(context.Background(), event); err != nil {
			h.log.Error("Ошибка отправки user.created", "error", err, "user_id", user.ID)
		} else {
			h.log.Debug("Событие user.created отправлено", "user_id", user.ID, "email", user.Email)
		}
	}()

	h.log.Info("Пользователь зарегистрирован",
		"user_id", user.ID,
		"email", user.Email,
		"first_name", req.FirstName,
		"last_name", req.LastName,
	)

	h.writeJSON(w, http.StatusCreated, authdto.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		IsVerified: user.IsVerified,
		CreatedAt:  user.CreatedAt,
		LastLogin:  user.LastLogin,
	})
}

// Login вход пользователя с rate limiting по реальному IP
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Получаем реальный IP клиента (даже через gateway)
	ip := utils.GetRealIP(r)

	// Rate limiting по IP
	allowed, err := h.rateLimiter.Allow(r.Context(), ip)
	if err != nil {
		h.log.Error("Ошибка rate limiter", "error", err)
		// Пропускаем запрос при ошибке Redis, но логируем
	} else if !allowed {
		h.log.Debug("Rate limit exceeded for IP",
			"ip", ip,
			"limit", h.rateLimiter.GetLimit(),
			"window_seconds", h.rateLimiter.GetWindow().Seconds(),
		)
		h.writeError(w, http.StatusTooManyRequests, "too many login attempts", "")
		return
	}

	var req authdto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrInvalidPassword):
			h.writeError(w, http.StatusUnauthorized, "invalid credentials", "")
		case errors.Is(err, service.ErrUserNotVerified):
			h.writeError(w, http.StatusForbidden, "email not verified", "")
		default:
			h.log.Error("Ошибка входа", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// При успешном входе сбрасываем счётчик попыток для этого IP
	if err := h.rateLimiter.Reset(r.Context(), ip); err != nil {
		h.log.Error("Ошибка сброса rate limiter", "error", err)
	}

	h.log.Info("Успешный вход", "email", req.Email, "ip", ip)
	h.writeJSON(w, http.StatusOK, authdto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.authService.Cfg.TokenLifetime.Seconds()),
	})
}

// Refresh обновление access token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Читаем тело запроса для отладки
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var req authdto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Ошибка декодирования запроса", "error", err, "body", string(body))
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			h.writeError(w, http.StatusUnauthorized, "invalid refresh token", "")
		case errors.Is(err, service.ErrTokenExpired):
			h.writeError(w, http.StatusUnauthorized, "refresh token expired", "")
		case errors.Is(err, service.ErrTokenRevoked):
			h.writeError(w, http.StatusUnauthorized, "refresh token revoked", "")
		default:
			h.log.Error("Ошибка обновления токена", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.log.Info("Токены успешно обновлены")
	h.writeJSON(w, http.StatusOK, authdto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.authService.Cfg.TokenLifetime.Seconds()),
	})
}

// Logout выход пользователя
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req authdto.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		h.log.Error("Ошибка выхода", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, authdto.MessageResponse{Message: "successfully logged out"})
}

// VerifyEmail подтверждение email
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	if token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required", "")
		return
	}

	result, err := h.authService.VerifyEmail(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			h.writeError(w, http.StatusNotFound, "user not found", "")
		case errors.Is(err, service.ErrInvalidToken):
			h.writeError(w, http.StatusBadRequest, "invalid verification token", "")
		default:
			h.log.Error("Ошибка подтверждения email", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// Если токен истек — отправляем новое письмо
	if result.IsExpired {
		h.log.Info("VerifyEmail: токен истек, отправляем новое письмо",
			"user_id", result.UserID,
			"email", result.UserEmail,
		)

		// Генерируем новый токен
		newToken, err := h.authService.ResendVerificationEmail(r.Context(), result.UserID)
		if err != nil {
			h.log.Error("VerifyEmail: ошибка генерации нового токена", "error", err)
			h.writeJSON(w, http.StatusOK, authdto.MessageResponse{
				Message: "Verification link expired. A new verification email has been sent. Please check your inbox.",
			})
			return
		}

		// Получаем профиль из User Service
		firstName := ""
		lastName := ""
		profile, err := h.userClient.GetProfile(r.Context(), result.UserID)
		if err != nil {
			h.log.Error("VerifyEmail: ошибка получения профиля", "error", err, "user_id", result.UserID)
		} else if profile != nil {
			firstName = profile.FirstName
			lastName = profile.LastName
		}

		// Публикуем событие для отправки нового письма
		go func() {
			eventData := authdto.UserCreatedData{
				UserID:            result.UserID,
				Email:             result.UserEmail,
				FirstName:         firstName,
				LastName:          lastName,
				VerificationToken: newToken,
			}

			event := kafka.Event{
				EventID:   uuid.New().String(),
				EventType: "user.created",
				Timestamp: time.Now(),
				Data:      eventData,
			}

			if err := h.kafkaProducer.Publish(context.Background(), event); err != nil {
				h.log.Error("Ошибка отправки user.created для повторной верификации",
					"error", err,
					"user_id", result.UserID,
				)
			}
		}()

		h.writeJSON(w, http.StatusBadRequest, authdto.MessageResponse{
			Message: "Verification link expired. A new verification email has been sent. Please check your inbox.",
		})
		return
	}

	// Если пользователь уже подтвержден
	if result.UserVerified {
		h.writeJSON(w, http.StatusOK, authdto.MessageResponse{
			Message: "Email already verified. You can now login.",
		})
		return
	}

	// Успешное подтверждение
	h.writeJSON(w, http.StatusOK, authdto.MessageResponse{
		Message: "Email successfully verified. You can now login.",
	})
}

// RequestPasswordReset запрос сброса пароля
// Работает в двух режимах:
// 1. Без авторизации + email в теле → для "Забыл пароль"
// 2. С авторизацией + пустое тело → для "Сменить пароль" (берет email из токена)
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req authdto.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	var email string
	var userID uuid.UUID
	var firstName string

	// Пробуем получить user_id из контекста (если пользователь авторизован через AuthOptional)
	userIDStr, _ := r.Context().Value("user_id").(string)

	if req.Email != "" {
		// Сценарий 1: "Забыл пароль" — email передан в теле запроса
		email = req.Email
		user, err := h.authService.GetUserByEmail(r.Context(), email)
		if err == nil && user != nil {
			userID = user.ID
		}
	} else if userIDStr != "" {
		// Сценарий 2: "Сменить пароль" — пользователь авторизован, берем email из его профиля
		uid, err := uuid.Parse(userIDStr)
		if err == nil {
			user, err := h.authService.GetUserByID(r.Context(), uid)
			if err == nil && user != nil {
				userID = uid
				email = user.Email
			}
		}
	}

	// Если email не определен — не раскрываем информацию
	if email == "" {
		h.writeJSON(w, http.StatusOK, authdto.MessageResponse{
			Message: "If email exists, reset link will be sent",
		})
		return
	}

	// Получаем имя пользователя из User Service
	if userID != uuid.Nil {
		profile, err := h.userClient.GetProfile(r.Context(), userID)
		if err == nil && profile != nil {
			firstName = profile.FirstName
		}
	}

	// Генерируем токен сброса
	resetToken, err := h.authService.RequestPasswordReset(r.Context(), email)
	if err != nil {
		h.log.Error("Ошибка запроса сброса пароля", "error", err)
	}

	if resetToken != "" {
		h.log.Info("Сгенерирован токен сброса пароля", "email", email)

		// Публикуем событие в Kafka для отправки письма
		go func() {
			eventData := authdto.PasswordResetData{
				UserID:     userID,
				Email:      email,
				FirstName:  firstName,
				ResetToken: resetToken,
			}
			event := kafka.Event{
				EventID:   uuid.New().String(),
				EventType: "password.reset",
				Timestamp: time.Now(),
				Data:      eventData,
			}
			if err := h.kafkaProducer.PublishTo(context.Background(), h.passwordResetTopic, event); err != nil {
				h.log.Error("Ошибка отправки password.reset", "error", err, "email", email)
			} else {
				h.log.Info("Событие password.reset отправлено", "email", email)
			}
		}()
	}

	h.writeJSON(w, http.StatusOK, authdto.MessageResponse{
		Message: "If email exists, reset link will be sent",
	})
}

// ResetPassword сброс пароля
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	var req authdto.ResetPasswordConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if token == "" {
		token = req.Token
	}
	if token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required", "")
		return
	}
	if len(req.NewPassword) < 6 {
		h.writeError(w, http.StatusBadRequest, "password must be at least 6 characters", "")
		return
	}

	if err := h.authService.ResetPassword(r.Context(), token, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			h.writeError(w, http.StatusBadRequest, "invalid reset token", "")
		default:
			h.log.Error("Ошибка сброса пароля", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, authdto.MessageResponse{Message: "password successfully reset"})
}

// GetMe получение информации о текущем пользователе
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Можно получить полную информацию о пользователе из БД
	// Но пока возвращаем только ID
	h.writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

// Validate для gateway (возвращает user_id)
func (h *AuthHandler) Validate(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

// GetUserByEmail получает пользователя по email (внутренний эндпоинт для Event Service)
func (h *AuthHandler) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		h.writeError(w, http.StatusBadRequest, "email is required", "")
		return
	}

	h.log.Debug("GetUserByEmail: запрос", "email", email)

	user, err := h.authService.GetUserByEmail(r.Context(), email)
	if err != nil {
		h.log.Error("GetUserByEmail: ошибка", "error", err, "email", email)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	if user == nil {
		h.writeJSON(w, http.StatusNotFound, map[string]string{"message": "user not found"})
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// GetUserEmail получает email пользователя по ID (внутренний эндпоинт для Event Service)
func (h *AuthHandler) GetUserEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid user id", "")
		return
	}

	h.log.Debug("GetUserEmail: запрос", "user_id", userID)

	email, err := h.authService.GetUserEmail(r.Context(), userID)
	if err != nil {
		h.log.Error("GetUserEmail: ошибка", "error", err, "user_id", userID)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{
		"email": email,
	})
}
