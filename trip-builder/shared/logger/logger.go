package logger

import (
	"context"
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func New(level slog.Level) *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return &Logger{slog.New(handler)}
}

func (l *Logger) WithRequestID(ctx context.Context) *Logger {
	if reqID, ok := ctx.Value("request_id").(string); ok {
		return &Logger{l.With(slog.String("request_id", reqID))}
	}
	return l
}