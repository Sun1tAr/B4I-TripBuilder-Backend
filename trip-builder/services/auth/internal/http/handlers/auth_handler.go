package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/gorilla/mux"
)

type AuthHandler struct {
	authService *service.AuthService
	log         *logger.Logger
}

func NewAuthHandler(authService *service.AuthService, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
	}
}

func (h *AuthHandler) writeError(w http.ResponseWriter, status int, err string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error:   err,
		Details: details,
	})
}

func (h *AuthHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Register регистрация нового пользователя
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Валидация
	if req.Email == "" || req.Password == "" {
		h.writeError(w, http.StatusBadRequest, "email and password are required", "")
		return
	}
	if len(req.Password) < 6 {
		h.writeError(w, http.StatusBadRequest, "password must be at least 6 characters", "")
		return
	}

	// TODO: заменить _ на verifyToken
	user, _, err := h.authService.Register(r.Context(), req.Email, req.Password)
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

	// TODO: отправить verifyToken через Notification Service (Kafka)

	h.log.Info("Пользователь зарегистрирован", "user_id", user.ID)
	h.writeJSON(w, http.StatusCreated, dto.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
		IsVerified: user.IsVerified,
		CreatedAt:  user.CreatedAt,
		LastLogin:  user.LastLogin,
	})
}

// Login вход пользователя
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			h.writeError(w, http.StatusUnauthorized, "invalid credentials", "")
		case errors.Is(err, service.ErrInvalidPassword):
			h.writeError(w, http.StatusUnauthorized, "invalid credentials", "")
		case errors.Is(err, service.ErrUserNotVerified):
			h.writeError(w, http.StatusForbidden, "email not verified", "")
		default:
			h.log.Error("Ошибка входа", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.writeJSON(w, http.StatusOK, dto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.authService.Cfg.TokenLifetime.Seconds()),
	})
}

// Refresh обновление access token
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("Refresh: ошибка декодирования запроса", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	h.log.Debug("Refresh: получен запрос", "refresh_token_length", len(req.RefreshToken))

	accessToken, refreshToken, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			h.log.Warn("Refresh: невалидный токен")
			h.writeError(w, http.StatusUnauthorized, "invalid refresh token", "")
		case errors.Is(err, service.ErrTokenExpired):
			h.log.Warn("Refresh: токен истёк")
			h.writeError(w, http.StatusUnauthorized, "refresh token expired", "")
		case errors.Is(err, service.ErrTokenRevoked):
			h.log.Warn("Refresh: токен отозван")
			h.writeError(w, http.StatusUnauthorized, "refresh token revoked", "")
		default:
			h.log.Error("Refresh: внутренняя ошибка", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	h.log.Info("Refresh: токены успешно обновлены")
	h.writeJSON(w, http.StatusOK, dto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.authService.Cfg.TokenLifetime.Seconds()),
	})
}

// Logout выход пользователя
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		h.log.Error("Ошибка выхода", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		return
	}

	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "successfully logged out"})
}

// VerifyEmail подтверждение email
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	if token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required", "")
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), token); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			h.writeError(w, http.StatusBadRequest, "invalid verification token", "")
		default:
			h.log.Error("Ошибка подтверждения email", "error", err)
			h.writeError(w, http.StatusInternalServerError, "internal server error", "")
		}
		return
	}

	// TODO: редирект на страницу успеха или просто сообщение
	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "email successfully verified"})
}

// RequestPasswordReset запрос сброса пароля
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	resetToken, err := h.authService.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		h.log.Error("Ошибка запроса сброса пароля", "error", err)
		// Не сообщаем об ошибке клиенту
	}

	if resetToken != "" {
		// TODO: отправить resetToken через Notification Service
		h.log.Info("Сгенерирован токен сброса пароля", "email", req.Email, "token", resetToken)
	}

	// Всегда возвращаем успех (безопасность)
	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "if email exists, reset link will be sent"})
}

// ResetPassword сброс пароля
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	var req dto.ResetPasswordConfirmRequest
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

	h.writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "password successfully reset"})
}

// GetMe получение информации о текущем пользователе
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// TODO: получить пользователя из сервиса
	// Пока заглушка
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