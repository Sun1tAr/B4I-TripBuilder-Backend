package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

type SchemaCreator struct {
	log *logger.Logger
}

func NewSchemaCreator(log *logger.Logger) *SchemaCreator {
	return &SchemaCreator{
		log: log,
	}
}

// EnsureSchemaExists создаёт схему если её нет, с защитой от конкурентного доступа
func (sc *SchemaCreator) EnsureSchemaExists(db *sql.DB, schema string) error {
	maxRetries := 5
	baseDelay := 100 * time.Millisecond
	maxDelay := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := sc.tryCreateSchema(db, schema)
		if err == nil {
			return nil
		}

		if isSerializationError(err) {
			delay := baseDelay * (1 << attempt)
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.5)
			delay = delay - jitter/2 + jitter

			if delay > maxDelay {
				delay = maxDelay
			}

			sc.log.Warn("Конфликт при создании схемы, повтор через %v (попытка %d/%d)",
				delay, attempt+1, maxRetries)

			time.Sleep(delay)
			continue
		}

		return fmt.Errorf("failed to create schema: %w", err)
	}

	return fmt.Errorf("failed to create schema after %d attempts", maxRetries)
}

func (sc *SchemaCreator) tryCreateSchema(db *sql.DB, schema string) error {
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	err = tx.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)",
		schema,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check schema existence: %w", err)
	}

	if !exists {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
			return fmt.Errorf("failed to execute CREATE SCHEMA: %w", err)
		}
		sc.log.Info("Схема создана", "schema", schema)
	} else {
		sc.log.Debug("Схема уже существует", "schema", schema)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func isSerializationError(err error) bool {
	if pqErr, ok := err.(interface{ Code() string }); ok {
		code := pqErr.Code()
		return code == "40001" || code == "40P01"
	}
	return false
}