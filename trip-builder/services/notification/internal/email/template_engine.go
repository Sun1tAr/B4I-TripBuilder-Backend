package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification/internal/domain/repository"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type TemplateEngine struct {
	templateRepo repository.TemplateRepository
	log          *logger.Logger
}

func NewTemplateEngine(templateRepo repository.TemplateRepository, log *logger.Logger) *TemplateEngine {
	log.Debug("TemplateEngine: инициализация")
	return &TemplateEngine{
		templateRepo: templateRepo,
		log:          log,
	}
}

// RenderData содержит данные для рендеринга шаблона (ЭКСПОРТИРУЕМЫЙ ТИП)
type RenderData struct {
	FirstName        string
	LastName         string
	Email            string
	EventTitle       string
	EventStartDate   string
	EventEndDate     string
	RouteTitle       string
	InviterName      string
	InviteLink       string
	TaskTitle        string
	TaskDescription  string
	TaskPriority     string
	TaskDueDate      string
	TaskLink         string
	MessageText      string
	SenderName       string
	ChatLink         string
	DaysLeft         int
	EventLink        string
	VerificationLink string
}

// Render рендерит шаблон с данными
func (e *TemplateEngine) Render(ctx context.Context, templateType, language string, data *RenderData) (subject, body string, err error) {
	e.log.Debug("TemplateEngine.Render: начало",
		"type", templateType,
		"language", language,
	)

	// Получаем шаблон из БД
	tpl, err := e.templateRepo.GetByType(ctx, templateType, language)
	if err != nil {
		e.log.Error("TemplateEngine.Render: ошибка получения шаблона",
			"error", err,
			"type", templateType,
		)
		return "", "", err
	}

	if tpl == nil {
		e.log.Warn("TemplateEngine.Render: шаблон не найден, используем fallback",
			"type", templateType,
		)
		return e.renderFallback(templateType, data)
	}

	// Создаем карту данных для шаблона
	templateData := e.buildTemplateData(data)

	// Парсим и выполняем шаблон
	tmpl, err := template.New("email").Parse(tpl.BodyTemplate)
	if err != nil {
		e.log.Error("TemplateEngine.Render: ошибка парсинга шаблона",
			"error", err,
			"type", templateType,
		)
		return "", "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		e.log.Error("TemplateEngine.Render: ошибка выполнения шаблона",
			"error", err,
			"type", templateType,
		)
		return "", "", err
	}

	// Рендерим subject
	subjectTmpl, err := template.New("subject").Parse(tpl.Subject)
	if err != nil {
		e.log.Warn("TemplateEngine.Render: ошибка парсинга subject, используем как есть",
			"error", err,
		)
		subject = tpl.Subject
	} else {
		var subjectBuf bytes.Buffer
		if err := subjectTmpl.Execute(&subjectBuf, templateData); err != nil {
			subject = tpl.Subject
		} else {
			subject = subjectBuf.String()
		}
	}

	e.log.Debug("TemplateEngine.Render: шаблон успешно отрендерен",
		"type", templateType,
		"subject", subject,
		"body_length", buf.Len(),
	)

	return subject, buf.String(), nil
}

