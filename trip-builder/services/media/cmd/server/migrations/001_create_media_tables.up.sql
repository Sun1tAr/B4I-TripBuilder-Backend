-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_media;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_media;

-- Таблица медиа
CREATE TABLE IF NOT EXISTS media (
                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                     user_id UUID NOT NULL,
                                     event_id UUID,
                                     waypoint_id UUID,
                                     purpose VARCHAR(50) NOT NULL,
                                     filename VARCHAR(255) NOT NULL,
                                     object_key TEXT NOT NULL,
                                     thumbnail_key TEXT,
                                     medium_key TEXT,
                                     media_type VARCHAR(50) NOT NULL,
                                     file_size BIGINT NOT NULL,
                                     width INT,
                                     height INT,
                                     created_at TIMESTAMP DEFAULT now()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_media_user_id ON media(user_id);
CREATE INDEX IF NOT EXISTS idx_media_event_id ON media(event_id);
CREATE INDEX IF NOT EXISTS idx_media_waypoint_id ON media(waypoint_id);
CREATE INDEX IF NOT EXISTS idx_media_purpose ON media(purpose);
CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC);