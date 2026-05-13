package email

import (
	"context"
	"fmt"
	"net/smtp"
	"time"
	"encoding/base64"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type Sender struct {
	host     string
	port     string
	username string
	password string
	from     string
	log      *logger.Logger
}

func NewSender(host, port, username, password, from string, log *logger.Logger) *Sender {
	log.Debug("Email.Sender: инициализация",
		"host", host,
		"port", port,
		"from", from,
	)

	return &Sender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		log:      log,
	}
}

// Send отправляет письмо
func (s *Sender) Send(ctx context.Context, to, subject, htmlBody string) error {
	startTime := time.Now()
	s.log.Debug("Email.Sender: начало отправки",
		"to", maskEmail(to),
		"subject", subject,
	)

	// Формируем заголовки письма
	headers := make(map[string]string)
	headers["From"] = s.from
	headers["To"] = to
	headers["Subject"] = encodeRFC2047(subject)
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""

	// Собираем полное сообщение
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	// Аутентификация и отправка
	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
		s.log.Debug("Email.Sender: используется аутентификация")
	} else {
		s.log.Debug("Email.Sender: отправка без аутентификации")
	}

	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	s.log.Debug("Email.Sender: подключение к SMTP",
		"addr", addr,
	)

	err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(message))
	if err != nil {
		s.log.Error("Email.Sender: ошибка отправки",
			"error", err,
			"to", maskEmail(to),
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
		return fmt.Errorf("smtp send failed: %w", err)
	}

	s.log.Info("Email.Sender: письмо отправлено",
		"to", maskEmail(to),
		"subject", subject,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)
	return nil
}

// SendAsync отправляет письмо асинхронно (в горутине)
func (s *Sender) SendAsync(ctx context.Context, to, subject, htmlBody string) {
	go func() {
		// Используем background context, чтобы не прерывать отправку
		bgCtx := context.Background()
		if err := s.Send(bgCtx, to, subject, htmlBody); err != nil {
			s.log.Error("Email.Sender: асинхронная отправка не удалась",
				"error", err,
				"to", maskEmail(to),
			)
		}
	}()
}

// maskEmail маскирует email для логирования (user@example.com -> u***@example.com)
func maskEmail(email string) string {
	if len(email) == 0 {
		return email
	}
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			atIndex = i
			break
		}
	}
	if atIndex <= 1 {
		return email
	}
	return email[:1] + "***" + email[atIndex:]
}

func encodeRFC2047(s string) string {
	// Если строка ASCII — не кодируем
	if isASCII(s) {
		return s
	}
	return fmt.Sprintf("=?utf-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(s)))
}

func isASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}
