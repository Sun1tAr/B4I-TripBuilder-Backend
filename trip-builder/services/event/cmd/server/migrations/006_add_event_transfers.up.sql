-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Таблица перемещений для поездок
CREATE TABLE IF NOT EXISTS event_transfers (
                                               id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                               event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                                               from_waypoint_id UUID NOT NULL REFERENCES event_waypoints(id) ON DELETE CASCADE,
                                               to_waypoint_id UUID NOT NULL REFERENCES event_waypoints(id) ON DELETE CASCADE,
                                               duration_hours INT NOT NULL DEFAULT 0,
                                               duration_minutes INT NOT NULL DEFAULT 0,
                                               transport_type VARCHAR(20) NOT NULL DEFAULT 'car',
                                               planned_start TIMESTAMP,
                                               planned_end TIMESTAMP,
                                               actual_start TIMESTAMP,
                                               actual_end TIMESTAMP,
                                               order_index INT NOT NULL DEFAULT 0,
                                               created_at TIMESTAMP DEFAULT now(),
                                               updated_at TIMESTAMP
);

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_event_transfers_event_id ON event_transfers(event_id);
CREATE INDEX IF NOT EXISTS idx_event_transfers_from_waypoint ON event_transfers(from_waypoint_id);
CREATE INDEX IF NOT EXISTS idx_event_transfers_to_waypoint ON event_transfers(to_waypoint_id);
CREATE INDEX IF NOT EXISTS idx_event_transfers_order_index ON event_transfers(event_id, order_index);