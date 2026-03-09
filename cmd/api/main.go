package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/swagger"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/handlers"
	"github.com/Map130/universities/internal/repository"

	_ "github.com/Map130/universities/docs/swagger"
)

func requireEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		log.Fatalf("FATAL: обязательная переменная окружения %s не задана", key)
	}
	return value
}

// @title           Universities KZ API
// @version         1.0
// @description     API для поиска университетов, специальностей и образовательных программ Казахстана.
// @description     Предоставляет данные о вузах, группах ОП, специальностях и предметах ЕНТ.

// @contact.name   Map130 Team
// @contact.url    https://github.com/Map130/universities

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @schemes   http https
func main() {
	dbCfg := db.Config{
		URL:       requireEnv("SURREAL_URL"),
		User:      requireEnv("SURREAL_USER"),
		Pass:      requireEnv("SURREAL_PASS"),
		Namespace: requireEnv("SURREAL_NS"),
		Database:  requireEnv("SURREAL_DB"),
	}
	appPort := requireEnv("APP_PORT")

	ctx := context.Background()

	// ── Пул подключений к SurrealDB ─────────────────────────
	// SurrealDB Go SDK v1.x использует одно WS-соединение на *surrealdb.DB.
	// Под конкурентной нагрузкой одно соединение захлёбывается.
	// Пул из 2×CPU соединений решает проблему.
	poolSize := runtime.NumCPU() * 2
	if poolSize < 4 {
		poolSize = 4
	}
	pool, err := db.NewPool(ctx, db.PoolConfig{
		Config: dbCfg,
		Size:   poolSize,
	})
	if err != nil {
		log.Fatalf("Ошибка создания пула подключений к SurrealDB: %v", err)
	}

	if err := db.RunMigrationsOnPool(ctx, pool); err != nil {
		log.Fatalf("Ошибка миграции схемы: %v", err)
	}

	// Инициализация репозиториев (используют пул подключений)
	uniRepo := repository.NewUniversityRepository(pool)
	groupRepo := repository.NewSpecialtyGroupRepository(pool)
	specRepo := repository.NewSpecialtyRepository(pool)
	subjectRepo := repository.NewSubjectRepository(pool)

	log.Println("[app] repositories initialized")

	// ── Handler (все хендлеры в одном месте) ─────────────────
	h := handlers.NewHandler(uniRepo, groupRepo, specRepo, subjectRepo)

	app := fiber.New(fiber.Config{
		AppName:      "Universities KZ v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// ── Gzip/Deflate сжатие (on-the-fly, без файлового кэша) ─
	// Fiber middleware compress сжимает ответы на лету.
	// В отличие от fiber.Static{Compress: true}, который создаёт
	// .fiber.gz файлы рядом с оригиналами и НЕ обновляет их при
	// пересборке Tailwind → браузер получает устаревший CSS.
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))

	// ── Swagger UI ──────────────────────────────────────────
	app.Get("/swagger/*", swagger.HandlerDefault)

	// ── Health ──────────────────────────────────────────────
	app.Get("/health", h.HealthCheck)

	// ── API v1 ──────────────────────────────────────────────
	v1 := app.Group("/api/v1")

	// Universities
	v1.Get("/universities", h.GetUniversities)
	v1.Get("/universities/:id", h.GetUniversityByID)

	// Specialty Groups
	v1.Get("/groups", h.GetGroups)
	v1.Get("/groups/:id", h.GetGroupByID)

	// Specialties
	v1.Get("/specialties", h.GetSpecialties)

	// Subjects
	v1.Get("/subjects", h.GetSubjects)

	// ── Graceful Shutdown ───────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + appPort); err != nil {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	log.Printf("[app] listening on :%s", appPort)
	log.Printf("[app] Swagger UI available at http://localhost:%s/swagger/index.html", appPort)

	<-quit
	log.Println("[app] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("[app] server shutdown error: %v", err)
	}
	if err := pool.Close(shutdownCtx); err != nil {
		log.Printf("[app] db pool close error: %v", err)
	}

	log.Println("[app] stopped")
}
