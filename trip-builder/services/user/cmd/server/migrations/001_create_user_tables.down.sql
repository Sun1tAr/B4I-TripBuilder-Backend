-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_user;

-- Удаляем таблицы в обратном порядке (из-за зависимостей)
DROP TABLE IF EXISTS follows;
DROP TABLE IF EXISTS achievements;
DROP TABLE IF EXISTS stats;
DROP TABLE IF EXISTS profiles;

-- Примечание: схему не удаляем, так как в ней могут быть другие объекты