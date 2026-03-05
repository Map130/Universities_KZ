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
// При сборке с тегом `noauth` аутентификация отключена — используется
// NoAuthMiddleware, подставляющий фейковые данные администратора.
//
// Параметры:
//   - app:          Fiber application instance.
//   - cfg:          конфигурация админки (пути к views, dev mode).
//   - sessionStore: Fiber session store (общий для auth и admin).
//   - uniRepo:      репозиторий вузов (SurrealDB).
//   - specRepo:     репозиторий специальностей (SurrealDB).
//   - groupRepo:    репозиторий групп ОП (SurrealDB).
//   - subjectRepo:  репозиторий предметов ЕНТ (SurrealDB).
//   - uploader:     файловое хранилище (MinIO/S3).
//
// Возвращает *Renderer для возможного pre-warming кэша шаблонов.
func Setup(
	app *fiber.App,
	cfg Config,
	sessionStore *session.Store,
	uniRepo repository.UniversityRepository,
	specRepo repository.SpecialtyRepository,
	groupRepo repository.SpecialtyGroupRepository,
	subjectRepo repository.SubjectRepository,
	uploader storage.Uploader,
) *Renderer {
	// ── Template Renderer ───────────────────────────────────
	renderer := NewRenderer(cfg.ViewsDir, cfg.DevMode)

	// ── Admin Handlers ──────────────────────────────────────
	uniHandlers := NewHandlers(uniRepo, sessionStore, uploader, renderer)
	specHandlers := NewSpecialtyHandlers(specRepo, groupRepo, sessionStore, renderer)
	groupHandlers := NewGroupHandlers(groupRepo, subjectRepo, sessionStore, renderer)
	subjectHandlers := NewSubjectHandlers(subjectRepo, sessionStore, renderer)

	// ── Auth Middleware ──────────────────────────────────────
	// При сборке с тегом noauth используется middleware без проверки
	// аутентификации (фейковые данные админа в Locals).
	// В обычном (production) билде — полноценный auth.AuthRequired.
	var authMiddleware fiber.Handler
	if auth.IsNoAuth() {
		authMiddleware = auth.NoAuthMiddleware()
	} else {
		authMiddleware = auth.AuthRequired(sessionStore)
	}

	// ── Admin Route Group (защищён AuthRequired или NoAuth) ─
	admin := app.Group("/admin", authMiddleware)

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

	// ── Specialties CRUD ────────────────────────────────────
	// Все маршруты регистрируются внутри RegisterRoutes:
	//   GET    /admin/specialties            → Index  (список)
	//   GET    /admin/specialties/new        → New    (форма создания)
	//   POST   /admin/specialties            → Create (создание)
	//   GET    /admin/specialties/:id/edit   → Edit   (форма редактирования)
	//   PUT    /admin/specialties/:id        → Update (обновление)
	//   POST   /admin/specialties/:id        → UpdatePost (graceful degradation)
	//   DELETE /admin/specialties/:id        → Delete (удаление)
	specGroup := admin.Group("/specialties")
	specHandlers.RegisterRoutes(specGroup)

	// ── Groups CRUD ─────────────────────────────────────────
	// Все маршруты регистрируются внутри RegisterRoutes:
	//   GET    /admin/groups            → Index  (список)
	//   GET    /admin/groups/new        → New    (форма создания)
	//   POST   /admin/groups            → Create (создание)
	//   GET    /admin/groups/:id/edit   → Edit   (форма редактирования)
	//   PUT    /admin/groups/:id        → Update (обновление)
	//   POST   /admin/groups/:id        → UpdatePost (graceful degradation)
	//   DELETE /admin/groups/:id        → Delete (удаление)
	grpGroup := admin.Group("/groups")
	groupHandlers.RegisterRoutes(grpGroup)

	// ── Subjects CRUD ───────────────────────────────────────
	// Все маршруты регистрируются внутри RegisterRoutes:
	//   GET    /admin/subjects            → Index  (список)
	//   GET    /admin/subjects/new        → New    (форма создания)
	//   POST   /admin/subjects            → Create (создание)
	//   GET    /admin/subjects/:id/edit   → Edit   (форма редактирования)
	//   PUT    /admin/subjects/:id        → Update (обновление)
	//   POST   /admin/subjects/:id        → UpdatePost (graceful degradation)
	//   DELETE /admin/subjects/:id        → Delete (удаление)
	subjGroup := admin.Group("/subjects")
	subjectHandlers.RegisterRoutes(subjGroup)

	// ── Stub routes (разделы в разработке) ──────────────────
	// Каждый раздел sidebar должен отдавать страницу, а не 404.
	// По мере реализации — заменяем stub на полноценный handler.

	type stubContent struct {
		Title       string
		Description string
	}

	stubHandler := func(title, description, navKey string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			return renderer.RenderPage(c, "stub.html", PageData{
				Title:     title,
				Admin:     adminDataFromLocals(c),
				ActiveNav: navKey,
				Content: stubContent{
					Title:       title,
					Description: description,
				},
			})
		}
	}

	admin.Get("/admins", stubHandler(
		"Администраторы",
		"Управление whitelist-ом email-адресов администраторов с доступом через Google OAuth. Этот раздел сейчас в разработке.",
		"admins",
	))

	return renderer
}

// adminDataFromLocals извлекает данные админа из c.Locals (заполняются middleware).
// Вынесено из Handlers, чтобы использовать и в stub-хендлерах.
func adminDataFromLocals(c *fiber.Ctx) AdminData {
	info := auth.GetAdminFromLocals(c)
	if info == nil {
		return AdminData{}
	}
	return AdminData{
		Email:     info.Email,
		Name:      info.Name,
		AvatarURL: info.AvatarURL,
	}
}
