package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"path"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/admin"
	"github.com/Map130/universities/internal/auth"
	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/storage"
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

	// ── MinIO config ────────────────────────────────────────
	minioCfg := storage.Config{
		Endpoint:       requireEnv("MINIO_ENDPOINT"), // например "localhost:9000"
		AccessKey:      requireEnv("MINIO_ROOT_USER"),
		SecretKey:      requireEnv("MINIO_ROOT_PASSWORD"),
		UseSSL:         os.Getenv("MINIO_USE_SSL") == "true",
		PublicEndpoint: requireEnv("MINIO_PUBLIC_URL"), // например "http://localhost:9000"
	}

	ctx := context.Background()

	surrealDB, err := db.Connect(ctx, cfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к SurrealDB: %v", err)
	}

	if err := db.RunMigrations(ctx, surrealDB); err != nil {
		log.Fatalf("Ошибка миграции схемы: %v", err)
	}

	// ── Инициализация MinIO storage ─────────────────────────
	store, err := storage.NewMinioStorage(ctx, minioCfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к MinIO: %v", err)
	}

	// Инициализация репозиториев
	uniRepo := repository.NewUniversityRepository(surrealDB)
	groupRepo := repository.NewSpecialtyGroupRepository(surrealDB)
	specRepo := repository.NewSpecialtyRepository(surrealDB)
	subjectRepo := repository.NewSubjectRepository(surrealDB)
	adminRepo := repository.NewAdminRepository(surrealDB)

	log.Println("[app] repositories initialized")

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

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "online", "db": "connected"})
	})

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
	}, sessionStore, uniRepo, store)

	// В production режиме прогреваем кэш шаблонов при старте.
	if os.Getenv("APP_ENV") == "production" {
		if err := adminRenderer.WalkTemplates(); err != nil {
			log.Printf("[app] warning: template pre-cache error: %v", err)
		}
	}

	log.Println("[app] admin panel initialized at /admin")

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

	// ── Upload logo ─────────────────────────────────────────
	v1.Post("/universities/:id/logo", func(c *fiber.Ctx) error {
		// 1. Парсим ID вуза.
		id, err := parseRecordID("university", c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// 2. Проверяем, что вуз существует.
		uni, err := uniRepo.GetByID(c.Context(), id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "university not found"})
		}

		// 3. Извлекаем файл из multipart-формы.
		file, err := c.FormFile("logo")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing 'logo' file in form"})
		}

		// 4. Загружаем изображение в MinIO.
		logoURL, err := store.UploadImage(c.Context(), file)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// 5. Удаляем старый логотип, если был.
		if uni.LogoURL != nil && *uni.LogoURL != "" {
			oldFileName := path.Base(*uni.LogoURL)
			// Ошибку удаления логируем, но не блокируем запрос.
			if delErr := store.DeleteFile(c.Context(), storage.BucketLogos, oldFileName); delErr != nil {
				log.Printf("[upload] warning: failed to delete old logo %s: %v", oldFileName, delErr)
			}
		}

		// 6. Обновляем logo_url в базе данных.
		uni.LogoURL = &logoURL
		updated, err := uniRepo.Update(c.Context(), id, *uni)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("logo uploaded but DB update failed: %v", err)})
		}

		return c.JSON(fiber.Map{
			"message":    "logo uploaded successfully",
			"logo_url":   logoURL,
			"university": updated,
		})
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
