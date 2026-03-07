// Package admin — хендлеры админ-панели для управления предметами ЕНТ.
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
//  SubjectHandlers — набор HTTP-хендлеров для админки предметов ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// SubjectHandlers объединяет зависимости для HTTP-хендлеров предметов ЕНТ.
type SubjectHandlers struct {
	subjectRepo repository.SubjectRepository
	store       *session.Store
	renderer    *Renderer
}

// NewSubjectHandlers создаёт набор хендлеров для управления предметами ЕНТ.
func NewSubjectHandlers(
	subjectRepo repository.SubjectRepository,
	store *session.Store,
	renderer *Renderer,
) *SubjectHandlers {
	return &SubjectHandlers{
		subjectRepo: subjectRepo,
		store:       store,
		renderer:    renderer,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Route Registration
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes регистрирует все маршруты админки предметов ЕНТ.
//
// Маршруты:
//
//	GET    /admin/subjects            → список предметов
//	GET    /admin/subjects/new        → форма создания
//	POST   /admin/subjects            → создание предмета
//	GET    /admin/subjects/:id/edit   → форма редактирования
//	PUT    /admin/subjects/:id        → обновление предмета
//	POST   /admin/subjects/:id        → UpdatePost (graceful degradation)
//	DELETE /admin/subjects/:id        → удаление предмета
func (h *SubjectHandlers) RegisterRoutes(group fiber.Router) {
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

// SubjectsListData передаётся в шаблон subjects/index.html.
type SubjectsListData struct {
	Subjects []models.Subject
	Search   string
}

// SubjectFormData передаётся в шаблон subjects/form.html.
type SubjectFormData struct {
	// IsEdit — true для формы редактирования, false — создания.
	IsEdit bool

	// RecordID — строковый ID предмета (для URL).
	RecordID string

	// Subject — данные предмета (заполненные или пустые).
	Subject models.Subject
}

// ────────────────────────────────────────────────────────────────────────────
//  Index — список предметов ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// Index рендерит страницу со списком предметов ЕНТ.
//
//	GET /admin/subjects
//	GET /admin/subjects?search=...
func (h *SubjectHandlers) Index(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("search"))

	allSubjects, err := h.subjectRepo.GetAll(c.Context())
	if err != nil {
		log.Printf("[admin/subjects] Index: GetAll error: %v", err)
		allSubjects = []models.Subject{}
	}

	// Фильтрация на стороне Go (в SubjectRepository нет полнотекстового поиска).
	var subjects []models.Subject
	if search != "" {
		lower := strings.ToLower(search)
		for _, s := range allSubjects {
			if strings.Contains(strings.ToLower(s.Name.RU), lower) ||
				strings.Contains(strings.ToLower(s.Name.KZ), lower) ||
				strings.Contains(strings.ToLower(s.Name.EN), lower) {
				subjects = append(subjects, s)
			}
		}
	} else {
		subjects = allSubjects
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

	return h.renderer.RenderPage(c, "subjects/index.html", PageData{
		Title:     "Предметы ЕНТ",
		Admin:     h.adminData(c),
		ActiveNav: "subjects",
		Flash:     flash,
		Content: SubjectsListData{
			Subjects: subjects,
			Search:   search,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  New — форма создания
// ────────────────────────────────────────────────────────────────────────────

// New рендерит пустую форму для создания нового предмета ЕНТ.
//
//	GET /admin/subjects/new
func (h *SubjectHandlers) New(c *fiber.Ctx) error {
	return h.renderer.RenderPage(c, "subjects/form.html", PageData{
		Title:     "Новый предмет ЕНТ",
		Admin:     h.adminData(c),
		ActiveNav: "subjects",
		Content: SubjectFormData{
			IsEdit:   false,
			RecordID: "",
			Subject:  models.Subject{},
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Create — создание предмета ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// Create обрабатывает POST-запрос на создание предмета ЕНТ.
//
//	POST /admin/subjects
func (h *SubjectHandlers) Create(c *fiber.Ctx) error {
	subject, errs := h.parseSubjectForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, false, "", subject, errs)
	}

	created, err := h.subjectRepo.Create(c.Context(), subject)
	if err != nil {
		log.Printf("[admin/subjects] Create: DB error: %v", err)
		errs["_global"] = "Ошибка при сохранении в базу данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, false, "", subject, errs)
	}

	log.Printf("[admin/subjects] Created: %v (%s)", created.ID, created.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Предмет «%s» успешно создан", created.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Edit — форма редактирования
// ────────────────────────────────────────────────────────────────────────────

// Edit рендерит форму редактирования существующего предмета ЕНТ.
//
//	GET /admin/subjects/:id/edit
func (h *SubjectHandlers) Edit(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("subject", id)

	subject, err := h.subjectRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/subjects] Edit: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Предмет ЕНТ не найден")
	}

	return h.renderer.RenderPage(c, "subjects/form.html", PageData{
		Title:     fmt.Sprintf("Редактирование — %s", subject.Name.RU),
		Admin:     h.adminData(c),
		ActiveNav: "subjects",
		Content: SubjectFormData{
			IsEdit:   true,
			RecordID: id,
			Subject:  *subject,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Update — обновление предмета ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// Update обрабатывает PUT-запрос на обновление предмета ЕНТ.
//
//	PUT /admin/subjects/:id
func (h *SubjectHandlers) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("subject", id)

	// Проверяем существование.
	_, err := h.subjectRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/subjects] Update: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Предмет ЕНТ не найден")
	}

	subject, errs := h.parseSubjectForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, true, id, subject, errs)
	}

	updated, err := h.subjectRepo.Update(c.Context(), recordID, subject)
	if err != nil {
		log.Printf("[admin/subjects] Update: DB error: %v", err)
		errs["_global"] = "Ошибка при обновлении в базе данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, true, id, subject, errs)
	}

	log.Printf("[admin/subjects] Updated: %v (%s)", updated.ID, updated.Name.RU)

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Предмет «%s» успешно обновлён", updated.Name.RU))
}

// UpdatePost обрабатывает POST-запрос с _method=PUT (Graceful Degradation).
//
//	POST /admin/subjects/:id
func (h *SubjectHandlers) UpdatePost(c *fiber.Ctx) error {
	method := strings.ToUpper(c.FormValue("_method"))
	if method == "PUT" {
		return h.Update(c)
	}
	return c.Status(fiber.StatusMethodNotAllowed).SendString("Method Not Allowed")
}

// ────────────────────────────────────────────────────────────────────────────
//  Delete — удаление предмета ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// Delete удаляет предмет ЕНТ по ID.
//
//	DELETE /admin/subjects/:id
func (h *SubjectHandlers) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("subject", id)

	existing, err := h.subjectRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/subjects] Delete: GetByID(%s) error: %v", id, err)
		if isHTMXRequest(c) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return h.redirectToListWithFlash(c, "error", "Предмет ЕНТ не найден")
	}

	if err := h.subjectRepo.Delete(c.Context(), recordID); err != nil {
		log.Printf("[admin/subjects] Delete: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления")
		}
		return h.redirectToListWithFlash(c, "error", "Ошибка при удалении предмета ЕНТ")
	}

	log.Printf("[admin/subjects] Deleted: %s (%s)", id, existing.Name.RU)

	if isHTMXRequest(c) {
		return c.SendString("")
	}

	return h.redirectToListWithFlash(c, "success",
		fmt.Sprintf("Предмет «%s» удалён", existing.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Вспомогательные методы
// ────────────────────────────────────────────────────────────────────────────

// parseSubjectForm извлекает и валидирует данные формы предмета ЕНТ.
// Возвращает модель Subject и map ошибок.
func (h *SubjectHandlers) parseSubjectForm(c *fiber.Ctx) (models.Subject, map[string]string) {
	errs := make(map[string]string)

	nameRU := strings.TrimSpace(c.FormValue("name_ru"))
	nameKZ := strings.TrimSpace(c.FormValue("name_kz"))
	nameEN := strings.TrimSpace(c.FormValue("name_en"))

	// Валидация обязательных полей.
	if nameRU == "" {
		errs["name_ru"] = "Название на русском обязательно"
	}
	if nameKZ == "" {
		errs["name_kz"] = "Название на казахском обязательно"
	}
	if nameEN == "" {
		errs["name_en"] = "Название на английском обязательно"
	}

	subject := models.Subject{
		Name: models.LocalizedName{
			KZ: nameKZ,
			RU: nameRU,
			EN: nameEN,
		},
	}

	return subject, errs
}

// renderFormWithErrors рендерит форму с ошибками валидации.
func (h *SubjectHandlers) renderFormWithErrors(
	c *fiber.Ctx,
	isEdit bool,
	recordID string,
	subject models.Subject,
	errs map[string]string,
) error {
	title := "Новый предмет ЕНТ"
	if isEdit {
		title = fmt.Sprintf("Редактирование — %s", subject.Name.RU)
	}

	c.Status(fiber.StatusUnprocessableEntity)

	return h.renderer.RenderPage(c, "subjects/form.html", PageData{
		Title:     title,
		Admin:     h.adminData(c),
		ActiveNav: "subjects",
		Errors:    errs,
		Content: SubjectFormData{
			IsEdit:   isEdit,
			RecordID: recordID,
			Subject:  subject,
		},
	})
}

// adminData извлекает данные администратора из c.Locals.
func (h *SubjectHandlers) adminData(c *fiber.Ctx) AdminData {
	return adminDataFromLocals(c)
}

// redirectToListWithFlash выполняет редирект на список предметов ЕНТ с flash-сообщением.
func (h *SubjectHandlers) redirectToListWithFlash(c *fiber.Ctx, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/subjects?flash=%s&flash_msg=%s",
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}
