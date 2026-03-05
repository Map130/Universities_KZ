// Package admin — хендлеры админ-панели для управления группами ОП.
package admin

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
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
	groupRepo   repository.SpecialtyGroupRepository
	subjectRepo repository.SubjectRepository
	store       *session.Store
	renderer    *Renderer
}

// NewGroupHandlers создаёт набор хендлеров для управления группами ОП.
func NewGroupHandlers(
	groupRepo repository.SpecialtyGroupRepository,
	subjectRepo repository.SubjectRepository,
	store *session.Store,
	renderer *Renderer,
) *GroupHandlers {
	return &GroupHandlers{
		groupRepo:   groupRepo,
		subjectRepo: subjectRepo,
		store:       store,
		renderer:    renderer,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Route Registration
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes регистрирует все маршруты админки групп ОП.
//
// Маршруты:
//
//	GET    /admin/groups                          → список групп ОП
//	GET    /admin/groups/new                      → форма создания
//	POST   /admin/groups                          → создание группы ОП
//	GET    /admin/groups/:id/edit                 → форма редактирования
//	PUT    /admin/groups/:id                      → обновление группы ОП
//	POST   /admin/groups/:id                      → UpdatePost (graceful degradation)
//	DELETE /admin/groups/:id                      → удаление группы ОП
//	POST   /admin/groups/:id/requires             → добавить связь с предметом
//	DELETE /admin/groups/:id/requires/:requires_id → удалить связь с предметом
func (h *GroupHandlers) RegisterRoutes(group fiber.Router) {
	group.Get("/", h.Index)
	group.Get("/new", h.New)
	group.Post("/", h.Create)
	group.Get("/:id/edit", h.Edit)
	group.Put("/:id", h.Update)
	group.Post("/:id", h.UpdatePost)
	group.Delete("/:id", h.Delete)

	// ── Requires (связки предметов ЕНТ) ─────────────────────
	group.Post("/:id/requires", h.AddRequires)
	group.Delete("/:id/requires/:requires_id", h.DeleteRequires)
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

	// AllSubjects — все доступные предметы ЕНТ (для dropdown).
	AllSubjects []models.Subject

	// RequiredSubjects — текущие связи группы с предметами ЕНТ (requires edges).
	RequiredSubjects []models.RequiredSubject

	// SelectedSubject1 — строковый ID первого выбранного предмета ЕНТ (для <select>).
	SelectedSubject1 string

	// SelectedSubject2 — строковый ID второго выбранного предмета ЕНТ (для <select>).
	SelectedSubject2 string
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
	allSubjects := h.loadAllSubjects(c)

	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     "Новая группа ОП",
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:           false,
			RecordID:         "",
			Group:            models.SpecialtyGroup{},
			AllSubjects:      allSubjects,
			RequiredSubjects: []models.RequiredSubject{},
			SelectedSubject1: "",
			SelectedSubject2: "",
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
	group, subj1, subj2, errs := h.parseGroupForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, false, "", group, subj1, subj2, errs)
	}

	created, err := h.groupRepo.Create(c.Context(), group)
	if err != nil {
		log.Printf("[admin/groups] Create: DB error: %v", err)
		errs["_global"] = "Ошибка при сохранении в базу данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, false, "", group, subj1, subj2, errs)
	}

	log.Printf("[admin/groups] Created: %v (%s — %s)", created.ID, created.Code, created.Name.RU)

	createdID := extractRecordIDValue(*created.ID)
	groupRecordID := surrealmodels.NewRecordID("specialty_group", createdID)

	// Привязываем предметы ЕНТ сразу при создании.
	h.saveRequiresEdges(c, groupRecordID, subj1, subj2)

	redirectURL := fmt.Sprintf("/admin/groups/%s/edit?flash=%s&flash_msg=%s",
		createdID,
		url.QueryEscape("success"),
		url.QueryEscape(fmt.Sprintf("Группа ОП «%s — %s» успешно создана", created.Code, created.Name.RU)),
	)
	return HTMXRedirect(c, redirectURL)
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

	group, requiredSubjects, err := h.groupRepo.GetWithSubjects(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/groups] Edit: GetWithSubjects(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Группа ОП не найдена")
	}

	allSubjects := h.loadAllSubjects(c)

	// Извлекаем текущие выбранные предметы из requires edges.
	sel1, sel2 := h.extractSelectedSubjects(requiredSubjects)

	// Flash-сообщения из query-параметров (например, после создания).
	var flash *FlashMessage
	if flashType := c.Query("flash"); flashType != "" {
		if flashMsg := c.Query("flash_msg"); flashMsg != "" {
			flash = &FlashMessage{
				Type:    flashType,
				Message: flashMsg,
			}
		}
	}

	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     fmt.Sprintf("Редактирование — %s", group.Code),
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Flash:     flash,
		Content: GroupFormData{
			IsEdit:           true,
			RecordID:         id,
			Group:            *group,
			AllSubjects:      allSubjects,
			RequiredSubjects: requiredSubjects,
			SelectedSubject1: sel1,
			SelectedSubject2: sel2,
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

	group, subj1, subj2, errs := h.parseGroupForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, true, id, group, subj1, subj2, errs)
	}

	updated, err := h.groupRepo.Update(c.Context(), recordID, group)
	if err != nil {
		log.Printf("[admin/groups] Update: DB error: %v", err)
		errs["_global"] = "Ошибка при обновлении в базе данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, true, id, group, subj1, subj2, errs)
	}

	log.Printf("[admin/groups] Updated: %v (%s — %s)", updated.ID, updated.Code, updated.Name.RU)

	// Пересоздаём связи requires (удаляем старые, создаём новые).
	h.saveRequiresEdges(c, recordID, subj1, subj2)

	redirectURL := fmt.Sprintf("/admin/groups/%s/edit?flash=%s&flash_msg=%s",
		id,
		url.QueryEscape("success"),
		url.QueryEscape(fmt.Sprintf("Группа ОП «%s — %s» успешно обновлена", updated.Code, updated.Name.RU)),
	)
	return HTMXRedirect(c, redirectURL)
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

	// Удаляем все связи requires перед удалением группы.
	if err := h.groupRepo.DeleteAllRequires(c.Context(), recordID); err != nil {
		log.Printf("[admin/groups] Delete: DeleteAllRequires(%s) error: %v", id, err)
		// Продолжаем удаление группы — ошибка не критична.
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
//  AddRequires — добавление связи группы ОП с предметом ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// AddRequires создаёт графовую связь specialty_group -> subject.
//
//	POST /admin/groups/:id/requires
//
// Form fields:
//   - subject_id: строковый ID предмета (без таблицы)
//   - priority:   1 (профильный) или 2 (второй)
func (h *GroupHandlers) AddRequires(c *fiber.Ctx) error {
	id := c.Params("id")
	groupRecordID := surrealmodels.NewRecordID("specialty_group", id)

	// Проверяем существование группы.
	_, err := h.groupRepo.GetByID(c.Context(), groupRecordID)
	if err != nil {
		log.Printf("[admin/groups] AddRequires: group %s not found: %v", id, err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusNotFound).SendString("Группа ОП не найдена")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Группа ОП не найдена")
	}

	subjectIDStr := strings.TrimSpace(c.FormValue("subject_id"))
	priorityStr := strings.TrimSpace(c.FormValue("priority"))

	if subjectIDStr == "" {
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusUnprocessableEntity).SendString("Выберите предмет ЕНТ")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Выберите предмет ЕНТ")
	}

	priority, err := strconv.Atoi(priorityStr)
	if err != nil || (priority != 1 && priority != 2) {
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusUnprocessableEntity).SendString("Приоритет должен быть 1 (профильный) или 2 (второй)")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Приоритет должен быть 1 (профильный) или 2 (второй)")
	}

	subjectRecordID := surrealmodels.NewRecordID("subject", subjectIDStr)

	// Проверяем существование предмета.
	subject, err := h.subjectRepo.GetByID(c.Context(), subjectRecordID)
	if err != nil {
		log.Printf("[admin/groups] AddRequires: subject %s not found: %v", subjectIDStr, err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusNotFound).SendString("Предмет ЕНТ не найден")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Предмет ЕНТ не найден")
	}

	_, err = h.groupRepo.CreateRequires(c.Context(), groupRecordID, subjectRecordID, models.CreateRequiresInput{
		Priority: models.SubjectPriority(priority),
	})
	if err != nil {
		log.Printf("[admin/groups] AddRequires: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка при создании связи")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Ошибка при создании связи с предметом")
	}

	priorityLabel := "профильный"
	if priority == 2 {
		priorityLabel = "второй"
	}

	log.Printf("[admin/groups] AddRequires: group=%s subject=%s (%s) priority=%d",
		id, subjectIDStr, subject.Name.RU, priority)

	return h.redirectToEditWithFlash(c, id, "success",
		fmt.Sprintf("Предмет «%s» (%s) привязан к группе", subject.Name.RU, priorityLabel))
}

