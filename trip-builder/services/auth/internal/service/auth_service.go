package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/metrics"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserNotVerified = errors.New("email not verified")
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenExpired    = errors.New("token expired")
	ErrTokenRevoked    = errors.New("token revoked")
	ErrEmailExists     = errors.New("email already exists")
)

type AuthService struct {
	Cfg       *config.Config
	userRepo  repository.UserRepository
	tokenRepo repository.RefreshTokenRepository
	log       *logger.Logger
}

var authMetrics = metrics.NewBusinessMetrics("auth")

var (
	authRegistrationsTotal = authMetrics.NewCounter("auth_registrations_total", "Total user registrations")
	authLoginsTotal        = authMetrics.NewCounter("auth_logins_total", "Total successful logins")
	authFailedLoginsTotal  = authMetrics.NewCounter("auth_failed_logins_total", "Total failed login attempts")
)

func NewAuthService(
	cfg *config.Config,
	userRepo repository.UserRepository,
	tokenRepo repository.RefreshTokenRepository,
	log *logger.Logger,
) *AuthService {
	return &AuthService{
		Cfg:       cfg,
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		log:       log,
	}
}

// Register регистрация нового пользователя
func (s *AuthService) Register(ctx context.Context, email, password string) (*models.User, string, error) {
	s.log.Debug("Register: начало", "email", email)

	// Проверяем, существует ли пользователь
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.log.Error("Register: ошибка проверки существования", "error", err)
		return nil, "", err
	}
	if existing != nil {
		s.log.Debug("Register: email уже существует", "email", email)
		return nil, "", ErrEmailExists
	}

	// Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.Cfg.BCryptCost)
	if err != nil {
		s.log.Error("Register: ошибка хеширования пароля", "error", err)
		return nil, "", err
	}

	// Создаём пользователя
	user, err := s.userRepo.Create(ctx, email, string(hash))
	if err != nil {
		s.log.Error("Register: ошибка создания пользователя", "error", err)
		return nil, "", err
	}

	// Генерируем токен подтверждения email
	verifyToken, err := s.generateVerificationToken(user.ID)
	if err != nil {
		s.log.Error("Register: ошибка генерации токена", "error", err)
		return user, "", err
	}

	s.log.Info("Register: пользователь зарегистрирован", "user_id", user.ID, "email", email)

	authRegistrationsTotal.Inc()
	return user, verifyToken, nil
}

// Login аутентификация пользователя
func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	s.log.Debug("Login: начало", "email", email)

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		s.log.Error("Login: ошибка получения пользователя", "error", err)
		return "", "", err
	}
	if user == nil {
		s.log.Debug("Login: пользователь не найден", "email", email)
		return "", "", ErrUserNotFound
	}

	// Проверяем пароль
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		s.log.Debug("Login: неверный пароль", "email", email)
		return "", "", ErrInvalidPassword
	}

	// Проверяем подтверждение email
	if !user.IsVerified {
		s.log.Debug("Login: email не подтвержден", "email", email)
		return "", "", ErrUserNotVerified
	}

	// Обновляем last_login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Генерируем токены
	accessToken, err := s.generateAccessToken(user.ID)
	if err != nil {
		s.log.Error("Login: ошибка генерации access token", "error", err)
		return "", "", err
	}

	refreshToken, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		s.log.Error("Login: ошибка генерации refresh token", "error", err)
		return "", "", err
	}

	s.log.Info("Login: успешный вход", "user_id", user.ID, "email", email)

	authLoginsTotal.Inc()
	return accessToken, refreshToken, nil
}

