-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_route;

-- Удаляем таблицы в обратном порядке (из-за зависимостей)
DROP TABLE IF EXISTS route_likes;
DROP TABLE IF EXISTS route_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS waypoints;
DROP TABLE IF EXISTS routes;

-- Примечание: схему не удаляем, так как в ней могут быть другие объекты