// ────────────────────────────────────────────────────────────────────────────
//  DeleteRequires — удаление связи группы ОП с предметом ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// DeleteRequires удаляет графовую связь requires.
//
//	DELETE /admin/groups/:id/requires/:requires_id
func (h *GroupHandlers) DeleteRequires(c *fiber.Ctx) error {
	id := c.Params("id")
	requiresID := c.Params("requires_id")
	requiresRecordID := surrealmodels.NewRecordID("requires", requiresID)

	if err := h.groupRepo.DeleteRequires(c.Context(), requiresRecordID); err != nil {
		log.Printf("[admin/groups] DeleteRequires: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления связи")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Ошибка при удалении связи с предметом")
	}

	log.Printf("[admin/groups] DeleteRequires: group=%s requires=%s deleted", id, requiresID)

	if isHTMXRequest(c) {
		// Возвращаем пустую строку — HTMX удалит строку из таблицы (hx-swap="outerHTML").
		return c.SendString("")
	}

	return h.redirectToEditWithFlash(c, id, "success", "Связь с предметом удалена")
}

// ────────────────────────────────────────────────────────────────────────────
//  Вспомогательные методы
// ────────────────────────────────────────────────────────────────────────────

// parseGroupForm извлекает и валидирует данные формы группы ОП.
// Возвращает модель SpecialtyGroup и map ошибок.
func (h *GroupHandlers) parseGroupForm(c *fiber.Ctx) (models.SpecialtyGroup, string, string, map[string]string) {
	errs := make(map[string]string)

	code := strings.TrimSpace(c.FormValue("code"))
	nameRU := strings.TrimSpace(c.FormValue("name_ru"))
	nameKZ := strings.TrimSpace(c.FormValue("name_kz"))
	nameEN := strings.TrimSpace(c.FormValue("name_en"))

	subject1 := strings.TrimSpace(c.FormValue("subject_1"))
	subject2 := strings.TrimSpace(c.FormValue("subject_2"))

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
	if subject1 == "" {
		errs["subject_1"] = "Выберите первый предмет ЕНТ"
	}
	if subject2 == "" {
		errs["subject_2"] = "Выберите второй предмет ЕНТ"
	}
	if subject1 != "" && subject2 != "" && subject1 == subject2 {
		errs["subject_2"] = "Предметы ЕНТ должны быть разными"
	}

	group := models.SpecialtyGroup{
		Code: code,
		Name: models.LocalizedName{
			KZ: nameKZ,
			RU: nameRU,
			EN: nameEN,
		},
	}

	return group, subject1, subject2, errs
}

