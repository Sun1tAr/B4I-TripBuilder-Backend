package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

type Level = slog.Level

const (
	LevelTrace Level = slog.LevelDebug - 4
	LevelDebug Level = slog.LevelDebug
	LevelInfo  Level = slog.LevelInfo
	LevelWarn  Level = slog.LevelWarn
	LevelError Level = slog.LevelError
)

type Logger struct {
	*slog.Logger
	serviceName string
	pid         int
}

type Options struct {
	Level       Level
	ServiceName string
	Format      string // "console" (цветной) или "json"
}

// New создаёт логгер с заданными опциями
func New(opts Options) *Logger {
	if opts.ServiceName == "" {
		opts.ServiceName = "unknown"
	}

	pid := os.Getpid()
	handler := newHandler(opts)

	return &Logger{
		Logger:      slog.New(handler),
		serviceName: opts.ServiceName,
		pid:         pid,
	}
}

func newHandler(opts Options) slog.Handler {
	level := opts.Level
	if level == 0 {
		level = LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Убираем стандартный timestamp, будем добавлять свой
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			// Убираем стандартный level
			if a.Key == slog.LevelKey {
				return slog.Attr{}
			}
			// Убираем стандартный message, будем добавлять своим порядком
			if a.Key == slog.MessageKey {
				return slog.Attr{}
			}
			return a
		},
	}

	if opts.Format == "json" {
		return &jsonHandler{
			Handler: slog.NewJSONHandler(os.Stdout, handlerOpts),
			opts:    opts,
		}
	}
	return &consoleHandler{
		Handler: slog.NewTextHandler(os.Stdout, handlerOpts),
		opts:    opts,
	}
}

// consoleHandler – цветной вывод для терминала
type consoleHandler struct {
	slog.Handler
	opts Options
}

func (h *consoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Уровень + цвет
	var levelStr string
	switch r.Level {
	case LevelTrace:
		levelStr = color.HiBlackString("TRACE")
	case LevelDebug:
		levelStr = color.BlueString("DEBUG")
	case LevelInfo:
		levelStr = color.GreenString("INFO ")
	case LevelWarn:
		levelStr = color.YellowString("WARN ")
	case LevelError:
		levelStr = color.RedString("ERROR")
	default:
		levelStr = fmt.Sprintf("%-5s", r.Level.String())
	}

	// Время (ISO8601 с таймзоной)
	timeStr := r.Time.Format("2006-01-02T15:04:05.000-07:00")
	timeStr = color.HiBlackString(timeStr)

	// PID
	pidStr := color.WhiteString("%5d", os.Getpid())

	// Service name
	serviceStr := color.CyanString("[%-15s]", h.opts.ServiceName)

	// Thread name (ID горутины)
	thread := getGoroutineID()
	threadStr := color.HiBlackString("[%s]", thread)

	// Logger name (из source)
	loggerName := "?"
	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			parts := strings.Split(f.File, "/")
			if len(parts) > 3 {
				loggerName = strings.Join(parts[len(parts)-3:], "/")
			} else {
				loggerName = f.File
			}
			loggerName = fmt.Sprintf("%s:%d", loggerName, f.Line)
		}
	}
	loggerNameStr := color.HiBlueString("%-40s", loggerName)

	// Сообщение
	msg := r.Message

	// Собираем дополнительные атрибуты в одну строку [key=value ...]
	attrs := make([]string, 0)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "" {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
		}
		return true
	})
	var attrsStr string
	if len(attrs) > 0 {
		attrsStr = " [" + strings.Join(attrs, " ") + "]"
	}

	// Итоговая строка: время уровень pid [service] [thread] logger : сообщение [attrs]
	fmt.Fprintf(os.Stdout, "%s %s %s %s %s %s : %s%s\n",
		timeStr,
		levelStr,
		pidStr,
		serviceStr,
		threadStr,
		loggerNameStr,
		msg,
		attrsStr,
	)

	return nil
}

// jsonHandler – JSON-вывод для продакшена
type jsonHandler struct {
	slog.Handler
	opts Options
}

func (h *jsonHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := make(map[string]interface{})

	attrs["timestamp"] = r.Time.Format("2006-01-02T15:04:05.000-07:00")
	attrs["level"] = r.Level.String()
	attrs["pid"] = os.Getpid()
	attrs["service"] = h.opts.ServiceName
	attrs["thread"] = getGoroutineID()
	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		attrs["logger"] = fmt.Sprintf("%s:%d", f.File, f.Line)
	} else {
		attrs["logger"] = "?"
	}
	attrs["message"] = r.Message

	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	data, err := json.Marshal(attrs)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

// WithRequestID возвращает логгер с добавленным полем request_id из контекста
func (l *Logger) WithRequestID(ctx context.Context) *Logger {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		return &Logger{
			Logger:      l.With(slog.String("request_id", reqID)),
			serviceName: l.serviceName,
			pid:         l.pid,
		}
	}
	return l
}

// вспомогательная функция для получения ID горутины
func getGoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	return idField
}