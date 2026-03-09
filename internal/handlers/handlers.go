// Package handlers содержит HTTP-хендлеры с Swagger-аннотациями.
package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
)

// Handler объединяет все зависимости для HTTP-хендлеров.
type Handler struct {
	UniRepo     repository.UniversityRepository
	GroupRepo   repository.SpecialtyGroupRepository
	SpecRepo    repository.SpecialtyRepository
	SubjectRepo repository.SubjectRepository
}

// NewHandler создаёт Handler с заданными зависимостями.
func NewHandler(
	uniRepo repository.UniversityRepository,
	groupRepo repository.SpecialtyGroupRepository,
	specRepo repository.SpecialtyRepository,
	subjectRepo repository.SubjectRepository,
) *Handler {
	return &Handler{
		UniRepo:     uniRepo,
		GroupRepo:   groupRepo,
		SpecRepo:    specRepo,
		SubjectRepo: subjectRepo,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Health
// ────────────────────────────────────────────────────────────────────────────

// HealthCheck godoc
// @Summary      Проверка состояния сервиса
// @Description  Возвращает статус сервиса и подключения к БД
// @Tags         health
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Router       /health [get]
func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "online", "db": "connected"})
}

// ────────────────────────────────────────────────────────────────────────────
//  Universities
// ────────────────────────────────────────────────────────────────────────────

// GetUniversities godoc
// @Summary      Получить список вузов
// @Description  Возвращает список университетов Казахстана с поддержкой фильтрации по городу, типу, полнотекстовому поиску и пагинации
// @Tags         universities
// @Produce      json
// @Param        city    query     string  false  "Фильтр по городу (точное совпадение)"          example(Алматы)
// @Param        type    query     string  false  "Тип вуза"                                       Enums(public, private)
// @Param        search  query     string  false  "Полнотекстовый поиск по названию"               example(МУИТ)
// @Param        lang    query     string  false  "Язык поиска"                                    Enums(kz, ru, en)  default(ru)
// @Param        limit   query     int     false  "Максимальное число записей"                     default(50)
// @Param        offset  query     int     false  "Смещение для пагинации"                         default(0)
// @Success      200     {array}   models.SwaggerUniversity
// @Failure      500     {object}  models.ErrorResponse
// @Router       /api/v1/universities [get]
func (h *Handler) GetUniversities(c *fiber.Ctx) error {
	f := models.UniversityFilters{
		City:   c.Query("city"),
		Type:   models.UniversityType(c.Query("type")),
		Search: c.Query("search"),
		Lang:   c.Query("lang"),
		Limit:  c.QueryInt("limit", 50),
		Offset: c.QueryInt("offset", 0),
	}

	unis, err := h.UniRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(unis)
}

