-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Меняем тип колонок с DATE на TIMESTAMP
ALTER TABLE events
ALTER COLUMN start_date TYPE TIMESTAMP,
    ALTER COLUMN end_date TYPE TIMESTAMP;