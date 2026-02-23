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

func requireEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		log.Fatalf("FATAL: обязательная переменная окружения %s не задана", key)
	}
	return value
}

func main() {
	cfg := db.Config{
		URL:       requireEnv("SURREAL_URL"),
		User:      requireEnv("SURREAL_USER"),
		Pass:      requireEnv("SURREAL_PASS"),
		Namespace: requireEnv("SURREAL_NS"),
		Database:  requireEnv("SURREAL_DB"),
	}
	appPort := requireEnv("APP_PORT")

	ctx := context.Background()

	surrealDB, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к SurrealDB: %v", err)
	}

	if err := db.RunMigrations(ctx, surrealDB); err != nil {
		log.Fatalf("Ошибка миграции схемы: %v", err)
	}

	// Инициализация репозиториев
	uniRepo := repository.NewUniversityRepository(surrealDB)
	groupRepo := repository.NewSpecialtyGroupRepository(surrealDB)
	specRepo := repository.NewSpecialtyRepository(surrealDB)
	subjectRepo := repository.NewSubjectRepository(surrealDB)

	log.Println("[app] repositories initialized")

	app := fiber.New(fiber.Config{
		AppName:      "Universities KZ v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "online", "db": "connected"})
	})

	v1 := app.Group("/api/v1")

	// ── Universities ────────────────────────────────────────
	v1.Get("/universities", func(c *fiber.Ctx) error {
		unis, err := uniRepo.GetAll(c.Context(), defaultUniversityFilters())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(unis)
	})

	v1.Get("/universities/:id", func(c *fiber.Ctx) error {
		id, err := parseRecordID("university", c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		detail, err := uniRepo.GetWithSpecialties(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(detail)
	})

	// ── Specialty Groups ────────────────────────────────────
	v1.Get("/groups", func(c *fiber.Ctx) error {
		groups, err := groupRepo.GetAll(c.Context(), models.SpecialtyGroupFilters{Limit: 100})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(groups)
	})

	v1.Get("/groups/:id", func(c *fiber.Ctx) error {
		id, err := parseRecordID("specialty_group", c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		group, subjects, err := groupRepo.GetWithSubjects(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"group": group, "subjects": subjects})
	})

	// ── Specialties ─────────────────────────────────────────
	v1.Get("/specialties", func(c *fiber.Ctx) error {
		specs, err := specRepo.GetAll(c.Context(), defaultSpecialtyFilters())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(specs)
	})

	// ── Subjects ────────────────────────────────────────────
	v1.Get("/subjects", func(c *fiber.Ctx) error {
		subjects, err := subjectRepo.GetAll(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(subjects)
	})

	// ── Graceful Shutdown ───────────────────────────────────
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

func defaultUniversityFilters() models.UniversityFilters {
	return models.UniversityFilters{Limit: 50}
}

func defaultSpecialtyFilters() models.SpecialtyFilters {
	return models.SpecialtyFilters{Limit: 50}
}

func parseRecordID(table, id string) (surrealmodels.RecordID, error) {
	if id == "" {
		return surrealmodels.RecordID{}, fmt.Errorf("empty record ID")
	}
	return surrealmodels.NewRecordID(table, id), nil
}
