-- Создаём схему, если не существует
CREATE SCHEMA IF NOT EXISTS tpb_notification;

-- Устанавливаем search_path для текущей сессии
SET search_path TO tpb_notification;

-- Таблица уведомлений
CREATE TABLE IF NOT EXISTS notifications (
                                             id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                             user_id UUID NOT NULL,
                                             type VARCHAR(50) NOT NULL,
                                             title VARCHAR(255),
                                             body TEXT,
                                             data JSONB,
                                             is_read BOOLEAN DEFAULT false,
                                             created_at TIMESTAMP DEFAULT now()
);

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type);

-- Таблица устройств пользователей (только для хранения токенов, push пока не реализуем)
CREATE TABLE IF NOT EXISTS user_devices (
                                            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                            user_id UUID NOT NULL,
                                            token VARCHAR(255) NOT NULL,
                                            platform VARCHAR(20) NOT NULL,
                                            last_used TIMESTAMP DEFAULT now()
);

-- Индексы для устройств
CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_user_devices_token ON user_devices(token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_devices_user_token ON user_devices(user_id, token);

-- Таблица шаблонов писем
CREATE TABLE IF NOT EXISTS email_templates (
                                               id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                               type VARCHAR(50) NOT NULL,
                                               subject VARCHAR(255),
                                               body_template TEXT,
                                               language VARCHAR(10) DEFAULT 'ru',
                                               created_at TIMESTAMP DEFAULT now(),
                                               updated_at TIMESTAMP
);

-- Уникальный индекс на тип и язык
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_templates_type_language ON email_templates(type, language);
CREATE INDEX IF NOT EXISTS idx_email_templates_type ON email_templates(type);

-- Вставляем базовые шаблоны
INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('user.welcome', 'Добро пожаловать в Trip Builder!',
     '<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body><h1>Привет, {{.FirstName}}!</h1><p>Спасибо за регистрацию в Trip Builder. Теперь вы можете создавать маршруты и планировать поездки.</p><p>С уважением,<br>Команда Trip Builder</p></body></html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;

INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('event.invite', 'Вас пригласили в поездку "{{.EventTitle}}"',
     '<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body><h1>Привет, {{.FirstName}}!</h1><p><strong>{{.InviterName}}</strong> приглашает вас присоединиться к поездке <strong>"{{.EventTitle}}"</strong>.</p><p>📅 Дата начала: {{.StartDate}}</p><p>📍 Маршрут: {{.RouteTitle}}</p><p><a href="{{.InviteLink}}" style="background-color: #4CAF50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Принять приглашение</a></p><p>С уважением,<br>Команда Trip Builder</p></body></html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;

INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('task.assigned', 'Вам назначена задача "{{.TaskTitle}}"',
     '<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body><h1>Привет, {{.FirstName}}!</h1><p>В поездке <strong>"{{.EventTitle}}"</strong> вам назначена задача:</p><h2>{{.TaskTitle}}</h2><p>{{.TaskDescription}}</p><p>⏰ Срок выполнения: {{.DueDate}}</p><p>📊 Приоритет: {{.Priority}}</p><p><a href="{{.TaskLink}}">Перейти к задаче</a></p><p>С уважением,<br>Команда Trip Builder</p></body></html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;

INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('event.reminder', 'Напоминание о поездке "{{.EventTitle}}"',
     '<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body><h1>Привет, {{.FirstName}}!</h1><p>Напоминаем, что ваша поездка <strong>"{{.EventTitle}}"</strong> начинается {{.StartDate}}.</p><p>Осталось дней: {{.DaysLeft}}</p><p><a href="{{.EventLink}}">Перейти к поездке</a></p><p>С уважением,<br>Команда Trip Builder</p></body></html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;

INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('chat.message', 'Новое сообщение в чате поездки "{{.EventTitle}}"',
     '<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body><h1>Привет, {{.FirstName}}!</h1><p>У вас новое сообщение в чате поездки <strong>"{{.EventTitle}}"</strong>:</p><div style="background-color: #f5f5f5; padding: 15px; border-radius: 5px; margin: 10px 0;"><strong>{{.SenderName}}:</strong><br>{{.MessageText}}</div><p><a href="{{.ChatLink}}">Перейти в чат</a></p><p>С уважением,<br>Команда Trip Builder</p></body></html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;