// Package admin — хендлеры админ-панели для управления группами ОП.
package admin

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
)

// ────────────────────────────────────────────────────────────────────────────
//  GroupHandlers — набор HTTP-хендлеров для админки групп ОП
// ────────────────────────────────────────────────────────────────────────────

// GroupHandlers объединяет зависимости для HTTP-хендлеров групп ОП.
type GroupHandlers struct {
	groupRepo repository.SpecialtyGroupRepository
	store     *session.Store
	renderer  *Renderer
}

// NewGroupHandlers создаёт набор хендлеров для управления группами ОП.
func NewGroupHandlers(
	groupRepo repository.SpecialtyGroupRepository,
	store *session.Store,
	renderer *Renderer,
) *GroupHandlers {
	return &GroupHandlers{
		groupRepo: groupRepo,
		store:     store,
		renderer:  renderer,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Route Registration
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes регистрирует все маршруты админки групп ОП.
//
// Маршруты:
//
//	GET    /admin/groups            → список групп ОП
//	GET    /admin/groups/new        → форма создания
//	POST   /admin/groups            → создание группы ОП
//	GET    /admin/groups/:id/edit   → форма редактирования
//	PUT    /admin/groups/:id        → обновление группы ОП
//	POST   /admin/groups/:id        → UpdatePost (graceful degradation)
//	DELETE /admin/groups/:id        → удаление группы ОП
func (h *GroupHandlers) RegisterRoutes(group fiber.Router) {
	group.Get("/", h.Index)
	group.Get("/new", h.New)
	group.Post("/", h.Create)
	group.Get("/:id/edit", h.Edit)
	group.Put("/:id", h.Update)
	group.Post("/:id", h.UpdatePost)
	group.Delete("/:id", h.Delete)
}

// ────────────────────────────────────────────────────────────────────────────
//  View Models (данные для шаблонов)
// ────────────────────────────────────────────────────────────────────────────

// GroupsListData передаётся в шаблон groups/index.html.
type GroupsListData struct {
	Groups []models.SpecialtyGroup
	Search string
}

// GroupFormData передаётся в шаблон groups/form.html.
type GroupFormData struct {
	// IsEdit — true для формы редактирования, false — создания.
	IsEdit bool

	// RecordID — строковый ID группы ОП (для URL).
	RecordID string

	// Group — данные группы ОП (заполненные или пустые).
	Group models.SpecialtyGroup
}

// ────────────────────────────────────────────────────────────────────────────
//  Index — список групп ОП
// ────────────────────────────────────────────────────────────────────────────

// Index рендерит страницу со списком групп ОП.
//
//	GET /admin/groups
//	GET /admin/groups?search=...
func (h *GroupHandlers) Index(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("search"))

	filters := models.SpecialtyGroupFilters{
		Search: search,
		Lang:   "ru",
		Limit:  200,
	}

	groups, err := h.groupRepo.GetAll(c.Context(), filters)
	if err != nil {
		log.Printf("[admin/groups] Index: GetAll error: %v", err)
		groups = []models.SpecialtyGroup{}
	}

	// Flash-сообщения из query-параметров.
	var flash *FlashMessage
	if flashType := c.Query("flash"); flashType != "" {
		if flashMsg := c.Query("flash_msg"); flashMsg != "" {
			flash = &FlashMessage{
				Type:    flashType,
				Message: flashMsg,
			}
		}
	}

	return h.renderer.RenderPage(c, "groups/index.html", PageData{
		Title:     "Группы ОП",
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Flash:     flash,
		Content: GroupsListData{
			Groups: groups,
			Search: search,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  New — форма создания
// ────────────────────────────────────────────────────────────────────────────

// New рендерит пустую форму для создания новой группы ОП.
//
//	GET /admin/groups/new
func (h *GroupHandlers) New(c *fiber.Ctx) error {
	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     "Новая группа ОП",
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   false,
			RecordID: "",
			Group:    models.SpecialtyGroup{},
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Create — создание группы ОП
// ────────────────────────────────────────────────────────────────────────────

// Create обрабатывает POST-запрос на создание группы ОП.
//
//	POST /admin/groups
func (h *GroupHandlers) Create(c *fiber.Ctx) error {
	group, errs := h.parseGroupForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, false, "", group, errs)
	}

	created, err := h.groupRepo.Create(c.Context(), group)
	if err != nil {
		log.Printf("[admin/groups] Create: DB error: %v", err)
		errs["_global"] = "Ошибка при сохранении в базу данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, false, "", group, errs)
	}

	log.Printf("[admin/groups] Created: %v (%s — %s)", created.ID, created.Code, created.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Группа ОП «%s — %s» успешно создана", created.Code, created.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Edit — форма редактирования
// ────────────────────────────────────────────────────────────────────────────

// Edit рендерит форму редактирования существующей группы ОП.
//
//	GET /admin/groups/:id/edit
func (h *GroupHandlers) Edit(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty_group", id)

	group, err := h.groupRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/groups] Edit: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Группа ОП не найдена")
	}

	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     fmt.Sprintf("Редактирование — %s", group.Code),
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   true,
			RecordID: id,
			Group:    *group,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Update — обновление группы ОП
// ────────────────────────────────────────────────────────────────────────────

// Update обрабатывает PUT-запрос на обновление группы ОП.
//
//	PUT /admin/groups/:id
func (h *GroupHandlers) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty_group", id)

	// Проверяем существование.
	_, err := h.groupRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/groups] Update: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Группа ОП не найдена")
	}

	group, errs := h.parseGroupForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, true, id, group, errs)
	}

	updated, err := h.groupRepo.Update(c.Context(), recordID, group)
	if err != nil {
		log.Printf("[admin/groups] Update: DB error: %v", err)
		errs["_global"] = "Ошибка при обновлении в базе данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, true, id, group, errs)
	}

	log.Printf("[admin/groups] Updated: %v (%s — %s)", updated.ID, updated.Code, updated.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Группа ОП «%s — %s» успешно обновлена", updated.Code, updated.Name.RU))
}

// UpdatePost обрабатывает POST-запрос с _method=PUT (Graceful Degradation).
//
//	POST /admin/groups/:id
func (h *GroupHandlers) UpdatePost(c *fiber.Ctx) error {
	method := strings.ToUpper(c.FormValue("_method"))
	if method == "PUT" {
		return h.Update(c)
	}
	return c.Status(fiber.StatusMethodNotAllowed).SendString("Method Not Allowed")
}

// ────────────────────────────────────────────────────────────────────────────
//  Delete — удаление группы ОП
// ────────────────────────────────────────────────────────────────────────────

// Delete удаляет группу ОП по ID.
//
//	DELETE /admin/groups/:id
func (h *GroupHandlers) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty_group", id)

	existing, err := h.groupRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/groups] Delete: GetByID(%s) error: %v", id, err)
		if isHTMXRequest(c) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return h.redirectToListWithFlash(c, "error", "Группа ОП не найдена")
	}

	if err := h.groupRepo.Delete(c.Context(), recordID); err != nil {
		log.Printf("[admin/groups] Delete: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления")
		}
		return h.redirectToListWithFlash(c, "error", "Ошибка при удалении группы ОП")
	}

	log.Printf("[admin/groups] Deleted: %s (%s — %s)", id, existing.Code, existing.Name.RU)

	if isHTMXRequest(c) {
		return c.SendString("")
	}

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Группа ОП «%s — %s» удалена", existing.Code, existing.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Вспомогательные методы
// ────────────────────────────────────────────────────────────────────────────