// renderFormWithErrors рендерит форму с ошибками валидации.
func (h *GroupHandlers) renderFormWithErrors(
	c *fiber.Ctx,
	isEdit bool,
	recordID string,
	group models.SpecialtyGroup,
	selectedSubject1 string,
	selectedSubject2 string,
	errs map[string]string,
) error {
	title := "Новая группа ОП"
	if isEdit {
		title = fmt.Sprintf("Редактирование — %s", group.Code)
	}

	allSubjects := h.loadAllSubjects(c)

	// Загружаем текущие связи requires (если редактирование).
	var requiredSubjects []models.RequiredSubject
	if isEdit && recordID != "" {
		groupRecordID := surrealmodels.NewRecordID("specialty_group", recordID)
		_, reqs, err := h.groupRepo.GetWithSubjects(c.Context(), groupRecordID)
		if err != nil {
			log.Printf("[admin/groups] renderFormWithErrors: GetWithSubjects error: %v", err)
			requiredSubjects = []models.RequiredSubject{}
		} else {
			requiredSubjects = reqs
		}
	}

	c.Status(fiber.StatusUnprocessableEntity)

	return h.renderer.RenderPage(c, "groups/form.html", PageData{
		Title:     title,
		Admin:     h.adminData(c),
		ActiveNav: "groups",
		Errors:    errs,
		Content: GroupFormData{
			IsEdit:           isEdit,
			RecordID:         recordID,
			Group:            group,
			AllSubjects:      allSubjects,
			RequiredSubjects: requiredSubjects,
			SelectedSubject1: selectedSubject1,
			SelectedSubject2: selectedSubject2,
		},
	})
}

