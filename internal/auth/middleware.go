// Package auth реализует Google OAuth 2.0 аутентификацию для админ-панели.
package auth

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

// ────────────────────────────────────────────────────────────────────────────
//  Middleware: AuthRequired
// ────────────────────────────────────────────────────────────────────────────

// AuthRequired возвращает Fiber-middleware, который проверяет наличие
// активной admin-сессии перед допуском к защищённым роутам.
//
// Проверка выполняется исключительно по данным сессии (cookie) —
// middleware НЕ обращается ни к Google, ни к SurrealDB на каждый запрос.
// Whitelist-проверка происходит один раз, при OAuth callback.
//
// HTMX support:
//
//	Когда сессия истекла и запрос пришёл от HTMX (заголовок HX-Request),
//	middleware не может выполнить стандартный HTTP-редирект (302), потому что
//	HTMX перехватывает ответ и пытается вставить его в DOM.
//	Вместо этого middleware возвращает 200 OK с заголовком HX-Redirect,
//	который заставляет HTMX выполнить полную навигацию на страницу логина.
//
// Использование:
//
//	admin := app.Group("/admin", auth.AuthRequired(store))
//	admin.Get("/", adminDashboardHandler)
//	admin.Get("/universities", adminUniversitiesHandler)
func AuthRequired(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Получаем сессию из cookie.
		sess, err := store.Get(c)
		if err != nil {
			log.Printf("[auth] middleware: session get error: %v", err)
			return redirectToLogin(c)
		}

		// 2. Проверяем, есть ли email администратора в сессии.
		//    Если сессия истекла, Fiber session store вернёт пустую сессию,
		//    и GetAdminEmail вернёт "".
		if !IsAuthenticated(sess) {
			log.Printf("[auth] middleware: unauthenticated request to %s", c.Path())
			return redirectToLogin(c)
		}

		// 3. Сохраняем email в Locals для использования в хендлерах
		//    без повторного чтения сессии.
		email := GetAdminEmail(sess)
		c.Locals("admin_email", email)
		c.Locals("admin_name", GetAdminName(sess))
		c.Locals("admin_avatar_url", GetAdminAvatarURL(sess))

		// 4. Пропускаем запрос дальше по цепочке middleware → handler.
		return c.Next()
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  AuthOptional — мягкая проверка (не блокирует, только заполняет Locals)
// ────────────────────────────────────────────────────────────────────────────

// AuthOptional возвращает middleware, который заполняет c.Locals данными
// администратора из сессии, но НЕ блокирует запрос, если сессия отсутствует.
//
// Полезно для страниц, которые доступны всем, но отображают дополнительный
// контент для залогиненных администраторов (например, кнопку «Админка»).
func AuthOptional(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			// Сессия недоступна — просто пропускаем дальше без данных.
			return c.Next()
		}

		if IsAuthenticated(sess) {
			c.Locals("admin_email", GetAdminEmail(sess))
			c.Locals("admin_name", GetAdminName(sess))
			c.Locals("admin_avatar_url", GetAdminAvatarURL(sess))
		}

		return c.Next()
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  Helper: redirectToLogin
// ────────────────────────────────────────────────────────────────────────────

// redirectToLogin перенаправляет пользователя на страницу входа.
//
// Обрабатывает два сценария:
//
//  1. Обычный HTTP-запрос (браузер) — стандартный 302 Redirect.
//
//  2. HTMX-запрос (заголовок HX-Request присутствует) — HTMX перехватывает
//     HTTP-редиректы и пытается подставить тело ответа в DOM, что ломает
//     навигацию. Поэтому для HTMX мы возвращаем 200 OK с заголовком
//     HX-Redirect, который сообщает HTMX выполнить полную навигацию
//     (window.location) на указанный URL.
//
// Ссылка: https://htmx.org/reference/#response_headers
func redirectToLogin(c *fiber.Ctx) error {
	if isHTMXRequest(c) {
		// HTMX: нельзя использовать стандартный HTTP 302, потому что
		// HTMX follow'ит редирект и вставляет HTML логин-страницы
		// внутрь текущего элемента. HX-Redirect заставляет HTMX
		// выполнить полную навигацию (как обычная ссылка).
		c.Set("HX-Redirect", loginPath)
		// Возвращаем 200 — HTMX обработает заголовок HX-Redirect
		// и выполнит навигацию до того, как попытается обработать body.
		return c.SendStatus(fiber.StatusOK)
	}

	// Обычный запрос — стандартный HTTP-редирект.
	return c.Redirect(loginPath, fiber.StatusFound)
}

// ────────────────────────────────────────────────────────────────────────────
//  Helper: GetAdminFromLocals
// ────────────────────────────────────────────────────────────────────────────

// AdminInfo содержит данные администратора, извлечённые из c.Locals.
// Используется хендлерами для доступа к данным текущего администратора
// без необходимости повторно читать сессию.
type AdminInfo struct {
	// Email — email администратора (из Google).
	Email string

	// Name — отображаемое имя (из Google-профиля).
	Name string

	// AvatarURL — URL аватарки (из Google-профиля).
	AvatarURL string
}

// GetAdminFromLocals извлекает данные администратора из c.Locals,
// заполненные middleware AuthRequired.
//
// Возвращает nil, если данные отсутствуют (middleware не был применён
// или пользователь не аутентифицирован).
//
// Пример использования в хендлере:
//
//	func dashboardHandler(c *fiber.Ctx) error {
//	    admin := auth.GetAdminFromLocals(c)
//	    if admin == nil {
//	        return c.SendStatus(fiber.StatusUnauthorized)
//	    }
//	    return c.JSON(fiber.Map{"admin": admin.Email})
//	}
func GetAdminFromLocals(c *fiber.Ctx) *AdminInfo {
	emailVal := c.Locals("admin_email")
	if emailVal == nil {
		return nil
	}

	email, ok := emailVal.(string)
	if !ok || email == "" {
		return nil
	}

	info := &AdminInfo{
		Email: email,
	}

	if nameVal := c.Locals("admin_name"); nameVal != nil {
		if name, ok := nameVal.(string); ok {
			info.Name = name
		}
	}

	if avatarVal := c.Locals("admin_avatar_url"); avatarVal != nil {
		if avatar, ok := avatarVal.(string); ok {
			info.AvatarURL = avatar
		}
	}

	return info
}
