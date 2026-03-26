SET search_path TO tpb_event;

-- Удаляем индексы
DROP INDEX IF EXISTS idx_event_participants_invites;
DROP INDEX IF EXISTS idx_event_participants_active;

-- Восстанавливаем колонку status
ALTER TABLE event_participants ADD COLUMN status VARCHAR(20) DEFAULT 'pending';

-- Восстанавливаем данные из флагов
-- Полностью подтверждённые (оба флага true)
UPDATE event_participants
SET status = 'accepted'
WHERE approved_by_owner = true AND approved_by_user = true;

-- Приглашения/заявки (один флаг true, другой false)
UPDATE event_participants
SET status = 'accepted'
WHERE (approved_by_owner = true AND approved_by_user = false)
   OR (approved_by_owner = false AND approved_by_user = true);

-- Все остальные (оба false)
UPDATE event_participants
SET status = 'pending'
WHERE approved_by_owner = false AND approved_by_user = false;

-- Обновляем статус для владельцев (они всегда приняты)
UPDATE event_participants
SET status = 'accepted'
WHERE role = 'owner';

-- Удаляем флаги
ALTER TABLE event_participants DROP COLUMN approved_by_owner;
ALTER TABLE event_participants DROP COLUMN approved_by_user;