// loadAllSubjects загружает все предметы ЕНТ (для select-dropdown).
func (h *GroupHandlers) loadAllSubjects(c *fiber.Ctx) []models.Subject {
	subjects, err := h.subjectRepo.GetAll(c.Context())
	if err != nil {
		log.Printf("[admin/groups] loadAllSubjects: GetAll error: %v", err)
		return []models.Subject{}
	}
	return subjects
}

// saveRequiresEdges удаляет все текущие связи requires и создаёт новые
// для двух выбранных предметов ЕНТ.
func (h *GroupHandlers) saveRequiresEdges(c *fiber.Ctx, groupID surrealmodels.RecordID, subj1, subj2 string) {
	// Удаляем все существующие связи.
	if err := h.groupRepo.DeleteAllRequires(c.Context(), groupID); err != nil {
		log.Printf("[admin/groups] saveRequiresEdges: DeleteAllRequires error: %v", err)
	}

	// Создаём связь для первого предмета (priority=1).
	if subj1 != "" {
		subjectRecordID := surrealmodels.NewRecordID("subject", subj1)
		if _, err := h.groupRepo.CreateRequires(c.Context(), groupID, subjectRecordID, models.CreateRequiresInput{
			Priority: models.SubjectPriorityProfile,
		}); err != nil {
			log.Printf("[admin/groups] saveRequiresEdges: CreateRequires subject1 error: %v", err)
		}
	}

	// Создаём связь для второго предмета (priority=2).
	if subj2 != "" {
		subjectRecordID := surrealmodels.NewRecordID("subject", subj2)
		if _, err := h.groupRepo.CreateRequires(c.Context(), groupID, subjectRecordID, models.CreateRequiresInput{
			Priority: models.SubjectPrioritySecondary,
		}); err != nil {
			log.Printf("[admin/groups] saveRequiresEdges: CreateRequires subject2 error: %v", err)
		}
	}
}

// extractSelectedSubjects извлекает строковые ID выбранных предметов из requires edges.
// Возвращает (subject1_id, subject2_id) на основе приоритета.
func (h *GroupHandlers) extractSelectedSubjects(reqs []models.RequiredSubject) (string, string) {
	var sel1, sel2 string
	for _, req := range reqs {
		subjectID := ""
		if req.Out.ID != nil {
			subjectID = extractRecordIDValue(*req.Out.ID)
		}
		switch req.Priority {
		case models.SubjectPriorityProfile:
			sel1 = subjectID
		case models.SubjectPrioritySecondary:
			sel2 = subjectID
		}
	}
	return sel1, sel2
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

// redirectToEditWithFlash выполняет редирект на форму редактирования группы ОП с flash-сообщением.
func (h *GroupHandlers) redirectToEditWithFlash(c *fiber.Ctx, id, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/groups/%s/edit?flash=%s&flash_msg=%s",
		id,
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}
