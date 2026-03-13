-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_user;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_user;

-- Таблица профилей
CREATE TABLE IF NOT EXISTS profiles (
                                        user_id UUID PRIMARY KEY,
                                        first_name VARCHAR(100),
                                        last_name VARCHAR(100),
                                        avatar_url TEXT,
                                        bio TEXT,
                                        phone VARCHAR(20),
                                        language VARCHAR(10) DEFAULT 'ru',
                                        timezone VARCHAR(50) DEFAULT 'UTC',
                                        notifications_enabled BOOLEAN DEFAULT true,
                                        created_at TIMESTAMP DEFAULT now(),
                                        updated_at TIMESTAMP
);

-- Таблица статистики
CREATE TABLE IF NOT EXISTS stats (
                                     user_id UUID PRIMARY KEY REFERENCES profiles(user_id) ON DELETE CASCADE,
                                     routes_created INT DEFAULT 0,
                                     events_done INT DEFAULT 0,
                                     countries_visited INT DEFAULT 0,
                                     photos_uploaded INT DEFAULT 0,
                                     followers INT DEFAULT 0,
                                     following INT DEFAULT 0
);

-- Таблица достижений
CREATE TABLE IF NOT EXISTS achievements (
                                            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                            user_id UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
                                            type VARCHAR(50) NOT NULL,
                                            title VARCHAR(100) NOT NULL,
                                            description TEXT,
                                            earned_at TIMESTAMP DEFAULT now()
);

-- Индекс для поиска достижений пользователя
CREATE INDEX IF NOT EXISTS idx_achievements_user_id ON achievements(user_id);

-- Таблица подписок
CREATE TABLE IF NOT EXISTS follows (
                                       follower_id UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
                                       followee_id UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
                                       created_at TIMESTAMP DEFAULT now(),
                                       PRIMARY KEY (follower_id, followee_id)
);

-- Индексы для быстрого получения подписчиков/подписок
CREATE INDEX IF NOT EXISTS idx_follows_followee_id ON follows(followee_id);
CREATE INDEX IF NOT EXISTS idx_follows_follower_id ON follows(follower_id);