package migrate

import (
	"database/sql"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/postgres"
	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Config struct {
	Schema     string
	MigrationsPath string
	Enabled    bool
}

// Run выполняет миграции для сервиса
func Run(db *sql.DB, cfg Config, log *logger.Logger) error {
	if !cfg.Enabled {
		log.Info("Миграции отключены", "schema", cfg.Schema)
		return nil
	}

	log.Info("Запуск миграций", "schema", cfg.Schema)

	// Создаём схему через shared утилиту
	schemaCreator := postgres.NewSchemaCreator(log)
	if err := schemaCreator.EnsureSchemaExists(db, cfg.Schema); err != nil {
		return fmt.Errorf("failed to ensure schema exists: %w", err)
	}

	// Устанавливаем search_path
	if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s", cfg.Schema)); err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}

	// Создаём драйвер с указанием схемы
	driver, err := migratepg.WithInstance(db, &migratepg.Config{
		SchemaName: cfg.Schema,
	})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Путь к папке с миграциями
	m, err := migrate.NewWithDatabaseInstance(
		cfg.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Применяем миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		log.Info("Миграции не найдены", "schema", cfg.Schema)
	} else {
		log.Info("Миграции успешно применены",
			"schema", cfg.Schema,
			"version", version,
			"dirty", dirty)
	}

	return nil
}