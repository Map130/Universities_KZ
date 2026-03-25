package web

import (
	"github.com/gofiber/fiber/v2"
	
	"github.com/Map130/universities/internal/models"
	"github.com/Map130/universities/internal/repository"
	"github.com/Map130/universities/internal/views"
)

type WebHandler struct {
	UniRepo repository.UniversityRepository
}

func RegisterWebRoutes(app *fiber.App, uniRepo repository.UniversityRepository) {
	h := &WebHandler{
		UniRepo: uniRepo,
	}

	app.Get("/", h.HomeHandler)
	app.Get("/universities", h.UniversitiesHandler)
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
