package web

import (
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
)

// Render рендерит templ.Component с помощью Fiber
func Render(c *fiber.Ctx, component templ.Component) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return component.Render(c.Context(), c.Response().BodyWriter())
}
