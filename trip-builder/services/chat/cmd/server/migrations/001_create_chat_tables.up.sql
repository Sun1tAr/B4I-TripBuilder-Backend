-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_chat;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_chat;

-- Таблица сообщений
CREATE TABLE IF NOT EXISTS messages (
                                        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                        event_id UUID NOT NULL,
                                        user_id UUID NOT NULL,
                                        text TEXT NOT NULL,
                                        type VARCHAR(20) DEFAULT 'text', -- text, system, task_created, media_uploaded, waypoint_visited
                                        metadata JSONB, -- дополнительная информация (id задачи, id медиа, имя пользователя)
                                        created_at TIMESTAMP DEFAULT now(),
                                        updated_at TIMESTAMP
);

-- Индексы для поиска
CREATE INDEX IF NOT EXISTS idx_messages_event_id ON messages(event_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(event_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_type ON messages(type);

-- Таблица прочтений сообщений
CREATE TABLE IF NOT EXISTS read_receipts (
                                             message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
                                             user_id UUID NOT NULL,
                                             read_at TIMESTAMP DEFAULT now(),
                                             PRIMARY KEY (message_id, user_id)
);

-- Индекс для подсчёта непрочитанных
CREATE INDEX IF NOT EXISTS idx_read_receipts_user_id ON read_receipts(user_id);