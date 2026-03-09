package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

<<<<<<< HEAD
<<<<<<< HEAD
	"github.com/gofiber/fiber/v2/middleware/session"
=======
	"runtime"
>>>>>>> d258f37 (Add SurrealDB connection pool and use it)

=======
>>>>>>> 12f28c3 (Add Swagger docs and API handlers)
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/swagger"

	"github.com/Map130/universities/internal/admin"
	"github.com/Map130/universities/internal/auth"
	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/handlers"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/storage"

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

// @host      localhost:3000
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

	// ── MinIO config ────────────────────────────────────────
	minioCfg := storage.Config{
		Endpoint:       requireEnv("MINIO_ENDPOINT"), // например "localhost:9000"
		AccessKey:      requireEnv("MINIO_ROOT_USER"),
		SecretKey:      requireEnv("MINIO_ROOT_PASSWORD"),
		UseSSL:         os.Getenv("MINIO_USE_SSL") == "true",
		PublicEndpoint: requireEnv("MINIO_PUBLIC_URL"), // например "http://localhost:9000"
	}

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

	// ── Инициализация MinIO storage ─────────────────────────
	store, err := storage.NewMinioStorage(ctx, minioCfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к MinIO: %v", err)
	}

<<<<<< HEAD
	// Инициализация репозиториев
	uniRepo := repository.NewUniversityRepository(surrealDB)
	groupRepo := repository.NewSpecialtyGroupRepository(surrealDB)
	specRepo := repository.NewSpecialtyRepository(surrealDB)
	subjectRepo := repository.NewSubjectRepository(surrealDB)
	adminRepo := repository.NewAdminRepository(surrealDB)
=======
	// Инициализация репозиториев (используют пул подключений)
	uniRepo := repository.NewUniversityRepository(pool)
	groupRepo := repository.NewSpecialtyGroupRepository(pool)
	specRepo := repository.NewSpecialtyRepository(pool)
	subjectRepo := repository.NewSubjectRepository(pool)
>>>>>>> d258f37 (Add SurrealDB connection pool and use it)

	log.Println("[app] repositories initialized")

<<<<<<< HEAD
	// ── Google OAuth 2.0 ────────────────────────────────────
	// При сборке с тегом noauth (go build -tags noauth) аутентификация
	// полностью отключена: OAuth не инициализируется, сессия создаётся
	// с фиктивным секретом, auth-роуты не регистрируются.
	var sessionStore *session.Store

	if auth.IsNoAuth() {
		log.Println("[app] ⚠️  noauth build: skipping Google OAuth, using dummy session store")
		sessionStore = auth.NewSessionStore("noauth-dev-secret")
	} else {
		authCfg, err := auth.LoadConfigFromEnv()
		if err != nil {
			log.Fatalf("Ошибка загрузки OAuth-конфигурации: %v", err)
		}
		auth.InitGothProviders(authCfg)
		sessionStore = auth.NewSessionStore(authCfg.SessionSecret)
		log.Println("[app] Google OAuth initialized")
	}
=======
	// ── Handler (все хендлеры в одном месте) ─────────────────
	h := handlers.NewHandler(uniRepo, groupRepo, specRepo, subjectRepo, store)
>>>>>>> 12f28c3 (Add Swagger docs and API handlers)

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

	// ── Static files (CSS, JS, images) ──────────────────────
	app.Static("/static", "./static", fiber.Static{
		Compress:      false, // сжатие через middleware выше, без stale .fiber.gz
		CacheDuration: 0,     // В dev без кеша; в production выставить 24h+
	})

	// ── Swagger UI ──────────────────────────────────────────
	app.Get("/swagger/*", swagger.HandlerDefault)

<<<<<<< HEAD
	// ── Auth routes (публичные) ─────────────────────────────
	// В noauth-билде OAuth-роуты не регистрируются (они не нужны).
	if !auth.IsNoAuth() {
		authHandlers := auth.NewHandlers(sessionStore, adminRepo)
		authHandlers.RegisterRoutes(app)
	}

	// ── Admin Panel (HTML, HTMX, Tailwind) ──────────────────
	// Setup регистрирует все /admin/* маршруты с AuthRequired middleware.
	// Renderer использует html/template с layout + фрагментами для HTMX.
	adminRenderer := admin.Setup(app, admin.Config{
		ViewsDir: "./views",
		DevMode:  os.Getenv("APP_ENV") != "production", // hot reload шаблонов в dev
	}, sessionStore, uniRepo, specRepo, groupRepo, subjectRepo, store)

	// В production режиме прогреваем кэш шаблонов при старте.
	if os.Getenv("APP_ENV") == "production" {
		if err := adminRenderer.WalkTemplates(); err != nil {
			log.Printf("[app] warning: template pre-cache error: %v", err)
		}
	}

	log.Println("[app] admin panel initialized at /admin")

=======
	// ── Health ──────────────────────────────────────────────
	app.Get("/health", h.HealthCheck)

	// ── API v1 ──────────────────────────────────────────────
>>>>>>> 12f28c3 (Add Swagger docs and API handlers)
	v1 := app.Group("/api/v1")

	// Universities
	v1.Get("/universities", h.GetUniversities)
	v1.Get("/universities/:id", h.GetUniversityByID)
	v1.Post("/universities/:id/logo", h.UploadLogo)

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
