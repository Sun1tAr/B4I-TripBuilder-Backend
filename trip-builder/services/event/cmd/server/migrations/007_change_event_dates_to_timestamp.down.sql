-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Возвращаем тип колонок обратно на DATE
ALTER TABLE events
ALTER COLUMN start_date TYPE DATE,
    ALTER COLUMN end_date TYPE DATE;