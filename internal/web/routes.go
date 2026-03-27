package web

import (
	"github.com/gofiber/fiber/v2"
	
	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/views"
)

type WebHandler struct {
	UniRepo  repository.UniversityRepository
	CalcRepo repository.CalculatorRepository
	SubjRepo repository.SubjectRepository
}

func RegisterWebRoutes(app *fiber.App, uniRepo repository.UniversityRepository, calcRepo repository.CalculatorRepository, subjRepo repository.SubjectRepository) {
	h := &WebHandler{
		UniRepo:  uniRepo,
		CalcRepo: calcRepo,
		SubjRepo: subjRepo,
	}

	app.Get("/", h.HomeHandler)
	app.Get("/universities", h.UniversitiesHandler)
	app.Get("/calculator", h.CalculatorHandler)
	app.Get("/calculator/results", h.CalculatorResultsHandler)
}

func (h *WebHandler) HomeHandler(c *fiber.Ctx) error {
	return Render(c, views.Home())
}

func (h *WebHandler) UniversitiesHandler(c *fiber.Ctx) error {
	f := models.UniversityFilters{
		Limit: 100,
	}
	unis, err := h.UniRepo.GetAll(c.Context(), f)
	if err != nil {
		return c.Status(500).SendString("Ошибка загрузки данных")
	}
	return Render(c, views.Universities(unis))
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

