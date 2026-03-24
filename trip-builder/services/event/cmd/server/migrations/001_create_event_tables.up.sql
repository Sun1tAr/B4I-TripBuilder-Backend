-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_event;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Таблица поездок (событий)
CREATE TABLE IF NOT EXISTS events (
                                      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                      route_id UUID NOT NULL,
                                      title VARCHAR(255) NOT NULL,
                                      description TEXT,
                                      owner_id UUID NOT NULL,
                                      start_date DATE NOT NULL,
                                      end_date DATE NOT NULL,
                                      is_group BOOLEAN DEFAULT false,
                                      status VARCHAR(20) DEFAULT 'planned', -- planned, active, completed, cancelled
                                      is_public BOOLEAN DEFAULT false,
                                      created_at TIMESTAMP DEFAULT now(),
                                      updated_at TIMESTAMP
);

-- Индексы для поиска
CREATE INDEX IF NOT EXISTS idx_events_owner_id ON events(owner_id);
CREATE INDEX IF NOT EXISTS idx_events_route_id ON events(route_id);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_start_date ON events(start_date);

-- Таблица участников поездки
CREATE TABLE IF NOT EXISTS event_participants (
                                                  event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                                                  user_id UUID NOT NULL,
                                                  role VARCHAR(20) DEFAULT 'member', -- owner, member
                                                  status VARCHAR(20) DEFAULT 'accepted', -- pending, accepted, declined
                                                  invited_email VARCHAR(255),
                                                  joined_at TIMESTAMP DEFAULT now(),
                                                  PRIMARY KEY (event_id, user_id)
);

-- Индекс для поиска по email приглашённого
CREATE INDEX IF NOT EXISTS idx_event_participants_invited_email ON event_participants(invited_email);

-- Таблица точек маршрута поездки (с фактическими датами)
CREATE TABLE IF NOT EXISTS event_waypoints (
                                               id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                               event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                                               route_waypoint_id UUID,
                                               city VARCHAR(255) NOT NULL,
                                               description TEXT,
                                               latitude DECIMAL(10,7),
                                               longitude DECIMAL(10,7),
                                               planned_start TIMESTAMP,
                                               planned_end TIMESTAMP,
                                               actual_start TIMESTAMP,
                                               actual_end TIMESTAMP,
                                               status VARCHAR(20) DEFAULT 'pending', -- pending, visited, skipped
                                               order_index INT NOT NULL,
                                               created_at TIMESTAMP DEFAULT now(),
                                               updated_at TIMESTAMP
);

-- Индекс для быстрого получения точек по событию
CREATE INDEX IF NOT EXISTS idx_event_waypoints_event_id ON event_waypoints(event_id);
CREATE INDEX IF NOT EXISTS idx_event_waypoints_order_index ON event_waypoints(event_id, order_index);

-- Таблица медиа (только метаданные, ссылки на MinIO)
CREATE TABLE IF NOT EXISTS waypoint_media (
                                              id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                              event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                                              waypoint_id UUID NOT NULL REFERENCES event_waypoints(id) ON DELETE CASCADE,
                                              user_id UUID NOT NULL,
                                              media_url TEXT NOT NULL,
                                              thumbnail_url TEXT,
                                              medium_url TEXT,
                                              media_type VARCHAR(20), -- photo, video
                                              description TEXT,
                                              created_at TIMESTAMP DEFAULT now()
);

-- Индексы для поиска медиа
CREATE INDEX IF NOT EXISTS idx_waypoint_media_event_id ON waypoint_media(event_id);
CREATE INDEX IF NOT EXISTS idx_waypoint_media_waypoint_id ON waypoint_media(waypoint_id);
CREATE INDEX IF NOT EXISTS idx_waypoint_media_user_id ON waypoint_media(user_id);

-- Таблица комментариев к точкам
CREATE TABLE IF NOT EXISTS waypoint_comments (
                                                 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                                 event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
                                                 waypoint_id UUID NOT NULL REFERENCES event_waypoints(id) ON DELETE CASCADE,
                                                 user_id UUID NOT NULL,
                                                 text TEXT NOT NULL,
                                                 created_at TIMESTAMP DEFAULT now(),
                                                 updated_at TIMESTAMP
);

-- Индексы для комментариев
CREATE INDEX IF NOT EXISTS idx_waypoint_comments_event_id ON waypoint_comments(event_id);
CREATE INDEX IF NOT EXISTS idx_waypoint_comments_waypoint_id ON waypoint_comments(waypoint_id);
CREATE INDEX IF NOT EXISTS idx_waypoint_comments_user_id ON waypoint_comments(user_id);