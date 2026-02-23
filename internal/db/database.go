// Package db реализует подключение к SurrealDB и миграцию схемы.
package db

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/surrealdb/surrealdb.go"
)

//go:embed schema.surql
var schemaFS embed.FS

// Config хранит параметры подключения к SurrealDB.
type Config struct {
	// URL — адрес SurrealDB (например, "ws://localhost:8000/rpc").
	URL string

	// User — имя пользователя для аутентификации (root-уровень).
	User string

	// Pass — пароль пользователя.
	Pass string

	// Namespace — пространство имён SurrealDB.
	Namespace string

	// Database — имя базы данных внутри пространства имён.
	Database string
}

// Connect устанавливает соединение с SurrealDB, выполняет SignIn и Use.
// Возвращает готовый к работе *surrealdb.DB.
//
// Вызывающая сторона отвечает за закрытие соединения через db.Close(ctx).
func Connect(ctx context.Context, cfg Config) (*surrealdb.DB, error) {
	// 1. Подключение к серверу.
	db, err := surrealdb.FromEndpointURLString(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: connect to %s: %w", cfg.URL, err)
	}

	// 2. Аутентификация (root-уровень).
	if _, err = db.SignIn(ctx, map[string]any{
		"user": cfg.User,
		"pass": cfg.Pass,
	}); err != nil {
		return nil, fmt.Errorf("db: sign in: %w", err)
	}

	// 3. Выбор namespace и database.
	if err = db.Use(ctx, cfg.Namespace, cfg.Database); err != nil {
		return nil, fmt.Errorf("db: use %s/%s: %w", cfg.Namespace, cfg.Database, err)
	}

	log.Printf("[db] connected to %s  ns=%s  db=%s", cfg.URL, cfg.Namespace, cfg.Database)
	return db, nil
}

// RunMigrations читает встроенный файл schema.surql и выполняет его
// через surrealdb.Query. Это создаёт (или обновляет) таблицы, индексы,
// события и связи, описанные в схеме.
//
// Функция идемпотентна: повторный запуск безопасен, т. к. SurrealDB
// перезаписывает DEFINE-объявления.
func RunMigrations(ctx context.Context, db *surrealdb.DB) error {
	schema, err := schemaFS.ReadFile("schema.surql")
	if err != nil {
		return fmt.Errorf("db: read embedded schema: %w", err)
	}

	if _, err = surrealdb.Query[any](ctx, db, string(schema), nil); err != nil {
		return fmt.Errorf("db: run migrations: %w", err)
	}

	log.Println("[db] schema migrations applied successfully")
	return nil
}
