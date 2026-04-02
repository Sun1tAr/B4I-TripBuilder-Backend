-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_task;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_task;

-- Таблица шаблонов задач
CREATE TABLE IF NOT EXISTS task_templates (
                                              id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                              title VARCHAR(255) NOT NULL,
                                              description TEXT,
                                              trigger_days INT NOT NULL, -- отрицательное = дней до старта, положительное = после старта
                                              priority VARCHAR(10) DEFAULT 'medium', -- high, medium, low
                                              created_at TIMESTAMP DEFAULT now()
);

-- Таблица задач
CREATE TABLE IF NOT EXISTS tasks (
                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                     event_id UUID NOT NULL,
                                     title VARCHAR(255) NOT NULL,
                                     description TEXT,
                                     assigned_to UUID, -- user_id; null означает "на всех"
                                     created_by UUID NOT NULL,
                                     priority VARCHAR(10) DEFAULT 'medium', -- high, medium, low
                                     status VARCHAR(20) DEFAULT 'pending', -- pending, done, cancelled
                                     due_date TIMESTAMP,
                                     completed_at TIMESTAMP,
                                     created_at TIMESTAMP DEFAULT now(),
                                     updated_at TIMESTAMP
);

-- Индексы для поиска
CREATE INDEX IF NOT EXISTS idx_tasks_event_id ON tasks(event_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks(due_date);

-- Таблица комментариев к задачам
CREATE TABLE IF NOT EXISTS task_comments (
                                             id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                             task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
                                             user_id UUID NOT NULL,
                                             text TEXT NOT NULL,
                                             created_at TIMESTAMP DEFAULT now()
);

-- Индекс для комментариев
CREATE INDEX IF NOT EXISTS idx_task_comments_task_id ON task_comments(task_id);

-- Начальные шаблоны задач
INSERT INTO task_templates (id, title, description, trigger_days, priority) VALUES
                                                                                (gen_random_uuid(), 'Забронировать отель', 'Забронировать отель в первом городе маршрута', -7, 'high'),
                                                                                (gen_random_uuid(), 'Купить билеты', 'Купить билеты на самолет/поезд', -14, 'high'),
                                                                                (gen_random_uuid(), 'Собрать документы', 'Проверить наличие загранпаспорта, виз', -30, 'medium'),
                                                                                (gen_random_uuid(), 'Скачать карты', 'Скачать офлайн-карты городов', -3, 'low'),
                                                                                (gen_random_uuid(), 'Чек-лист вещей', 'Составить список необходимых вещей', -7, 'medium'),
                                                                                (gen_random_uuid(), 'Обмен валюты', 'Обменять валюту', -5, 'low');