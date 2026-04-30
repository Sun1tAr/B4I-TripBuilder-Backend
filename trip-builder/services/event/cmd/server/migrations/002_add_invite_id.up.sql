-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Добавляем поле invite_id
ALTER TABLE event_participants ADD COLUMN invite_id UUID DEFAULT gen_random_uuid();

-- Делаем его NOT NULL после добавления
ALTER TABLE event_participants ALTER COLUMN invite_id SET NOT NULL;

-- Создаём индекс для быстрого поиска
CREATE INDEX idx_event_participants_invite_id ON event_participants(invite_id);