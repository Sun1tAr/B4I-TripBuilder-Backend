-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_route;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_route;

-- Таблица маршрутов
CREATE TABLE IF NOT EXISTS routes (
                                      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                      title VARCHAR(255) NOT NULL,
                                      description TEXT,
                                      country VARCHAR(100),
                                      image_url TEXT,
                                      user_id UUID NOT NULL,
                                      is_public BOOLEAN DEFAULT false,
                                      total_days INT,
                                      total_hours INT,
                                      likes_count INT DEFAULT 0,
                                      events_count INT DEFAULT 0,
                                      created_at TIMESTAMP DEFAULT now(),
                                      updated_at TIMESTAMP
);

-- Индексы для поиска
CREATE INDEX IF NOT EXISTS idx_routes_user_id ON routes(user_id);
CREATE INDEX IF NOT EXISTS idx_routes_country ON routes(country);
CREATE INDEX IF NOT EXISTS idx_routes_is_public ON routes(is_public);
CREATE INDEX IF NOT EXISTS idx_routes_created_at ON routes(created_at);
CREATE INDEX IF NOT EXISTS idx_routes_likes_count ON routes(likes_count);

-- Таблица точек маршрута
CREATE TABLE IF NOT EXISTS waypoints (
                                         id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                         route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
                                         city VARCHAR(255) NOT NULL,
                                         description TEXT,
                                         latitude DECIMAL(10,7) NOT NULL,
                                         longitude DECIMAL(10,7) NOT NULL,
                                         duration_days INT DEFAULT 0,
                                         duration_hours INT DEFAULT 0,
                                         order_index INT NOT NULL,
                                         created_at TIMESTAMP DEFAULT now(),
                                         updated_at TIMESTAMP
);

-- Индекс для быстрого получения точек маршрута
CREATE INDEX IF NOT EXISTS idx_waypoints_route_id ON waypoints(route_id);
CREATE INDEX IF NOT EXISTS idx_waypoints_order_index ON waypoints(route_id, order_index);

-- Таблица тегов
CREATE TABLE IF NOT EXISTS tags (
                                    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                    name VARCHAR(50) UNIQUE NOT NULL,
                                    routes_count INT DEFAULT 0,
                                    created_at TIMESTAMP DEFAULT now()
);

-- Индекс для поиска по имени тега
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

-- Таблица связи маршрутов и тегов
CREATE TABLE IF NOT EXISTS route_tags (
                                          route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
                                          tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
                                          created_at TIMESTAMP DEFAULT now(),
                                          PRIMARY KEY (route_id, tag_id)
);

-- Таблица лайков
CREATE TABLE IF NOT EXISTS route_likes (
                                           user_id UUID NOT NULL,
                                           route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
                                           created_at TIMESTAMP DEFAULT now(),
                                           PRIMARY KEY (user_id, route_id)
);

-- Индекс для подсчёта лайков
CREATE INDEX IF NOT EXISTS idx_route_likes_route_id ON route_likes(route_id);