// Refresh обновление access token по refresh token
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	s.log.Debug("Refresh: начало")

	token, err := s.tokenRepo.GetByToken(ctx, refreshToken)
	if err != nil {
		s.log.Error("Refresh: ошибка при поиске токена", "error", err)
		return "", "", err
	}
	if token == nil {
		s.log.Warn("Refresh: токен не найден в БД")
		return "", "", ErrInvalidToken
	}

	// Проверяем, не истёк ли
	now := time.Now()
	if now.After(token.ExpiresAt) {
		s.log.Info("Refresh: токен истёк, отзываем", "token_id", token.ID, "user_id", token.UserID)
		_ = s.tokenRepo.Revoke(ctx, refreshToken)
		return "", "", ErrTokenExpired
	}

	// Проверяем, не отозван ли
	if token.Revoked {
		s.log.Warn("Refresh: токен отозван", "token_id", token.ID, "user_id", token.UserID)
		return "", "", ErrTokenRevoked
	}

	// Отзываем старый refresh token
	if err := s.tokenRepo.Revoke(ctx, refreshToken); err != nil {
		s.log.Error("Refresh: ошибка при отзыве старого токена", "error", err, "token_id", token.ID)
		// Не фатально, продолжаем
	}

	// Генерируем новые токены
	accessToken, err := s.generateAccessToken(token.UserID)
	if err != nil {
		s.log.Error("Refresh: ошибка генерации access token", "error", err, "user_id", token.UserID)
		return "", "", err
	}

	newRefreshToken, err := s.generateRefreshToken(ctx, token.UserID)
	if err != nil {
		s.log.Error("Refresh: ошибка генерации refresh token", "error", err, "user_id", token.UserID)
		return "", "", err
	}

	s.log.Info("Refresh: токены обновлены", "user_id", token.UserID)

	return accessToken, newRefreshToken, nil
}

// Logout выход пользователя (отзыв refresh token)
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	s.log.Debug("Logout: начало")

	err := s.tokenRepo.Revoke(ctx, refreshToken)
	if err != nil {
		s.log.Error("Logout: ошибка отзыва токена", "error", err)
		return err
	}

	s.log.Info("Logout: успешный выход")
	return nil
}

// VerificationResult результат попытки верификации
type VerificationResult struct {
	Success      bool
	IsExpired    bool
	UserID       uuid.UUID
	UserEmail    string
	UserVerified bool
}

// VerifyEmail подтверждение email
func (s *AuthService) VerifyEmail(ctx context.Context, token string) (*VerificationResult, error) {
	s.log.Debug("VerifyEmail: начало проверки токена")

	userID, isExpired, err := s.parseVerificationToken(token)

	result := &VerificationResult{
		Success:   false,
		IsExpired: isExpired,
		UserID:    userID,
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.log.Error("VerifyEmail: ошибка получения пользователя", "error", err, "user_id", userID)
		return nil, err
	}
	if user == nil {
		s.log.Debug("VerifyEmail: пользователь не найден", "user_id", userID)
		return nil, ErrUserNotFound
	}

	result.UserEmail = user.Email
	result.UserVerified = user.IsVerified

	// Если токен истек
	if isExpired {
		s.log.Info("VerifyEmail: токен истек", "user_id", userID, "email", user.Email)
		return result, nil
	}

	// Если ошибка парсинга (не истек, но невалидный)
	if err != nil {
		s.log.Warn("VerifyEmail: невалидный токен", "error", err, "user_id", userID)
		return nil, err
	}

	// Если пользователь уже подтвержден
	if user.IsVerified {
		s.log.Info("VerifyEmail: email уже подтвержден", "user_id", userID)
		result.Success = true
		return result, nil
	}

	// Подтверждаем email
	if err := s.userRepo.VerifyEmail(ctx, userID); err != nil {
		s.log.Error("VerifyEmail: ошибка подтверждения", "error", err, "user_id", userID)
		return nil, err
	}

	result.Success = true
	s.log.Info("VerifyEmail: email успешно подтвержден", "user_id", userID)

	return result, nil
}

// ResendVerificationEmail отправляет новое письмо с подтверждением
func (s *AuthService) ResendVerificationEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	s.log.Debug("ResendVerificationEmail: генерация нового токена", "user_id", userID)

	// Генерируем новый токен
	newToken, err := s.generateVerificationToken(userID)
	if err != nil {
		s.log.Error("ResendVerificationEmail: ошибка генерации токена", "error", err)
		return "", err
	}

	s.log.Info("ResendVerificationEmail: новый токен сгенерирован", "user_id", userID)

	return newToken, nil
}

