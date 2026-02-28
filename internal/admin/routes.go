// Package admin реализует маршрутизацию и хендлеры админ-панели.
package admin

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"

	"github.com/Map130/universities/internal/auth"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/storage"
)

// Config содержит параметры для инициализации админ-панели.
type Config struct {
	// ViewsDir — путь к директории с HTML-шаблонами (например, "./views").
	ViewsDir string

	// DevMode — режим разработки (шаблоны перечитываются при каждом запросе).
	DevMode bool
}

// Setup инициализирует админ-панель: создаёт renderer, handlers и
// регистрирует все маршруты в Fiber app.
//
// Все маршруты /admin/* защищены middleware auth.AuthRequired, который
// проверяет наличие активной сессии администратора.
//
// Параметры:
//   - app:          Fiber application instance.
//   - cfg:          конфигурация админки (пути к views, dev mode).
//   - sessionStore: Fiber session store (общий для auth и admin).
//   - uniRepo:      репозиторий вузов (SurrealDB).
//   - uploader:     файловое хранилище (MinIO/S3).
//
// Возвращает *Renderer для возможного pre-warming кэша шаблонов.
func Setup(
	app *fiber.App,
	cfg Config,
	sessionStore *session.Store,
	uniRepo repository.UniversityRepository,
	uploader storage.Uploader,
) *Renderer {
	// ── Template Renderer ───────────────────────────────────
	renderer := NewRenderer(cfg.ViewsDir, cfg.DevMode)

	// ── Admin Handlers ──────────────────────────────────────
	uniHandlers := NewHandlers(uniRepo, sessionStore, uploader, renderer)

	// ── Admin Route Group (защищён AuthRequired) ────────────
	admin := app.Group("/admin", auth.AuthRequired(sessionStore))

	// Dashboard (заглушка — рендерит список вузов пока нет дашборда).
	admin.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/admin/universities", fiber.StatusFound)
	})

	// ── Universities CRUD ───────────────────────────────────
	// Все маршруты регистрируются внутри RegisterRoutes:
	//   GET    /admin/universities            → Index  (список)
	//   GET    /admin/universities/new        → New    (форма создания)
	//   POST   /admin/universities            → Create (создание)
	//   GET    /admin/universities/:id/edit   → Edit   (форма редактирования)
	//   PUT    /admin/universities/:id        → Update (обновление)
	//   POST   /admin/universities/:id        → UpdatePost (graceful degradation)
	//   DELETE /admin/universities/:id        → Delete (удаление)
	uniGroup := admin.Group("/universities")
	uniHandlers.RegisterRoutes(uniGroup)

	return renderer
}
