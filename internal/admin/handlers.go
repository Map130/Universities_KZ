// Package admin реализует HTTP-хендлеры админ-панели для управления вузами.
// Все хендлеры поддерживают два режима ответа:
//   - Полная страница (layout + content) — обычный HTTP-запрос.
//   - HTML-фрагмент — HTMX-запрос (заголовок HX-Request).
//
// Это обеспечивает Graceful Degradation: формы работают и без HTMX.
package admin

import (
	"fmt"
	"log"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/storage"
)

// ────────────────────────────────────────────────────────────────────────────
//  Handlers — набор HTTP-хендлеров для админки вузов
// ────────────────────────────────────────────────────────────────────────────

// Handlers объединяет зависимости для HTTP-хендлеров админ-панели.
type Handlers struct {
	uniRepo  repository.UniversityRepository
	specRepo repository.SpecialtyRepository
	store    *session.Store
	storage  storage.Uploader
	renderer *Renderer
}

// NewHandlers создаёт набор хендлеров для управления вузами.
//
//   - uniRepo:  репозиторий вузов (SurrealDB).
//   - store:    Fiber session store (для данных админа в layout).
//   - uploader: MinIO/S3 storage (для загрузки логотипов).
//   - renderer: движок шаблонов (html/template с layout).
func NewHandlers(
	uniRepo repository.UniversityRepository,
	specRepo repository.SpecialtyRepository,
	store *session.Store,
	uploader storage.Uploader,
	renderer *Renderer,
) *Handlers {
	return &Handlers{
		uniRepo:  uniRepo,
		specRepo: specRepo,
		store:    store,
		storage:  uploader,
		renderer: renderer,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Route Registration
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes регистрирует все маршруты админки вузов в Fiber router group.
//
// Предполагается, что group уже защищён middleware auth.AuthRequired.
//
// Маршруты:
//
//	GET    /admin/universities            → список вузов
//	GET    /admin/universities/new        → форма создания
//	POST   /admin/universities            → создание вуза
//	GET    /admin/universities/:id/edit   → форма редактирования
//	PUT    /admin/universities/:id        → обновление вуза
//	DELETE /admin/universities/:id        → удаление вуза
func (h *Handlers) RegisterRoutes(group fiber.Router) {
	group.Get("/", h.Index)
	group.Get("/new", h.New)
	group.Post("/", h.Create)
	group.Get("/:id/edit", h.Edit)
	group.Put("/:id", h.Update)
	group.Post("/:id", h.UpdatePost) // Graceful Degradation: POST + _method=PUT
	group.Delete("/:id", h.Delete)

	// ── Offers (связки специальностей) ───────────────────────
	group.Post("/:id/offers", h.AddOffer)
	group.Put("/:id/offers/:offer_id", h.UpdateOffer)
	group.Post("/:id/offers/:offer_id", h.UpdateOfferPost) // Graceful Degradation
	group.Delete("/:id/offers/:offer_id", h.DeleteOffer)
}

// ────────────────────────────────────────────────────────────────────────────
//  Index — список вузов
// ────────────────────────────────────────────────────────────────────────────

// UniversitiesListData передаётся в шаблон universities/index.html.
type UniversitiesListData struct {
	Universities []models.University
	Search       string
	TypeFilter   string
	CityFilter   string
}

// Index рендерит страницу со списком вузов.
//
//	GET /admin/universities
//	GET /admin/universities?search=...&type=...&city=...
//
// Поддерживает фильтрацию через query-параметры.
// Для HTMX-запроса (live search) — возвращает только tbody таблицы.
func (h *Handlers) Index(c *fiber.Ctx) error {
	// Парсим фильтры из query string.
	search := strings.TrimSpace(c.Query("search"))
	typeFilter := c.Query("type")
	cityFilter := strings.TrimSpace(c.Query("city"))

	filters := models.UniversityFilters{
		Search: search,
		Lang:   "ru",
		Limit:  100,
	}

	if typeFilter != "" {
		filters.Type = models.UniversityType(typeFilter)
	}
	if cityFilter != "" {
		filters.City = cityFilter
	}

	universities, err := h.uniRepo.GetAll(c.Context(), filters)
	if err != nil {
		log.Printf("[admin/universities] Index: GetAll error: %v", err)
		universities = []models.University{} // Показываем пустой список вместо 500.
	}

	// Flash-сообщения из query-параметров (после redirect из Create/Update/Delete).
	var flash *FlashMessage
	if flashType := c.Query("flash"); flashType != "" {
		flashMsg := c.Query("flash_msg")
		if flashMsg != "" {
			flash = &FlashMessage{
				Type:    flashType,
				Message: flashMsg,
			}
		}
	}

	return h.renderer.RenderPage(c, "universities/index.html", PageData{
		Title:     "Вузы",
		Admin:     h.adminData(c),
		ActiveNav: "universities",
		Flash:     flash,
		Content: UniversitiesListData{
			Universities: universities,
			Search:       search,
			TypeFilter:   typeFilter,
			CityFilter:   cityFilter,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  New — форма создания нового вуза
// ────────────────────────────────────────────────────────────────────────────

// UniversityFormData передаётся в шаблон universities/form.html.
type UniversityFormData struct {
	// IsEdit — true если это форма редактирования (PUT), false — создание (POST).
	IsEdit bool

	// RecordID — строковое представление ID вуза (для URL в action/hx-put).
	// Пустая строка для нового вуза.
	RecordID string

	// University — данные вуза (заполненные или пустые для новой записи).
	University models.University

	// AllSpecialties — все специальности для dropdown при добавлении offer.
	AllSpecialties []models.Specialty

	// Offers — текущие связи вуза со специальностями (offers edges).
	Offers []models.OfferWithSpecialty
}

// New рендерит пустую форму для создания нового вуза.
//
//	GET /admin/universities/new
func (h *Handlers) New(c *fiber.Ctx) error {
	return h.renderer.RenderPage(c, "universities/form.html", PageData{
		Title:     "Новый вуз",
		Admin:     h.adminData(c),
		ActiveNav: "universities",
		Content: UniversityFormData{
			IsEdit:         false,
			RecordID:       "",
			University:     models.University{},
			AllSpecialties: []models.Specialty{},
			Offers:         []models.OfferWithSpecialty{},
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Create — создание нового вуза
// ────────────────────────────────────────────────────────────────────────────

// Create обрабатывает POST-запрос на создание вуза.
//
//	POST /admin/universities  (multipart/form-data)
//
// Шаги:
//  1. Парсит form-данные (название, тип, описание, custom_css).
//  2. Валидирует обязательные поля.
//  3. Санитизирует custom_css через SanitizeCSS().
//  4. Если приложен файл логотипа — загружает его в MinIO.
//  5. Создаёт запись в SurrealDB.
//  6. Для HTMX — редиректит на список с toast-уведомлением.
//     Для обычного запроса — 302 Redirect.
func (h *Handlers) Create(c *fiber.Ctx) error {
	// 1. Парсим form-данные.
	uni, errs := h.parseUniversityForm(c)
	if len(errs) > 0 {
		// Валидация не прошла — возвращаем форму с ошибками.
		return h.renderFormWithErrors(c, false, "", uni, errs)
	}

	// 2. Загружаем логотип, если приложен.
	logoURL, logoErr := h.uploadLogo(c)
	if logoErr != nil {
		errs["logo"] = logoErr.Error()
		return h.renderFormWithErrors(c, false, "", uni, errs)
	}
	if logoURL != "" {
		uni.LogoURL = &logoURL
	}

	// 3. Создаём запись в базе данных.
	created, err := h.uniRepo.Create(c.Context(), uni)
	if err != nil {
		log.Printf("[admin/universities] Create: DB error: %v", err)
		errs["_global"] = "Ошибка при сохранении в базу данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, false, "", uni, errs)
	}

	log.Printf("[admin/universities] Created: %v (%s)", created.ID, created.Name.RU)

	// 4. Redirect на список вузов с flash-сообщением.
	return h.redirectToListWithFlash(c, "success", fmt.Sprintf("Вуз «%s» успешно создан", created.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  Edit — форма редактирования вуза
// ────────────────────────────────────────────────────────────────────────────

// Edit рендерит форму редактирования существующего вуза.
//
//	GET /admin/universities/:id/edit
func (h *Handlers) Edit(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("university", id)

	detail, err := h.uniRepo.GetWithSpecialties(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/universities] Edit: GetWithSpecialties(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Вуз не найден")
	}

	allSpecs := h.loadAllSpecialties(c)

	// Flash-сообщения из query-параметров (например, после добавления offer).
	var flash *FlashMessage
	if flashType := c.Query("flash"); flashType != "" {
		if flashMsg := c.Query("flash_msg"); flashMsg != "" {
			flash = &FlashMessage{
				Type:    flashType,
				Message: flashMsg,
			}
		}
	}

	return h.renderer.RenderPage(c, "universities/form.html", PageData{
		Title:     fmt.Sprintf("Редактирование — %s", detail.University.Name.RU),
		Admin:     h.adminData(c),
		ActiveNav: "universities",
		Flash:     flash,
		Content: UniversityFormData{
			IsEdit:         true,
			RecordID:       id,
			University:     detail.University,
			AllSpecialties: allSpecs,
			Offers:         detail.Offers,
		},
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  Update — обновление вуза
// ────────────────────────────────────────────────────────────────────────────

// Update обрабатывает PUT-запрос на обновление вуза.
//
//	PUT /admin/universities/:id  (multipart/form-data)
//
// Шаги аналогичны Create, но обновляет существующую запись.
func (h *Handlers) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("university", id)

	// Проверяем, что вуз существует.
	existing, err := h.uniRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/universities] Update: GetByID(%s) error: %v", id, err)
		return h.redirectToListWithFlash(c, "error", "Вуз не найден")
	}

	// Парсим form-данные.
	uni, errs := h.parseUniversityForm(c)
	if len(errs) > 0 {
		return h.renderFormWithErrors(c, true, id, uni, errs)
	}

	// Загружаем новый логотип, если приложен.
	logoURL, logoErr := h.uploadLogo(c)
	if logoErr != nil {
		errs["logo"] = logoErr.Error()
		return h.renderFormWithErrors(c, true, id, uni, errs)
	}

	if logoURL != "" {
		// Новый логотип загружен — удаляем старый из MinIO.
		if existing.LogoURL != nil && *existing.LogoURL != "" {
			oldFileName := path.Base(*existing.LogoURL)
			if delErr := h.storage.DeleteFile(c.Context(), storage.BucketLogos, oldFileName); delErr != nil {
				log.Printf("[admin/universities] Update: warning: failed to delete old logo %s: %v", oldFileName, delErr)
			}
		}
		uni.LogoURL = &logoURL
	} else {
		// Логотип не менялся — сохраняем текущий URL.
		uni.LogoURL = existing.LogoURL
	}

	// Обновляем запись в базе данных.
	updated, err := h.uniRepo.Update(c.Context(), recordID, uni)
	if err != nil {
		log.Printf("[admin/universities] Update: DB error: %v", err)
		errs["_global"] = "Ошибка при обновлении в базе данных. Попробуйте ещё раз."
		return h.renderFormWithErrors(c, true, id, uni, errs)
	}

	log.Printf("[admin/universities] Updated: %v (%s)", updated.ID, updated.Name.RU)

	return h.redirectToListWithFlash(c, "success", fmt.Sprintf("Вуз «%s» успешно обновлён", updated.Name.RU))
}

// UpdatePost обрабатывает POST-запрос с _method=PUT.
// Это Graceful Degradation: HTML-формы не поддерживают метод PUT,
// поэтому форма отправляется как POST с hidden-полем _method=PUT.
//
//	POST /admin/universities/:id
func (h *Handlers) UpdatePost(c *fiber.Ctx) error {
	method := strings.ToUpper(c.FormValue("_method"))
	if method == "PUT" {
		return h.Update(c)
	}
	// Если _method не PUT — трактуем как обычный POST (создание).
	return c.Status(fiber.StatusMethodNotAllowed).SendString("Method Not Allowed")
}

// ────────────────────────────────────────────────────────────────────────────
//  Delete — удаление вуза
// ────────────────────────────────────────────────────────────────────────────

// Delete удаляет вуз по ID.
//
//	DELETE /admin/universities/:id
//
// Для HTMX — возвращает пустой ответ (строка таблицы удаляется через hx-swap).
// Для обычного запроса — 302 Redirect на список.
func (h *Handlers) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	recordID := surrealmodels.NewRecordID("university", id)

	// Проверяем, что вуз существует, и получаем его для лога и удаления логотипа.
	existing, err := h.uniRepo.GetByID(c.Context(), recordID)
	if err != nil {
		log.Printf("[admin/universities] Delete: GetByID(%s) error: %v", id, err)
		if isHTMXRequest(c) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return h.redirectToListWithFlash(c, "error", "Вуз не найден")
	}

	// Удаляем логотип из MinIO.
	if existing.LogoURL != nil && *existing.LogoURL != "" {
		oldFileName := path.Base(*existing.LogoURL)
		if delErr := h.storage.DeleteFile(c.Context(), storage.BucketLogos, oldFileName); delErr != nil {
			log.Printf("[admin/universities] Delete: warning: failed to delete logo %s: %v", oldFileName, delErr)
		}
	}

	// Удаляем все связи offers перед удалением вуза.
	if err := h.uniRepo.DeleteAllOffers(c.Context(), recordID); err != nil {
		log.Printf("[admin/universities] Delete: DeleteAllOffers(%s) error: %v", id, err)
		// Продолжаем удаление вуза — ошибка не критична.
	}

	// Удаляем запись из базы данных.
	if err := h.uniRepo.Delete(c.Context(), recordID); err != nil {
		log.Printf("[admin/universities] Delete: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления")
		}
		return h.redirectToListWithFlash(c, "error", "Ошибка при удалении вуза")
	}

	log.Printf("[admin/universities] Deleted: %s (%s)", id, existing.Name.RU)

	// HTMX: возвращаем пустой ответ — hx-swap="outerHTML" удалит строку.
	if isHTMXRequest(c) {
		return c.SendString("")
	}

	return h.redirectToListWithFlash(c, "success", fmt.Sprintf("Вуз «%s» удалён", existing.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  AddOffer — добавление связи вуза со специальностью
// ────────────────────────────────────────────────────────────────────────────

// AddOffer создаёт графовую связь university -> specialty.
//
//	POST /admin/universities/:id/offers
//
// Form fields:
//   - specialty_id:       строковый ID специальности
//   - grant_count:        количество грантов
//   - quota_grant_count:  количество грантов по сельской квоте
//   - tuition_fee:        стоимость обучения (тенге/год)
//   - min_score:          минимальный балл ЕНТ
//   - last_year_threshold: проходной балл прошлого года
func (h *Handlers) AddOffer(c *fiber.Ctx) error {
	id := c.Params("id")
	uniRecordID := surrealmodels.NewRecordID("university", id)

	// Проверяем существование вуза.
	_, err := h.uniRepo.GetByID(c.Context(), uniRecordID)
	if err != nil {
		log.Printf("[admin/universities] AddOffer: university %s not found: %v", id, err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusNotFound).SendString("Вуз не найден")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Вуз не найден")
	}

	specIDStr := strings.TrimSpace(c.FormValue("specialty_id"))
	if specIDStr == "" {
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusUnprocessableEntity).SendString("Выберите специальность")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Выберите специальность")
	}

	offerInput, offerErrs := h.parseOfferForm(c)
	if len(offerErrs) > 0 {
		// Собираем первую ошибку для flash.
		var firstErr string
		for _, v := range offerErrs {
			firstErr = v
			break
		}
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusUnprocessableEntity).SendString(firstErr)
		}
		return h.redirectToEditWithFlash(c, id, "error", firstErr)
	}

	specRecordID := surrealmodels.NewRecordID("specialty", specIDStr)

	// Проверяем существование специальности.
	spec, err := h.specRepo.GetByID(c.Context(), specRecordID)
	if err != nil {
		log.Printf("[admin/universities] AddOffer: specialty %s not found: %v", specIDStr, err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusNotFound).SendString("Специальность не найдена")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Специальность не найдена")
	}

	_, err = h.uniRepo.CreateOffer(c.Context(), uniRecordID, specRecordID, offerInput)
	if err != nil {
		log.Printf("[admin/universities] AddOffer: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка при создании связи")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Ошибка при создании связи со специальностью")
	}

	log.Printf("[admin/universities] AddOffer: uni=%s spec=%s (%s)", id, specIDStr, spec.Name.RU)

	return h.redirectToEditWithFlash(c, id, "success",
		fmt.Sprintf("Специальность «%s — %s» привязана к вузу", spec.Code, spec.Name.RU))
}

// ────────────────────────────────────────────────────────────────────────────
//  UpdateOffer — обновление данных связи вуза со специальностью
// ────────────────────────────────────────────────────────────────────────────

// UpdateOffer обновляет данные графовой связи offers.
//
//	PUT /admin/universities/:id/offers/:offer_id
func (h *Handlers) UpdateOffer(c *fiber.Ctx) error {
	id := c.Params("id")
	offerID := c.Params("offer_id")
	offerRecordID := surrealmodels.NewRecordID("offers", offerID)

	offerInput, offerErrs := h.parseOfferForm(c)
	if len(offerErrs) > 0 {
		var firstErr string
		for _, v := range offerErrs {
			firstErr = v
			break
		}
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusUnprocessableEntity).SendString(firstErr)
		}
		return h.redirectToEditWithFlash(c, id, "error", firstErr)
	}

	_, err := h.uniRepo.UpdateOffer(c.Context(), offerRecordID, offerInput)
	if err != nil {
		log.Printf("[admin/universities] UpdateOffer: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка при обновлении связи")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Ошибка при обновлении данных специальности")
	}

	log.Printf("[admin/universities] UpdateOffer: uni=%s offer=%s updated", id, offerID)

	return h.redirectToEditWithFlash(c, id, "success", "Данные специальности обновлены")
}

// UpdateOfferPost обрабатывает POST с _method=PUT для Graceful Degradation.
//
//	POST /admin/universities/:id/offers/:offer_id
func (h *Handlers) UpdateOfferPost(c *fiber.Ctx) error {
	method := strings.ToUpper(c.FormValue("_method"))
	if method == "PUT" {
		return h.UpdateOffer(c)
	}
	return c.Status(fiber.StatusMethodNotAllowed).SendString("Method Not Allowed")
}

// ────────────────────────────────────────────────────────────────────────────
//  DeleteOffer — удаление связи вуза со специальностью
// ────────────────────────────────────────────────────────────────────────────

// DeleteOffer удаляет графовую связь offers.
//
//	DELETE /admin/universities/:id/offers/:offer_id
func (h *Handlers) DeleteOffer(c *fiber.Ctx) error {
	id := c.Params("id")
	offerID := c.Params("offer_id")
	offerRecordID := surrealmodels.NewRecordID("offers", offerID)

	if err := h.uniRepo.DeleteOffer(c.Context(), offerRecordID); err != nil {
		log.Printf("[admin/universities] DeleteOffer: DB error: %v", err)
		if isHTMXRequest(c) {
			return c.Status(fiber.StatusInternalServerError).SendString("Ошибка удаления связи")
		}
		return h.redirectToEditWithFlash(c, id, "error", "Ошибка при удалении связи со специальностью")
	}

	log.Printf("[admin/universities] DeleteOffer: uni=%s offer=%s deleted", id, offerID)

	if isHTMXRequest(c) {
		// Возвращаем пустую строку — HTMX удалит строку из таблицы (hx-swap="outerHTML").
		return c.SendString("")
	}

	return h.redirectToEditWithFlash(c, id, "success", "Связь со специальностью удалена")
}

// ────────────────────────────────────────────────────────────────────────────
//  Вспомогательные методы
// ────────────────────────────────────────────────────────────────────────────

// parseUniversityForm извлекает и валидирует данные формы вуза.
// Возвращает заполненную модель University и map ошибок валидации.
// Пустой map = валидация пройдена.
func (h *Handlers) parseUniversityForm(c *fiber.Ctx) (models.University, map[string]string) {
	errs := make(map[string]string)

	nameRU := strings.TrimSpace(c.FormValue("name_ru"))
	nameKZ := strings.TrimSpace(c.FormValue("name_kz"))
	nameEN := strings.TrimSpace(c.FormValue("name_en"))
	abbr := strings.TrimSpace(c.FormValue("abbr"))
	city := strings.TrimSpace(c.FormValue("city"))
	uniType := strings.TrimSpace(c.FormValue("type"))
	website := strings.TrimSpace(c.FormValue("website"))
	description := strings.TrimSpace(c.FormValue("description"))
	customCSS := c.FormValue("custom_css") // Не тримим — сохраняем форматирование.

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
	if abbr == "" {
		errs["abbr"] = "Аббревиатура обязательна"
	}
	if city == "" {
		errs["city"] = "Город обязателен"
	}

	uniTypeModel := models.UniversityType(uniType)
	if !uniTypeModel.IsValid() {
		errs["type"] = "Выберите тип вуза (Государственный или Частный)"
	}

	// Санитизация кастомного CSS.
	//
	// БЕЗОПАСНОСТЬ: SanitizeCSS удаляет опасные конструкции:
	//   - @import, @charset, @namespace (внешние ресурсы)
	//   - url() (data exfiltration через background-image и т.п.)
	//   - expression(), -moz-binding (JS-инъекция в CSS)
	//   - javascript:, data: URI-схемы
	//
	// Подробности — см. комментарии к SanitizeCSS() в render.go.
	sanitizedCSS := SanitizeCSS(customCSS)

	uni := models.University{
		Name: models.LocalizedName{
			KZ: nameKZ,
			RU: nameRU,
			EN: nameEN,
		},
		Abbr:      abbr,
		City:      city,
		Type:      uniTypeModel,
		CustomCSS: sanitizedCSS,
	}

	if website != "" {
		uni.Website = &website
	}
	if description != "" {
		uni.Description = &description
	}

	return uni, errs
}

// uploadLogo пытается загрузить файл логотипа из multipart-формы.
// Возвращает:
//   - (url, nil) — логотип загружен, url = публичный URL.
//   - ("", nil)  — логотип не приложен (файл пустой), это нормально.
//   - ("", err)  — ошибка загрузки.
func (h *Handlers) uploadLogo(c *fiber.Ctx) (string, error) {
	file, err := c.FormFile("logo")
	if err != nil {
		// Файл не приложен — это не ошибка, логотип необязателен.
		return "", nil
	}

	// Проверяем, что файл не пустой (некоторые браузеры отправляют пустой файл).
	if file.Size == 0 {
		return "", nil
	}

	// Загружаем в MinIO. Валидация MIME-типа и размера — внутри UploadImage.
	logoURL, err := h.storage.UploadImage(c.Context(), file)
	if err != nil {
		log.Printf("[admin/universities] uploadLogo: error: %v", err)
		return "", fmt.Errorf("Ошибка загрузки логотипа: %v", err)
	}

	return logoURL, nil
}

// renderFormWithErrors рендерит форму создания/редактирования с ошибками валидации.
func (h *Handlers) renderFormWithErrors(
	c *fiber.Ctx,
	isEdit bool,
	recordID string,
	uni models.University,
	errs map[string]string,
) error {
	title := "Новый вуз"
	if isEdit {
		title = fmt.Sprintf("Редактирование — %s", uni.Name.RU)
	}

	allSpecs := h.loadAllSpecialties(c)

	// Загружаем текущие offers (если редактирование).
	var offers []models.OfferWithSpecialty
	if isEdit && recordID != "" {
		uniRecordID := surrealmodels.NewRecordID("university", recordID)
		detail, err := h.uniRepo.GetWithSpecialties(c.Context(), uniRecordID)
		if err != nil {
			log.Printf("[admin/universities] renderFormWithErrors: GetWithSpecialties error: %v", err)
			offers = []models.OfferWithSpecialty{}
		} else {
			offers = detail.Offers
		}
	}

	// Устанавливаем HTTP 422 (Unprocessable Entity) для ошибок валидации.
	// HTMX по умолчанию не свопает контент при 4xx/5xx ошибках,
	// но мы настроили hx-target, поэтому ответ корректно отобразится.
	c.Status(fiber.StatusUnprocessableEntity)

	return h.renderer.RenderPage(c, "universities/form.html", PageData{
		Title:     title,
		Admin:     h.adminData(c),
		ActiveNav: "universities",
		Errors:    errs,
		Content: UniversityFormData{
			IsEdit:         isEdit,
			RecordID:       recordID,
			University:     uni,
			AllSpecialties: allSpecs,
			Offers:         offers,
		},
	})
}

// adminData извлекает данные администратора из c.Locals (заполнены middleware).
func (h *Handlers) adminData(c *fiber.Ctx) AdminData {
	return adminDataFromLocals(c)
}

// redirectToListWithFlash выполняет редирект на список вузов.
// Для HTMX — через HX-Redirect (полная навигация).
// Для обычного запроса — стандартный HTTP 302.
//
// Flash-сообщения передаются через query-параметры, т.к. мы не используем
// серверные flash (cookie-based flash усложнил бы код без явной выгоды).
func (h *Handlers) redirectToListWithFlash(c *fiber.Ctx, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/universities?flash=%s&flash_msg=%s",
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}

// redirectToEditWithFlash выполняет редирект на форму редактирования вуза с flash-сообщением.
func (h *Handlers) redirectToEditWithFlash(c *fiber.Ctx, id, flashType, message string) error {
	redirectURL := fmt.Sprintf("/admin/universities/%s/edit?flash=%s&flash_msg=%s",
		id,
		url.QueryEscape(flashType),
		url.QueryEscape(message),
	)
	return HTMXRedirect(c, redirectURL)
}

// loadAllSpecialties загружает все специальности (для select-dropdown).
func (h *Handlers) loadAllSpecialties(c *fiber.Ctx) []models.Specialty {
	specs, err := h.specRepo.GetAll(c.Context(), models.SpecialtyFilters{
		Lang:  "ru",
		Limit: 500,
	})
	if err != nil {
		log.Printf("[admin/universities] loadAllSpecialties: GetAll error: %v", err)
		return []models.Specialty{}
	}
	return specs
}

// parseOfferForm извлекает и валидирует данные формы offer.
// Возвращает CreateOfferInput и map ошибок.
func (h *Handlers) parseOfferForm(c *fiber.Ctx) (models.CreateOfferInput, map[string]string) {
	errs := make(map[string]string)

	grantCountStr := strings.TrimSpace(c.FormValue("grant_count"))
	quotaGrantCountStr := strings.TrimSpace(c.FormValue("quota_grant_count"))
	tuitionFeeStr := strings.TrimSpace(c.FormValue("tuition_fee"))
	minScoreStr := strings.TrimSpace(c.FormValue("min_score"))
	lastYearThresholdStr := strings.TrimSpace(c.FormValue("last_year_threshold"))

	var input models.CreateOfferInput

	if grantCountStr == "" {
		grantCountStr = "0"
	}
	grantCount, err := strconv.Atoi(grantCountStr)
	if err != nil || grantCount < 0 {
		errs["grant_count"] = "Количество грантов должно быть неотрицательным целым числом"
	} else {
		input.GrantCount = grantCount
	}

	if quotaGrantCountStr == "" {
		quotaGrantCountStr = "0"
	}
	quotaGrantCount, err := strconv.Atoi(quotaGrantCountStr)
	if err != nil || quotaGrantCount < 0 {
		errs["quota_grant_count"] = "Количество грантов по квоте должно быть неотрицательным целым числом"
	} else {
		input.QuotaGrantCount = quotaGrantCount
	}

	if tuitionFeeStr == "" {
		tuitionFeeStr = "0"
	}
	tuitionFee, err := strconv.Atoi(tuitionFeeStr)
	if err != nil || tuitionFee < 0 {
		errs["tuition_fee"] = "Стоимость обучения должна быть неотрицательным целым числом"
	} else {
		input.TuitionFee = tuitionFee
	}

	if minScoreStr == "" {
		minScoreStr = "0"
	}
	minScore, err := strconv.Atoi(minScoreStr)
	if err != nil || minScore < 0 || minScore > 140 {
		errs["min_score"] = "Минимальный балл ЕНТ должен быть от 0 до 140"
	} else {
		input.MinScore = minScore
	}

	if lastYearThresholdStr == "" {
		lastYearThresholdStr = "0"
	}
	lastYearThreshold, err := strconv.Atoi(lastYearThresholdStr)
	if err != nil || lastYearThreshold < 0 || lastYearThreshold > 140 {
		errs["last_year_threshold"] = "Проходной балл прошлого года должен быть от 0 до 140"
	} else {
		input.LastYearThreshold = lastYearThreshold
	}

	return input, errs
}
