// Package admin — хендлеры админ-панели для управления специальностями.
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
//  SpecialtyHandlers — набор HTTP-хендлеров для админки специальностей
// ────────────────────────────────────────────────────────────────────────────

// SpecialtyHandlers объединяет зависимости для HTTP-хендлеров специальностей.
type SpecialtyHandlers struct {
	specRepo  repository.SpecialtyRepository
	groupRepo repository.SpecialtyGroupRepository
	store     *session.Store
	renderer  *Renderer
}

// NewSpecialtyHandlers создаёт набор хендлеров для управления специальностями.
func NewSpecialtyHandlers(
	specRepo repository.SpecialtyRepository,
	groupRepo repository.SpecialtyGroupRepository,
	store *session.Store,
	renderer *Renderer,
) *SpecialtyHandlers {
	return &SpecialtyHandlers{
		specRepo:  specRepo,
		groupRepo: groupRepo,
		store:     store,
		renderer:  renderer,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Route Registration
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes регистрирует все маршруты админки специальностей.
//
// Маршруты:
//
//	GET    /admin/specialties            → список специальностей
//	GET    /admin/specialties/new        → форма создания
//	POST   /admin/specialties            → создание специальности
//	GET    /admin/specialties/:id/edit   → форма редактирования
//	PUT    /admin/specialties/:id        → обновление специальности
//	POST   /admin/specialties/:id        → UpdatePost (graceful degradation)
//	DELETE /admin/specialties/:id        → удаление специальности
func (h *SpecialtyHandlers) RegisterRoutes(group fiber.Router) {
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

// SpecialtyListItem — строка списка специальностей (с денормализованной группой).
type SpecialtyListItem struct {
	// Встроенные поля из models.Specialty
	ID   *surrealmodels.RecordID `json:"id,omitempty"`
	Code string                  `json:"code"`
	Name models.LocalizedName    `json:"name"`

	// Денормализованные поля группы (для отображения в таблице без JOIN).
	GroupCode string `json:"group_code"`
	GroupName string `json:"group_name"`
}

// SpecialtiesListData передаётся в шаблон specialties/index.html.
type SpecialtiesListData struct {
	Specialties []SpecialtyListItem
	Groups      []models.SpecialtyGroup // для dropdown-фильтра
	Search      string
	GroupFilter string
}

// SpecialtyFormData передаётся в шаблон specialties/form.html.
type SpecialtyFormData struct {
	// IsEdit — true для формы редактирования, false — создания.
	IsEdit bool

	// RecordID — строковый ID специальности (для URL).
	RecordID string

	// Specialty — данные специальности (заполненные или пустые).
	Specialty models.Specialty

	// Groups — список всех групп ОП для select-dropdown.
	Groups []models.SpecialtyGroup

	// SelectedGroupID — строковый ID выбранной группы
	// (для восстановления selected при ошибке валидации).
	SelectedGroupID string
}

// ────────────────────────────────────────────────────────────────────────────
//  Index — список специальностей
// ────────────────────────────────────────────────────────────────────────────

// Index рендерит страницу со списком специальностей.
//
//	GET /admin/specialties
//	GET /admin/specialties?search=...&group_code=...
func (h *SpecialtyHandlers) Index(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("search"))
	groupCode := strings.TrimSpace(c.Query("group_code"))

	filters := models.SpecialtyFilters{
		Search: search,
		Lang:   "ru",
		Limit:  200,
	}
	if groupCode != "" {
		filters.GroupCode = groupCode
	}

	specialties, err := h.specRepo.GetAll(c.Context(), filters)
	if err != nil {
		log.Printf("[admin/specialties] Index: GetAll error: %v", err)
		specialties = []models.Specialty{}
	}

	// Загружаем все группы ОП (для dropdown-фильтра и для денормализации).
	allGroups, err := h.groupRepo.GetAll(c.Context(), models.SpecialtyGroupFilters{Limit: 500})
	if err != nil {
		log.Printf("[admin/specialties] Index: GetAll groups error: %v", err)
		allGroups = []models.SpecialtyGroup{}
	}

	// Строим map группа ID → группа для быстрого lookup.
	groupMap := make(map[string]models.SpecialtyGroup, len(allGroups))
	for _, g := range allGroups {
		if g.ID != nil {
			groupMap[g.ID.String()] = g
		}
	}

	// Собираем view-модели для таблицы.
	items := make([]SpecialtyListItem, 0, len(specialties))
	for _, s := range specialties {
		item := SpecialtyListItem{
			ID:   s.ID,
			Code: s.Code,
			Name: s.Name,
		}
		// Resolve group code/name из record link.
		groupKey := s.Group.String()
		if g, ok := groupMap[groupKey]; ok {
			item.GroupCode = g.Code
			item.GroupName = g.Name.RU
		} else {
			// Fallback: показываем raw record link.
			item.GroupCode = groupKey
		}
		items = append(items, item)
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

	return h.renderer.RenderPage(c, "specialties/index.html", PageData{
		Title:     "Специальности",
		Admin:     h.adminData(c),
		ActiveNav: "specialties",
		Flash:     flash,
		Content: SpecialtiesListData{
			Specialties: items,
			Groups:      allGroups,
			Search:      search,
			GroupFilter: groupCode,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  New — форма создания
// ────────────────────────────────────────────────────────────────────────────

// New рендерит пустую форму для создания новой специальности.
//
//	GET /admin/specialties/new
func (h *SpecialtyHandlers) New(c *fiber.Ctx) error {
	groups, err := h.groupRepo.GetAll(c.Context(), models.SpecialtyGroupFilters{Limit: 500})
	if err != nil {
		log.Printf("[admin/specialties] New: GetAll groups error: %v", err)
		groups = []models.SpecialtyGroup{}
	}

	return h.renderer.RenderPage(c, "specialties/form.html", PageData{
		Title:     "Новая специальность",
		Admin:     h.adminData(c),
		ActiveNav: "specialties",
		Content: SpecialtyFormData{
			IsEdit:    false,
			RecordID:  "",
			Specialty: models.Specialty{},
			Groups:    groups,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Create — создание специальности
// ────────────────────────────────────────────────────────────────────────────

// Create обрабатывает POST-запрос на создание специальности.
//
//	POST /admin/specialties
func (h *SpecialtyHandlers) Create(c *fiber.Ctx) error {
	spec, selectedGroupID, errs := h.parseSpecialtyForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, false, "", spec, selectedGroupID, errs)
	}

	created, err := h.specRepo.Create(c.Context(), spec)
	if err != nil {
		log.Printf("[admin/specialties] Create: DB error: %v", err)
		errs["_global"] = "Ошибка при сохранении в базу данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, false, "", spec, selectedGroupID, errs)
	}

	log.Printf("[admin/specialties] Created: %v (%s — %s)", created.ID, created.Code, created.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Специальность «%s — %s» успешно создана", created.Code, created.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Edit — форма редактирования
// ────────────────────────────────────────────────────────────────────────────

// Edit рендерит форму редактирования существующей специальности.
//
//	GET /admin/specialties/:id/edit
func (h *SpecialtyHandlers) Edit(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty", id)

	spec, err := h.specRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/specialties] Edit: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Специальность не найдена")
	}

	groups, err := h.groupRepo.GetAll(c.Context(), models.SpecialtyGroupFilters{Limit: 500})
	if err != nil {
		log.Printf("[admin/specialties] Edit: GetAll groups error: %v", err)
		groups = []models.SpecialtyGroup{}
	}

	// Извлекаем строковый ID группы для selected в <select>.
	selectedGroupID := extractRecordIDValue(spec.Group)

	return h.renderer.RenderPage(c, "specialties/form.html", PageData{
		Title:     fmt.Sprintf("Редактирование — %s", spec.Code),
		Admin:     h.adminData(c),
		ActiveNav: "specialties",
		Content: SpecialtyFormData{
			IsEdit:          true,
			RecordID:        id,
			Specialty:       *spec,
			Groups:          groups,
			SelectedGroupID: selectedGroupID,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Update — обновление специальности
// ────────────────────────────────────────────────────────────────────────────

// Update обрабатывает PUT-запрос на обновление специальности.
//
//	PUT /admin/specialties/:id
func (h *SpecialtyHandlers) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty", id)

	// Проверяем существование.
	_, err := h.specRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/specialties] Update: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Специальность не найдена")
	}

	spec, selectedGroupID, errs := h.parseSpecialtyForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, true, id, spec, selectedGroupID, errs)
	}

	updated, err := h.specRepo.Update(c.Context(), recordID, spec)
	if err != nil {
		log.Printf("[admin/specialties] Update: DB error: %v", err)
		errs["_global"] = "Ошибка при обновлении в базе данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, true, id, spec, selectedGroupID, errs)
	}

	log.Printf("[admin/specialties] Updated: %v (%s — %s)", updated.ID, updated.Code, updated.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Специальность «%s — %s» успешно обновлена", updated.Code, updated.Name.RU))
}

// UpdatePost обрабатывает POST-запрос с _method=PUT (Graceful Degradation).
//
//	POST /admin/specialties/:id
func (h *SpecialtyHandlers) UpdatePost(c *fiber.Ctx) error {
	method := strings.ToUpper(c.FormValue("_method"))
	if method == "PUT" {
		return h.Update(c)
	}
	return c.Status(fiber.StatusMethodNotAllowed).SendString("Method Not Allowed")
}

// ────────────────────────────────────────────────────────────────────────────
//  Delete — удаление специальности
// ────────────────────────────────────────────────────────────────────────────

// Delete удаляет специальность по ID.
//
//	DELETE /admin/specialties/:id
func (h *SpecialtyHandlers) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("specialty", id)

	existing, err := h.specRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/specialties] Delete: GetByID(%s) error: %v", id, err)
		if isHTMXRequest(c) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return h.redirectToListWithFlash(c, "error", "Специальность не найдена")
	}

	if err := h.specRepo.Delete(c.Context(), recordID); err != nil {
		log.Printf("[admin/specialties] Delete: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления")
		}
		return h.redirectToListWithFlash(c, "error", "Ошибка при удалении специальности")
	}

	log.Printf("[admin/specialties] Deleted: %s (%s — %s)", id, existing.Code, existing.Name.RU)

	if isHTMXRequest(c) {
		return c.SendString("")
	}

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Специальность «%s — %s» удалена", existing.Code, existing.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Вспомогательные методы
// ────────────────────────────────────────────────────────────────────────────

// parseSpecialtyForm извлекает и валидирует данные формы специальности.
// Возвращает модель Specialty, строковый ID выбранной группы и map ошибок.
func (h *SpecialtyHandlers) parseSpecialtyForm(c *fiber.Ctx) (models.Specialty, string, map[string]string) {
	errs := make(map[string]string)

	code := strings.TrimSpace(c.FormValue("code"))
	nameRU := strings.TrimSpace(c.FormValue("name_ru"))
	nameKZ := strings.TrimSpace(c.FormValue("name_kz"))
	nameEN := strings.TrimSpace(c.FormValue("name_en"))
	groupIDStr := strings.TrimSpace(c.FormValue("group_id"))

	// Валидация обязательных полей.
	if code == "" {
		errs["code"] = "Код специальности обязателен"
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
	if groupIDStr == "" {
		errs["group_id"] = "Выберите группу образовательных программ"
	}

	spec := models.Specialty{
		Code: code,
		Name: models.LocalizedName{
			KZ: nameKZ,
			RU: nameRU,
			EN: nameEN,
		},
	}

	// Устанавливаем record link на группу ОП.
	if groupIDStr != "" {
		spec.Group = surrealmodels.NewRecordID("specialty_group", groupIDStr)
	}

	return spec, groupIDStr, errs
}

// renderFormWithErrors рендерит форму с ошибками валидации.
func (h *SpecialtyHandlers) renderFormWithErrors(
	c *fiber.Ctx,
	isEdit bool,
	recordID string,
	spec models.Specialty,
	selectedGroupID string,
	errs map[string]string,
) error {
	title := "Новая специальность"
	if isEdit {
		title = fmt.Sprintf("Редактирование — %s", spec.Code)
	}

	groups, err := h.groupRepo.GetAll(c.Context(), models.SpecialtyGroupFilters{Limit: 500})
	if err != nil {
		log.Printf("[admin/specialties] renderFormWithErrors: GetAll groups error: %v", err)
		groups = []models.SpecialtyGroup{}
	}

	c.Status(fiber.StatusUnprocessableEntity)

	return h.renderer.RenderPage(c, "specialties/form.html", PageData{
		Title:     title,
		Admin:     h.adminData(c),
		ActiveNav: "specialties",
		Errors:    errs,
		Content: SpecialtyFormData{
			IsEdit:          isEdit,
			RecordID:        recordID,
			Specialty:       spec,
			Groups:          groups,
			SelectedGroupID: selectedGroupID,
		},
	})
}

// adminData извлекает данные администратора из c.Locals.
func (h *SpecialtyHandlers) adminData(c *fiber.Ctx) AdminData {
	return adminDataFromLocals(c)
}

// redirectToListWithFlash выполняет редирект на список специальностей с flash-сообщением.
func (h *SpecialtyHandlers) redirectToListWithFlash(c *fiber.Ctx, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/specialties?flash=%s&flash_msg=%s",
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}

// extractRecordIDValue извлекает строковую часть ID из RecordID.
// Например, из "specialty_group:abc123" возвращает "abc123".
// Убирает angle-bracket escaping (⟨...⟩), если есть.
func extractRecordIDValue(rid surrealmodels.RecordID) string {
	s := rid.String()
	if idx := strings.Index(s, ":"); idx >= 0 {
		id := s[idx+1:]
		id = strings.TrimPrefix(id, "⟨")
		id = strings.TrimSuffix(id, "⟩")
		return id
	}
	return s
}