// parseGroupForm извлекает и валидирует данные формы группы ОП.
// Возвращает модель SpecialtyGroup и map ошибок.
func (h *GroupHandlers) parseGroupForm(c *fiber.Ctx) (models.SpecialtyGroup, map[string]string) {
	errs := make(map[string]string)

	code := strings.TrimSpace(c.FormValue("code"))
	nameRU := strings.TrimSpace(c.FormValue("name_ru"))
	nameKZ := strings.TrimSpace(c.FormValue("name_kz"))
	nameEN := strings.TrimSpace(c.FormValue("name_en"))

	// Валидация обязательных полей.
	if code == "" {
		errs["code"] = "Код группы ОП обязателен"
	}
	if nameRU == "" {
		errs["name_ru"] = "Название на русском обязательно"
	}
	if nameKZ == "" {
		errs["name_kz"] = "Название на казахском обязательно"
	}
	if nameEN == "" {
		errs["name_en"] = "Название на английском обязательно"
	}

	group := models.SpecialtyGroup{
		Code: code,
		Name: models.LocalizedName{
			KZ: nameKZ,
			RU: nameRU,
			EN: nameEN,
		},
	}

	return group, errs
}

// renderFormWithErrors рендерит форму с ошибками валидации.
func (h *GroupHandlers) renderFormWithErrors(
	c *fiber.Ctx,
	isEdit bool,
	recordID string,
	group models.SpecialtyGroup,
	errs map[string]string,
) error {
	title := "Новая группа ОП"
	if isEdit {
		title = fmt.Sprintf("Редактирование — %s", group.Code)
	}

	c.Status(fiber.StatusUnprocessableEntity)

	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     title,
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Errors:    errs,
		Content: GroupFormData{
			IsEdit:   isEdit,
			RecordID: recordID,
			Group:    group,
		},
	})
}

// adminData извлекает данные администратора из c.Locals.
func (h *GroupHandlers) adminData(c *fiber.Ctx) AdminData {
	return adminDataFromLocals(c)
}

// redirectToListWithFlash выполняет редирект на список групп ОП с flash-сообщением.
func (h *GroupHandlers) redirectToListWithFlash(c *fiber.Ctx, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/groups?flash=%s&flash_msg=%s",
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}
