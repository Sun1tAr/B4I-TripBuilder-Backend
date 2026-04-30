-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_auth;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_auth;

-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                     email VARCHAR(255) UNIQUE NOT NULL,
                                     password_hash VARCHAR(255) NOT NULL,
                                     is_verified BOOLEAN DEFAULT false,
                                     created_at TIMESTAMP DEFAULT now(),
                                     last_login TIMESTAMP
);

-- Индекс для поиска по email
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Таблица refresh токенов
CREATE TABLE IF NOT EXISTS refresh_tokens (
                                              id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                              user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                              token VARCHAR(255) UNIQUE NOT NULL,
                                              expires_at TIMESTAMP NOT NULL,
                                              revoked BOOLEAN DEFAULT false,
                                              created_at TIMESTAMP DEFAULT now()
);

-- Индекс для поиска по токену
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);

-- Индекс для поиска по user_id (для отзыва всех)
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Индекс для удаления просроченных
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);