-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Добавляем новые колонки
ALTER TABLE event_participants
    ADD COLUMN approved_by_owner BOOLEAN DEFAULT false,
    ADD COLUMN approved_by_user BOOLEAN DEFAULT null;

-- Переносим данные из status
-- Владельцы (role = 'owner') со статусом 'accepted' → approved_by_owner = true
UPDATE event_participants
SET approved_by_owner = true
WHERE status = 'accepted' AND role = 'owner';

-- Участники (role = 'member') со статусом 'accepted' → approved_by_user = true
UPDATE event_participants
SET approved_by_user = true
WHERE status = 'accepted' AND role = 'member';

-- Записи со статусом 'pending' остаются по умолчанию

-- Записи со статусом 'declined'
UPDATE event_participants
SET approved_by_user = false
WHERE status = 'declined';

-- Удаляем старую колонку
ALTER TABLE event_participants DROP COLUMN status;

-- Добавляем индекс для быстрого поиска приглашений
CREATE INDEX IF NOT EXISTS idx_event_participants_invites
    ON event_participants (approved_by_owner, approved_by_user)
    WHERE approved_by_owner = true AND approved_by_user = false;

-- Индекс для поиска участников (оба флага true)
CREATE INDEX IF NOT EXISTS idx_event_participants_active
    ON event_participants (event_id, user_id)
    WHERE approved_by_owner = true AND approved_by_user = true;