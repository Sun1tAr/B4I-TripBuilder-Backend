package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"bytes"
	"net/http"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/http/handlers/dto"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/ratelimit"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/service"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/utils"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/gorilla/mux"
)

type AuthHandler struct {
	authService *service.AuthService
	rateLimiter *ratelimit.RateLimiter
	log         *logger.Logger
}

func NewAuthHandler(authService *service.AuthService, rateLimiter *ratelimit.RateLimiter, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		rateLimiter: rateLimiter,
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Register регистрация нового пользователя
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Email == "" || req.Password == "" {
		h.writeError(w, http.StatusBadRequest, "email and password are required", "")
		return
	}
	if len(req.Password) < 6 {
		h.writeError(w, http.StatusBadRequest, "password must be at least 6 characters", "")
		return
	}

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

	h.log.Info("Пользователь зарегистрирован", "user_id", user.ID)
	h.writeJSON(w, http.StatusCreated, dto.UserResponse{
		ID:         user.ID,
		Email:      user.Email,
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
		// ЕДИНСТВЕННОЕ МЕСТО С ДЕТАЛЬНЫМ ЛОГИРОВАНИЕМ
		// Здесь логируем IP и информацию о превышении лимита
		h.log.Debug("Rate limit exceeded for IP",
			"ip", ip,
			"limit", h.rateLimiter.GetLimit(),
			"window_seconds", h.rateLimiter.GetWindow().Seconds(),
		)
		h.writeError(w, http.StatusTooManyRequests, "too many login attempts", "")
		return
	}

	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrInvalidPassword):
			// Неудачная попытка — счётчик уже увеличен, не сбрасываем
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
	h.writeJSON(w, http.StatusOK, dto.TokenResponse{
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

	var req dto.RefreshRequest
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
	}

	if resetToken != "" {
		h.log.Debug("Сгенерирован токен сброса пароля", "email", req.Email)
	}

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