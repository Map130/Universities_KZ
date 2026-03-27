package web

import (
	"github.com/gofiber/fiber/v2"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
	
	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/views"
)

type WebHandler struct {
	UniRepo   repository.UniversityRepository
	CalcRepo  repository.CalculatorRepository
	SubjRepo  repository.SubjectRepository
	GroupRepo repository.SpecialtyGroupRepository
}

func RegisterWebRoutes(app *fiber.App, uniRepo repository.UniversityRepository, calcRepo repository.CalculatorRepository, subjRepo repository.SubjectRepository, groupRepo repository.SpecialtyGroupRepository) {
	h := &WebHandler{
		UniRepo:   uniRepo,
		CalcRepo:  calcRepo,
		SubjRepo:  subjRepo,
		GroupRepo: groupRepo,
	}

	app.Get("/", h.HomeHandler)
	app.Get("/universities", h.UniversitiesHandler)
	app.Get("/universities/:id", h.UniversityDetailHandler)
	app.Get("/groups", h.GroupsHandler)
	app.Get("/calculator", h.CalculatorHandler)
	app.Get("/calculator/results", h.CalculatorResultsHandler)
}

func (h *WebHandler) HomeHandler(c *fiber.Ctx) error {
	return Render(c, views.Home())
}

func (h *WebHandler) UniversitiesHandler(c *fiber.Ctx) error {
	f := models.UniversityFilters{
		Search: c.Query("search"),
		City:   c.Query("city"),
		Type:   models.UniversityType(c.Query("type")),
		Limit:  100,
	}
	
	unis, err := h.UniRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(500).SendString("Ошибка загрузки данных")
	}
	
	// If it's an HTMX request, return only the partial list
	if c.Get("HX-Request") != "" {
		return Render(c, views.UniversityList(unis))
	}
	
	return Render(c, views.Universities(unis))
}

func (h *WebHandler) UniversityDetailHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).SendString("ID не указан")
	}
	
	recordID := surrealmodels.NewRecordID("university", id)
	detail, err := h.UniRepo.GetWithSpecialties(c.Context(), recordID)
	if err != nil {
		return c.Status(404).SendString("Вуз не найден")
	}
	
	return Render(c, views.UniversityDetail(*detail))
}

func (h *WebHandler) GroupsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	
	limit := 20
	offset := (page - 1) * limit
	
	f := models.SpecialtyGroupFilters{
		Limit:  limit,
		Offset: offset,
	}
	
	groups, err := h.GroupRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(500).SendString("Ошибка загрузки групп ОП")
	}
	
	return Render(c, views.GroupsPage(groups, page))
}

func (h *WebHandler) CalculatorHandler(c *fiber.Ctx) error {
	subjects, err := h.SubjRepo.GetAll(c.Context())
	if err != nil {
		return c.Status(500).SendString("Ошибка загрузки предметов")
	}
	return Render(c, views.CalculatorPage(subjects))
}

func (h *WebHandler) CalculatorResultsHandler(c *fiber.Ctx) error {
	req := models.CalculatorRequest{
		Score:    c.QueryInt("score", 0),
		Subject1: c.Query("subject1"),
		Subject2: c.Query("subject2"),
		City:     c.Query("city"),
	}
	
	results, err := h.CalcRepo.Calculate(c.Context(), req)
	if err != nil {
		return c.Status(500).SendString("Ошибка расчета")
	}
	
	return Render(c, views.CalculatorResults(results))
}