// buildTemplateData создает карту данных для шаблона
func (e *TemplateEngine) buildTemplateData(data *RenderData) map[string]interface{} {
	result := make(map[string]interface{})

	if data == nil {
		return result
	}

	if data.FirstName != "" {
		result["FirstName"] = data.FirstName
	}
	if data.LastName != "" {
		result["LastName"] = data.LastName
	}
	if data.Email != "" {
		result["Email"] = data.Email
	}
	if data.VerificationLink != "" {
		result["VerificationLink"] = data.VerificationLink
		e.log.Debug("buildTemplateData: VerificationLink добавлен в result",
			"value", data.VerificationLink,
		)
	} else {
		e.log.Warn("buildTemplateData: VerificationLink пустой!")
	}
	if data.EventTitle != "" {
		result["EventTitle"] = data.EventTitle
	}
	if data.EventStartDate != "" {
		result["StartDate"] = data.EventStartDate
		result["EventStartDate"] = data.EventStartDate
	}
	if data.EventEndDate != "" {
		result["EventEndDate"] = data.EventEndDate
	}
	if data.RouteTitle != "" {
		result["RouteTitle"] = data.RouteTitle
	}
	if data.InviterName != "" {
		result["InviterName"] = data.InviterName
	}
	if data.InviteLink != "" {
		result["InviteLink"] = data.InviteLink
	}
	if data.TaskTitle != "" {
		result["TaskTitle"] = data.TaskTitle
	}
	if data.TaskDescription != "" {
		result["TaskDescription"] = data.TaskDescription
	}
	if data.TaskPriority != "" {
		result["Priority"] = data.TaskPriority
	}
	if data.TaskDueDate != "" {
		result["DueDate"] = data.TaskDueDate
	}
	if data.TaskLink != "" {
		result["TaskLink"] = data.TaskLink
	}
	if data.MessageText != "" {
		result["MessageText"] = data.MessageText
	}
	if data.SenderName != "" {
		result["SenderName"] = data.SenderName
	}
	if data.ChatLink != "" {
		result["ChatLink"] = data.ChatLink
	}
	if data.DaysLeft > 0 {
		result["DaysLeft"] = data.DaysLeft
	}
	if data.EventLink != "" {
		result["EventLink"] = data.EventLink
	}

	if data.FirstName != "" || data.LastName != "" {
		fullName := strings.TrimSpace(data.FirstName + " " + data.LastName)
		result["FullName"] = fullName
	}

	return result
}

// renderFallback рендерит fallback-сообщение
func (e *TemplateEngine) renderFallback(templateType string, data *RenderData) (subject, body string, err error) {
	e.log.Warn("TemplateEngine.renderFallback: использование fallback",
		"type", templateType,
	)

	greeting := "Здравствуйте"
	if data != nil && data.FirstName != "" {
		greeting = fmt.Sprintf("Здравствуйте, %s", data.FirstName)
	}

	switch templateType {
	case "user.welcome":
		subject = "Добро пожаловать в Trip Builder!"
		body = fmt.Sprintf(`<h1>%s!</h1><p>Спасибо за регистрацию в Trip Builder.</p>`, greeting)
	case "event.invite":
		subject = "Вас пригласили в поездку"
		eventTitle := "поездку"
		if data != nil && data.EventTitle != "" {
			eventTitle = fmt.Sprintf(`"%s"`, data.EventTitle)
		}
		body = fmt.Sprintf(`<h1>%s!</h1><p>Вас пригласили присоединиться к %s.</p>`, greeting, eventTitle)
	case "task.assigned":
		subject = "Вам назначена задача"
		taskTitle := "новая задача"
		if data != nil && data.TaskTitle != "" {
			taskTitle = fmt.Sprintf(`"%s"`, data.TaskTitle)
		}
		body = fmt.Sprintf(`<h1>%s!</h1><p>Вам назначена %s.</p>`, greeting, taskTitle)
	case "event.reminder":
		subject = "Напоминание о поездке"
		eventTitle := "поездка"
		if data != nil && data.EventTitle != "" {
			eventTitle = fmt.Sprintf(`"%s"`, data.EventTitle)
		}
		body = fmt.Sprintf(`<h1>%s!</h1><p>Напоминаем о предстоящей поездке %s.</p>`, greeting, eventTitle)
	case "chat.message":
		subject = "Новое сообщение в чате"
		body = fmt.Sprintf(`<h1>%s!</h1><p>У вас новое сообщение в чате поездки.</p>`, greeting)
	default:
		subject = "Уведомление от Trip Builder"
		body = fmt.Sprintf(`<h1>%s!</h1><p>У вас новое уведомление в Trip Builder.</p>`, greeting)
	}

	body += "<p>С уважением,<br>Команда Trip Builder</p>"

	return subject, body, nil
}