// RequestPasswordReset запрос сброса пароля
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	s.log.Debug("RequestPasswordReset: начало", "email", email)

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		// Не сообщаем, существует ли пользователь (безопасность)
		s.log.Debug("RequestPasswordReset: пользователь не найден или ошибка", "email", email)
		return "", nil
	}

	resetToken, err := s.generateResetToken(user.ID)
	if err != nil {
		s.log.Error("RequestPasswordReset: ошибка генерации токена", "error", err)
		return "", err
	}

	s.log.Info("RequestPasswordReset: токен сброса сгенерирован", "user_id", user.ID)

	return resetToken, nil
}

// ResetPassword сброс пароля
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	s.log.Debug("ResetPassword: начало")

	userID, err := s.parseResetToken(token)
	if err != nil {
		s.log.Warn("ResetPassword: невалидный токен", "error", err)
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.Cfg.BCryptCost)
	if err != nil {
		s.log.Error("ResetPassword: ошибка хеширования пароля", "error", err)
		return err
	}

	// Отзываем все refresh токены пользователя
	_ = s.tokenRepo.RevokeAllForUser(ctx, userID)

	if err := s.userRepo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		s.log.Error("ResetPassword: ошибка обновления пароля", "error", err)
		return err
	}

	s.log.Info("ResetPassword: пароль успешно сброшен", "user_id", userID)

	return nil
}

// ValidateToken проверка JWT для gateway
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.Cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return userID, nil
}

// GetUserByID получение пользователя по ID
func (s *AuthService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

// GetUserByEmail получает пользователя по email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

// GetUserEmail получает email пользователя по ID
func (s *AuthService) GetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("user not found")
	}
	return user.Email, nil
}

// --- Вспомогательные методы ---

func (s *AuthService) generateAccessToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(s.Cfg.TokenLifetime).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *AuthService) generateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Генерируем случайный токен
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	_, err := s.tokenRepo.Create(ctx, userID, token, time.Now().Add(s.Cfg.RefreshLifetime))
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) generateVerificationToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(s.Cfg.VerificationTokenLifetime).Unix(),
		"type":    "email_verification",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *AuthService) parseVerificationToken(tokenString string) (uuid.UUID, bool, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.Cfg.JWTSecret), nil
	})

	// Проверяем, истек ли токен
	isExpired := false
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			isExpired = true
		} else {
			return uuid.Nil, false, ErrInvalidToken
		}
	}

	if token == nil && !isExpired {
		return uuid.Nil, false, ErrInvalidToken
	}

	// Если токен истек, все равно пытаемся извлечь claims
	var claims jwt.MapClaims
	if token != nil {
		claims, _ = token.Claims.(jwt.MapClaims)
	} else {
		// Парсим без проверки подписи для извлечения user_id из истекшего токена
		parser := jwt.Parser{}
		var unverifiedToken *jwt.Token
		unverifiedToken, _, _ = parser.ParseUnverified(tokenString, &claims)
		if unverifiedToken == nil {
			return uuid.Nil, isExpired, ErrInvalidToken
		}
	}

	if claims == nil {
		return uuid.Nil, isExpired, ErrInvalidToken
	}

	// Проверяем тип токена
	if claims["type"] != "email_verification" {
		return uuid.Nil, isExpired, ErrInvalidToken
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, isExpired, ErrInvalidToken
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, isExpired, ErrInvalidToken
	}

	return userID, isExpired, nil
}

func (s *AuthService) generateResetToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(s.Cfg.ResetPasswordTokenLifetime).Unix(),
		"type":    "password_reset",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *AuthService) parseResetToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.Cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	if claims["type"] != "password_reset" {
		return uuid.Nil, ErrInvalidToken
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	return uuid.Parse(userIDStr)
}
