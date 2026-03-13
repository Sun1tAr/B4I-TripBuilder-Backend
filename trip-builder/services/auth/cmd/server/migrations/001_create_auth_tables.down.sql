-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_auth;

-- Удаляем таблицы в обратном порядке (из-за зависимостей)
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

-- Примечание: схему не удаляем, так как в ней могут быть другие объекты