-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_event;

-- Удаляем таблицы в обратном порядке (из-за зависимостей)
DROP TABLE IF EXISTS waypoint_comments;
DROP TABLE IF EXISTS waypoint_media;
DROP TABLE IF EXISTS event_waypoints;
DROP TABLE IF EXISTS event_participants;
DROP TABLE IF EXISTS events;

-- Примечание: схему не удаляем, так как в ней могут быть другие объекты