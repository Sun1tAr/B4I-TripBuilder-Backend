INSERT INTO email_templates (type, subject, body_template, language) VALUES
    ('password.reset', 'Сброс пароля в Trip Builder',
     '<!DOCTYPE html>
    <html>
    <head><meta charset="UTF-8"></head>
    <body>
        <h1>Здравствуйте, {{.FirstName}}!</h1>
        <p>Вы запросили сброс пароля в Trip Builder.</p>
        <p>Чтобы установить новый пароль, перейдите по ссылке:</p>
        <p style="margin: 30px 0;">
            <a href="{{.ResetLink}}"
               style="background-color: #4CAF50; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; font-weight: bold;">
                Сбросить пароль
            </a>
        </p>
        <p>Или скопируйте эту ссылку в браузер:</p>
        <p style="background-color: #f5f5f5; padding: 10px; border-radius: 5px; word-break: break-all;">
            {{.ResetLink}}
        </p>
        <p>Ссылка действительна в течение 1 часа.</p>
        <p>Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо.</p>
        <hr style="margin: 30px 0;">
        <p>С уважением,<br>Команда Trip Builder</p>
    </body>
    </html>',
     'ru')
ON CONFLICT (type, language) DO NOTHING;