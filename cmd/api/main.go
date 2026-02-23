package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
)

// getEnv возвращает значение переменной окружения или fallback.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// ─────────────────────────────────────────────────────────
	//  1. Конфигурация из переменных окружения
	// ─────────────────────────────────────────────────────────
	cfg := db.Config{
		URL:       getEnv("SURREAL_URL", "ws://localhost:8000/rpc"),
		User:      getEnv("SURREAL_USER", "root"),
		Pass:      getEnv("SURREAL_PASS", "root"),
		Namespace: getEnv("SURREAL_NS", "test"),
		Database:  getEnv("SURREAL_DB", "test"),
	}
	appPort := getEnv("APP_PORT", "8080")

	ctx := context.Background()

	// ─────────────────────────────────────────────────────────
	//  2. Подключение к SurrealDB
	// ─────────────────────────────────────────────────────────
	surrealDB, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к SurrealDB: %v", err)
	}

	// ─────────────────────────────────────────────────────────
	//  3. Миграция схемы (schema.surql → SurrealDB)
	// ─────────────────────────────────────────────────────────
	if err := db.RunMigrations(ctx, surrealDB); err != nil {
		log.Fatalf("Ошибка миграции схемы: %v", err)
	}

	// ─────────────────────────────────────────────────────────
	//  4. Инициализация репозиториев (Dependency Injection)
	// ─────────────────────────────────────────────────────────
	uniRepo := repository.NewUniversityRepository(surrealDB)
	specRepo := repository.NewSpecialtyRepository(surrealDB)
	subjectRepo := repository.NewSubjectRepository(surrealDB)

	// Логируем для подтверждения (в будущем эти переменные
	// будут переданы в HTTP-хэндлеры).
	_ = uniRepo
	_ = specRepo
	_ = subjectRepo

	log.Println("[app] repositories initialized")

	// ─────────────────────────────────────────────────────────
	//  5. Инициализация Fiber
	// ─────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "Universities KZ v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// ── Health-check ────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "online",
			"db":     "connected",
		})
	})

	// ── API v1 (заготовки эндпоинтов) ───────────────────────
	v1 := app.Group("/api/v1")

	// Universities
	v1.Get("/universities", func(c *fiber.Ctx) error {
		// TODO: парсинг query-параметров → UniversityFilters
		unis, err := uniRepo.GetAll(c.Context(), defaultUniversityFilters())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(unis)
	})

	v1.Get("/universities/:id", func(c *fiber.Ctx) error {
		id, err := parseRecordID("university", c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		detail, err := uniRepo.GetWithSpecialties(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(detail)
	})

	// Specialties
	v1.Get("/specialties", func(c *fiber.Ctx) error {
		specs, err := specRepo.GetAll(c.Context(), defaultSpecialtyFilters())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(specs)
	})

	// Subjects
	v1.Get("/subjects", func(c *fiber.Ctx) error {
		subjects, err := subjectRepo.GetAll(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(subjects)
	})

	// ─────────────────────────────────────────────────────────
	//  6. Graceful Shutdown
	// ─────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + appPort); err != nil {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	log.Printf("[app] listening on :%s", appPort)

	<-quit
	log.Println("[app] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("[app] server shutdown error: %v", err)
	}

	if err := surrealDB.Close(shutdownCtx); err != nil {
		log.Printf("[app] db close error: %v", err)
	}

	log.Println("[app] stopped")
}

// ─────────────────────────────────────────────────────────────
//  Вспомогательные функции
// ─────────────────────────────────────────────────────────────

// defaultUniversityFilters возвращает фильтры по умолчанию (первые 50 записей).
func defaultUniversityFilters() models.UniversityFilters {
	return models.UniversityFilters{
		Limit: 50,
	}
}

// defaultSpecialtyFilters возвращает фильтры по умолчанию (первые 50 записей).
func defaultSpecialtyFilters() models.SpecialtyFilters {
	return models.SpecialtyFilters{
		Limit: 50,
	}
}

// parseRecordID преобразует строковый ID из URL в surrealmodels.RecordID.
// Пример: parseRecordID("university", "abc123") → RecordID{Table: "university", ID: "abc123"}
func parseRecordID(table, id string) (surrealmodels.RecordID, error) {
	if id == "" {
		return surrealmodels.RecordID{}, fmt.Errorf("empty record ID")
	}
	return surrealmodels.NewRecordID(table, id), nil
}
