-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_route;

-- Устанавливаем search_path
SET search_path TO tpb_route;

-- Таблица перемещений между точками маршрута
CREATE TABLE IF NOT EXISTS route_transfers (
                                               id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                               route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
                                               from_waypoint_id UUID NOT NULL REFERENCES waypoints(id) ON DELETE CASCADE,
                                               to_waypoint_id UUID NOT NULL REFERENCES waypoints(id) ON DELETE CASCADE,
                                               duration_hours INT NOT NULL DEFAULT 0,
                                               duration_minutes INT NOT NULL DEFAULT 0,
                                               transport_type VARCHAR(20) NOT NULL DEFAULT 'car',
                                               order_index INT NOT NULL DEFAULT 0,
                                               created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                               updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_route_transfers_route_id ON route_transfers (route_id);
CREATE INDEX IF NOT EXISTS idx_route_transfers_from_waypoint_id ON route_transfers (from_waypoint_id);
CREATE INDEX IF NOT EXISTS idx_route_transfers_to_waypoint_id ON route_transfers (to_waypoint_id);
CREATE INDEX IF NOT EXISTS idx_route_transfers_created_at ON route_transfers (created_at);
CREATE INDEX IF NOT EXISTS idx_route_transfers_updated_at ON route_transfers (updated_at);