-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Удаляем старый PRIMARY KEY
ALTER TABLE event_participants DROP CONSTRAINT event_participants_pkey;

-- Добавляем новый PRIMARY KEY на (event_id, invite_id)
ALTER TABLE event_participants ADD PRIMARY KEY (event_id, invite_id);

-- Добавляем UNIQUE для (event_id, user_id) для зарегистрированных пользователей
-- (для user_id = uuid.Nil это не будет блокировать)
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_participants_unique_user
    ON event_participants (event_id, user_id)
    WHERE user_id != '00000000-0000-0000-0000-000000000000';