-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Удаляем таблицу
DROP TABLE IF EXISTS event_transfers;