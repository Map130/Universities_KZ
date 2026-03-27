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
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/swagger"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/handlers"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/web"
	"github.com/Map130/universities/internal/webhook"

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

type webhookReposImpl struct {
	uni     repository.UniversityRepository
	spec    repository.SpecialtyRepository
	group   repository.SpecialtyGroupRepository
	subject repository.SubjectRepository
}

func (w *webhookReposImpl) Universities() repository.UniversityRepository { return w.uni }
func (w *webhookReposImpl) Specialties() repository.SpecialtyRepository   { return w.spec }
func (w *webhookReposImpl) Groups() repository.SpecialtyGroupRepository   { return w.group }
func (w *webhookReposImpl) Subjects() repository.SubjectRepository        { return w.subject }

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
	calcRepo := repository.NewCalculatorRepository(pool)

	log.Println("[app] repositories initialized")

	// ── Handler (все хендлеры в одном месте) ─────────────────
	h := handlers.NewHandler(uniRepo, groupRepo, specRepo, subjectRepo, calcRepo)

	app := fiber.New(fiber.Config{
		AppName:      "Universities KZ v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// ── Helmet Middleware (Security Headers) ─────────────────
	app.Use(helmet.New())

	// ── Gzip/Deflate сжатие (on-the-fly, без файлового кэша) ─
	// Fiber middleware compress сжимает ответы на лету.
	// В отличие от fiber.Static{Compress: true}, который создаёт
	// .fiber.gz файлы рядом с оригиналами и НЕ обновляет их при
	// пересборке Tailwind → браузер получает устаревший CSS.
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))

	// ── Static Files ─────────────────────────────────────────
	app.Static("/static", "./static")

	// ── Web (SSR Frontend) ──────────────────────────────────
	web.RegisterWebRoutes(app, uniRepo, calcRepo, subjectRepo, groupRepo)

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

	// Calculator
	v1.Get("/calculator", h.GetCalculatorResults)

	// ── Webhooks & Data Sync ────────────────────────────────
	reposImpl := &webhookReposImpl{
		uni:     uniRepo,
		spec:    specRepo,
		group:   groupRepo,
		subject: subjectRepo,
	}

	githubOwner := os.Getenv("GITHUB_OWNER")
	githubRepo := os.Getenv("GITHUB_REPO")
	githubBranch := os.Getenv("GITHUB_BRANCH")

	if githubOwner != "" && githubRepo != "" && githubBranch != "" {
		gitClient := webhook.NewGitClient(
			os.Getenv("GITHUB_TOKEN"),
			githubOwner,
			githubRepo,
			githubBranch,
		)

		// Webhook Endpoints
		app.Post("/webhook/git", webhook.WebhookHandler(reposImpl, os.Getenv("WEBHOOK_SECRET"), gitClient))
		app.Post("/webhook/sync", webhook.FullSyncHandler(reposImpl, gitClient))

		// Startup Sync
		log.Println("[app] starting initial data sync from GitHub...")
		syncCtx, syncCancel := context.WithTimeout(ctx, 5*time.Minute)

		tarStream, err := gitClient.FetchTarball(syncCtx)
		if err != nil {
			log.Printf("[app] Ошибка загрузки данных из Git: %v", err)
		} else {
			if err := webhook.ProcessTarball(syncCtx, tarStream, reposImpl); err != nil {
				log.Printf("[app] Ошибка обработки данных из Git: %v", err)
			} else {
				log.Println("[app] initial data sync completed successfully")
			}
			_ = tarStream.Close()
		}
		syncCancel()
	} else {
		log.Println("[app] skipping GitHub sync and webhooks: missing GITHUB_OWNER, GITHUB_REPO, or GITHUB_BRANCH")
	}

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
