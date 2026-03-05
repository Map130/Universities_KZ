//go:build noauth

// Package auth реализует Google OAuth 2.0 аутентификацию для админ-панели.
//
// Этот файл компилируется ТОЛЬКО при сборке с тегом noauth:
//
//	go build -tags noauth ./cmd/api
//
// В этом режиме аутентификация полностью отключена — все запросы к /admin/*
// проходят без проверки сессии, а в c.Locals подставляются фейковые данные
// администратора. Используется исключительно для локальной разработки.
package auth

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// IsNoAuth reports whether the application was built without authentication.
// When built with the `noauth` tag this returns true.
func IsNoAuth() bool { return true }

// NoAuthMiddleware возвращает Fiber-middleware, который пропускает все
// запросы без проверки аутентификации и подставляет фейковые данные
// администратора в c.Locals.
//
// Это позволяет хендлерам admin-панели работать без изменений —
// auth.GetAdminFromLocals(c) вернёт корректный AdminInfo.
//
// ⚠️  Доступен ТОЛЬКО в noauth-билде. В production-билде этой функции нет.
func NoAuthMiddleware() fiber.Handler {
	log.Println("[auth] ⚠️  WARNING: authentication is DISABLED (noauth build)")

	return func(c *fiber.Ctx) error {
		c.Locals("admin_email", "dev@localhost")
		c.Locals("admin_name", "Dev Admin")
		c.Locals("admin_avatar_url", "")
		return c.Next()
	}
}
