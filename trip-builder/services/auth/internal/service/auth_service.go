package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserNotVerified   = errors.New("email not verified")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenRevoked      = errors.New("token revoked")
	ErrEmailExists       = errors.New("email already exists")
)

type AuthService struct {
	Cfg       *config.Config
	userRepo  repository.UserRepository
	tokenRepo repository.RefreshTokenRepository
	log       *logger.Logger
}

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
	// Проверяем, существует ли пользователь
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if existing != nil {
		return nil, "", ErrEmailExists
	}

	// Хешируем пароль
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.Cfg.BCryptCost)
	if err != nil {
		return nil, "", err
	}

	// Создаём пользователя
	user, err := s.userRepo.Create(ctx, email, string(hash))
	if err != nil {
		return nil, "", err
	}

	// Генерируем токен подтверждения email
	verifyToken, err := s.generateVerificationToken(user.ID)
	if err != nil {
		return user, "", err
	}

	return user, verifyToken, nil
}

// Login аутентификация пользователя
func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}
	if user == nil {
		return "", "", ErrUserNotFound
	}

	// Проверяем пароль
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", "", ErrInvalidPassword
	}

	// Проверяем подтверждение email
	if !user.IsVerified {
		return "", "", ErrUserNotVerified
	}

	// Обновляем last_login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Генерируем токены
	accessToken, err := s.generateAccessToken(user.ID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// Refresh обновление access token по refresh token
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	//s.log.Debug("Refresh: поиск токена в БД", "token", refreshToken)

	token, err := s.tokenRepo.GetByToken(ctx, refreshToken)
	if err != nil {
		s.log.Error("Refresh: ошибка при поиске токена", "error", err)
		return "", "", err
	}
	if token == nil {
		s.log.Warn("Refresh: токен не найден в БД", "token", refreshToken)
		return "", "", ErrInvalidToken
	}

	//s.log.Debug("Refresh: токен найден",
	//	"token_id", token.ID,
	//	"user_id", token.UserID,
	//	"expires_at", token.ExpiresAt,
	//	"revoked", token.Revoked,
	//	"created_at", token.CreatedAt,
	//)

	// Проверяем, не истёк ли
	now := time.Now()
	//s.log.Debug("Refresh: проверка срока действия",
	//	"now", now,
	//	"expires_at", token.ExpiresAt,
	//	"is_expired", now.After(token.ExpiresAt),
	//)

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


	return accessToken, newRefreshToken, nil
}

// Logout выход пользователя (отзыв refresh token)
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.tokenRepo.Revoke(ctx, refreshToken)
}

// VerifyEmail подтверждение email
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.parseVerificationToken(token)
	if err != nil {
		return err
	}
	return s.userRepo.VerifyEmail(ctx, userID)
}

// RequestPasswordReset запрос сброса пароля
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		// Не сообщаем, существует ли пользователь (безопасность)
		return "", nil
	}

	resetToken, err := s.generateResetToken(user.ID)
	if err != nil {
		return "", err
	}

	return resetToken, nil
}

// ResetPassword сброс пароля
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := s.parseResetToken(token)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.Cfg.BCryptCost)
	if err != nil {
		return err
	}

	// Отзываем все refresh токены пользователя
	_ = s.tokenRepo.RevokeAllForUser(ctx, userID)

	return s.userRepo.UpdatePassword(ctx, userID, string(hash))
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
	// Простой подход: JWT с user_id и коротким сроком жизни
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"type":    "email_verification",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *AuthService) parseVerificationToken(tokenString string) (uuid.UUID, error) {
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

	if claims["type"] != "email_verification" {
		return uuid.Nil, ErrInvalidToken
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}

	return uuid.Parse(userIDStr)
}

func (s *AuthService) generateResetToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
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