// GetUniversityByID godoc
// @Summary      Получить вуз по ID
// @Description  Возвращает детальную информацию о вузе, включая список предлагаемых специальностей (через графовую связь offers)
// @Tags         universities
// @Produce      json
// @Param        id   path      string  true  "ID вуза (без префикса таблицы)"  example(abc123)
// @Success      200  {object}  models.SwaggerUniversityDetail
// @Failure      400  {object}  models.ErrorResponse
// @Failure      404  {object}  models.ErrorResponse
// @Router       /api/v1/universities/{id} [get]
func (h *Handler) GetUniversityByID(c *fiber.Ctx) error {
	id, err := parseRecordID("university", c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	detail, err := h.UniRepo.GetWithSpecialties(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}

// ────────────────────────────────────────────────────────────────────────────
//  Specialty Groups
// ────────────────────────────────────────────────────────────────────────────

// GetGroups godoc
// @Summary      Получить список групп образовательных программ
// @Description  Возвращает список групп ОП с поддержкой полнотекстового поиска и пагинации. К группе привязаны требования к предметам ЕНТ.
// @Tags         groups
// @Produce      json
// @Param        search  query     string  false  "Полнотекстовый поиск по названию"  example(Информационные технологии)
// @Param        lang    query     string  false  "Язык поиска"                        Enums(kz, ru, en)  default(ru)
// @Param        limit   query     int     false  "Максимальное число записей"         default(100)
// @Param        offset  query     int     false  "Смещение для пагинации"             default(0)
// @Success      200     {array}   models.SwaggerSpecialtyGroup
// @Failure      500     {object}  models.ErrorResponse
// @Router       /api/v1/groups [get]
func (h *Handler) GetGroups(c *fiber.Ctx) error {
	f := models.SpecialtyGroupFilters{
		Search: c.Query("search"),
		Lang:   c.Query("lang"),
		Limit:  c.QueryInt("limit", 100),
		Offset: c.QueryInt("offset", 0),
	}

	groups, err := h.GroupRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(groups)
}

// GetGroupByID godoc
// @Summary      Получить группу ОП по ID
// @Description  Возвращает группу образовательных программ вместе с привязанными предметами ЕНТ (через графовую связь requires)
// @Tags         groups
// @Produce      json
// @Param        id   path      string  true  "ID группы ОП (без префикса таблицы)"  example(b057)
// @Success      200  {object}  models.SwaggerGroupWithSubjects
// @Failure      400  {object}  models.ErrorResponse
// @Failure      404  {object}  models.ErrorResponse
// @Router       /api/v1/groups/{id} [get]
func (h *Handler) GetGroupByID(c *fiber.Ctx) error {
	id, err := parseRecordID("specialty_group", c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	group, subjects, err := h.GroupRepo.GetWithSubjects(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"group": group, "subjects": subjects})
}

// ────────────────────────────────────────────────────────────────────────────
//  Specialties
// ────────────────────────────────────────────────────────────────────────────

// GetSpecialties godoc
// @Summary      Получить список специальностей
// @Description  Возвращает список образовательных программ с поддержкой фильтрации по коду группы, полнотекстовому поиску и пагинации
// @Tags         specialties
// @Produce      json
// @Param        search      query     string  false  "Полнотекстовый поиск по названию"  example(Программная инженерия)
// @Param        lang        query     string  false  "Язык поиска"                        Enums(kz, ru, en)  default(ru)
// @Param        group_code  query     string  false  "Фильтр по коду группы ОП"           example(B057)
// @Param        limit       query     int     false  "Максимальное число записей"         default(50)
// @Param        offset      query     int     false  "Смещение для пагинации"             default(0)
// @Success      200         {array}   models.SwaggerSpecialty
// @Failure      500         {object}  models.ErrorResponse
// @Router       /api/v1/specialties [get]
func (h *Handler) GetSpecialties(c *fiber.Ctx) error {
	f := models.SpecialtyFilters{
		Search:    c.Query("search"),
		Lang:      c.Query("lang"),
		GroupCode: c.Query("group_code"),
		Limit:     c.QueryInt("limit", 50),
		Offset:    c.QueryInt("offset", 0),
	}

	specs, err := h.SpecRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(specs)
}

// ────────────────────────────────────────────────────────────────────────────
//  Subjects
// ────────────────────────────────────────────────────────────────────────────

// GetSubjects godoc
// @Summary      Получить список предметов ЕНТ
// @Description  Возвращает полный список предметов Единого национального тестирования
// @Tags         subjects
// @Produce      json
// @Success      200  {array}   models.SwaggerSubject
// @Failure      500  {object}  models.ErrorResponse
// @Router       /api/v1/subjects [get]
func (h *Handler) GetSubjects(c *fiber.Ctx) error {
	subjects, err := h.SubjectRepo.GetAll(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(subjects)
}

// ────────────────────────────────────────────────────────────────────────────
//  Helpers
// ────────────────────────────────────────────────────────────────────────────

func parseRecordID(table, id string) (surrealmodels.RecordID, error) {
	if id == "" {
		return surrealmodels.RecordID{}, fmt.Errorf("empty record ID")
	}
	return surrealmodels.NewRecordID(table, id), nil
}
