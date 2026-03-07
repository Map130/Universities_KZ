// Package auth реализует Google OAuth 2.0 аутентификацию для админ-панели.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/markbates/goth"

	"github.com/Map130/universities/internal/repository"
)

// ────────────────────────────────────────────────────────────────────────────
//  Константы
// ────────────────────────────────────────────────────────────────────────────

const (
	// providerName — имя goth-провайдера для Google.
	providerName = "google"

	// sessionKeyOAuthState — ключ для хранения CSRF state в сессии
	// во время OAuth-флоу (между /auth/google и /auth/google/callback).
	sessionKeyOAuthState = "oauth_state"

	// sessionKeyOAuthSession — ключ для сериализованной goth.Session,
	// необходимой для завершения OAuth-флоу в callback-хендлере.
	sessionKeyOAuthSession = "oauth_session"

	// defaultLoginRedirect — URL, на который перенаправляется
	// администратор после успешного входа.
	defaultLoginRedirect = "/admin"

	// defaultLogoutRedirect — URL после выхода.
	defaultLogoutRedirect = "/"

	// loginPath — путь к странице логина (для редиректов при 403).
	loginPath = "/auth/google"
)

// ────────────────────────────────────────────────────────────────────────────
//  Handlers
// ────────────────────────────────────────────────────────────────────────────

// Handlers объединяет HTTP-хендлеры для OAuth-флоу.
// Зависимости (session store, admin repo) инжектируются через конструктор.
type Handlers struct {
	store     *session.Store
	adminRepo repository.AdminRepository
}

// NewHandlers создаёт набор OAuth-хендлеров.
//
//   - store — Fiber session store (единый для всего приложения).
//   - adminRepo — репозиторий для проверки whitelist (allowed_admins).
func NewHandlers(store *session.Store, adminRepo repository.AdminRepository) *Handlers {
	return &Handlers{
		store:     store,
		adminRepo: adminRepo,
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  GET /auth/google — начало OAuth-флоу
// ────────────────────────────────────────────────────────────────────────────

// BeginAuth инициирует Google OAuth 2.0 flow.
//
// Workflow:
//  1. Получает Google-провайдер из goth.
//  2. Генерирует случайный state (CSRF-защита).
//  3. Вызывает provider.BeginAuth(state) для получения URL авторизации.
//  4. Сохраняет state и сериализованную goth-сессию в Fiber-сессию.
//  5. Перенаправляет пользователя на страницу Google для входа.
func (h *Handlers) BeginAuth(c *fiber.Ctx) error {
	// 1. Получаем зарегистрированный Google-провайдер.
	provider, err := goth.GetProvider(providerName)
	if err != nil {
		log.Printf("[auth] failed to get provider %q: %v", providerName, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "OAuth provider not configured",
		})
	}

	// 2. Генерируем криптографически стойкий state.
	state, err := generateState()
	if err != nil {
		log.Printf("[auth] failed to generate state: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal error",
		})
	}

	// 3. Начинаем OAuth-флоу через goth.
	gothSession, err := provider.BeginAuth(state)
	if err != nil {
		log.Printf("[auth] BeginAuth failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to start OAuth flow",
		})
	}

	// 4. Получаем URL для редиректа на Google.
	authURL, err := gothSession.GetAuthURL()
	if err != nil {
		log.Printf("[auth] GetAuthURL failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to get auth URL",
		})
	}

	// 5. Сериализуем goth-сессию для последующего использования в callback.
	gothSessionStr := gothSession.Marshal()

	// 6. Сохраняем state и goth-сессию в Fiber-сессию.
	sess, err := h.store.Get(c)
	if err != nil {
		log.Printf("[auth] session get error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "session error",
		})
	}

	sess.Set(sessionKeyOAuthState, state)
	sess.Set(sessionKeyOAuthSession, gothSessionStr)

	if err := sess.Save(); err != nil {
		log.Printf("[auth] session save error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "session error",
		})
	}

	log.Printf("[auth] redirecting to Google OAuth (state=%s...)", state[:8])
	return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
}

// ────────────────────────────────────────────────────────────────────────────
//  GET /auth/google/callback — обработка ответа от Google
// ────────────────────────────────────────────────────────────────────────────

// Callback обрабатывает редирект от Google после аутентификации.
//
// Workflow:
//  1. Проверяет state-параметр (CSRF-защита).
//  2. Восстанавливает goth-сессию из Fiber-сессии.
//  3. Авторизует пользователя через goth (обмен code → token).
//  4. Получает данные пользователя (email, name, avatar).
//  5. Проверяет email по whitelist в SurrealDB (таблица allowed_admins).
//  6. При успехе — записывает данные в сессию и редиректит на /admin.
//  7. При неудаче — 403 Forbidden.
func (h *Handlers) Callback(c *fiber.Ctx) error {
	// 1. Получаем Fiber-сессию.
	sess, err := h.store.Get(c)
	if err != nil {
		log.Printf("[auth] callback: session get error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "session error",
		})
	}

	// 2. Проверяем CSRF state.
	savedState, _ := sess.Get(sessionKeyOAuthState).(string)
	queryState := c.Query("state")

	if savedState == "" || queryState == "" || savedState != queryState {
		log.Printf("[auth] callback: state mismatch (saved=%q, query=%q)", savedState, queryState)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "invalid OAuth state — possible CSRF attack",
		})
	}

	// 3. Восстанавливаем goth-сессию.
	gothSessionStr, _ := sess.Get(sessionKeyOAuthSession).(string)
	if gothSessionStr == "" {
		log.Printf("[auth] callback: goth session not found in fiber session")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "OAuth session expired, please try again",
		})
	}

	// 4. Получаем провайдер и восстанавливаем goth.Session.
	provider, err := goth.GetProvider(providerName)
	if err != nil {
		log.Printf("[auth] callback: provider error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "OAuth provider not configured",
		})
	}

	gothSession, err := provider.UnmarshalSession(gothSessionStr)
	if err != nil {
		log.Printf("[auth] callback: unmarshal session error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to restore OAuth session",
		})
	}

	// 5. Авторизуем: обмениваем code на access_token.
	code := c.Query("code")
	if code == "" {
		// Google вернул ошибку вместо code.
		oauthErr := c.Query("error")
		log.Printf("[auth] callback: no code from Google, error=%q", oauthErr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Google denied access: %s", oauthErr),
		})
	}

	// goth.Params — это url.Values; Authorize ожидает параметр "code".
	_, err = gothSession.Authorize(provider, urlParams{"code": {code}})
	if err != nil {
		log.Printf("[auth] callback: authorize error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to authorize with Google",
		})
	}

	// 6. Получаем данные пользователя от Google.
	user, err := provider.FetchUser(gothSession)
	if err != nil {
		log.Printf("[auth] callback: fetch user error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch user data from Google",
		})
	}

	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email == "" {
		log.Printf("[auth] callback: empty email from Google")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Google did not return an email address",
		})
	}

	log.Printf("[auth] callback: Google user email=%s name=%q", email, user.Name)

	// 7. Проверяем whitelist: есть ли email в таблице allowed_admins.
	allowed, err := h.adminRepo.IsAllowed(c.Context(), email)
	if err != nil {
		log.Printf("[auth] callback: whitelist check error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "authorization check failed",
		})
	}

	if !allowed {
		log.Printf("[auth] callback: DENIED — email %s not in allowed_admins", email)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied. Your email is not in the admin whitelist.",
		})
	}

	// 8. Очищаем временные OAuth-данные из сессии.
	sess.Delete(sessionKeyOAuthState)
	sess.Delete(sessionKeyOAuthSession)

	// 9. Записываем данные администратора в сессию.
	SetAdminSession(sess, email, user.Name, user.AvatarURL)

	if err := sess.Save(); err != nil {
		log.Printf("[auth] callback: session save error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "session error",
		})
	}

	log.Printf("[auth] callback: SUCCESS — admin %s logged in", email)
	return c.Redirect(defaultLoginRedirect, fiber.StatusTemporaryRedirect)
}

// ────────────────────────────────────────────────────────────────────────────
//  POST /auth/logout — выход
// ────────────────────────────────────────────────────────────────────────────

// Logout уничтожает сессию администратора и перенаправляет на главную.
//
// Поддерживает HTMX: если запрос пришёл от HTMX (заголовок HX-Request),
// вместо HTTP-редиректа возвращает заголовок HX-Redirect, чтобы HTMX
// выполнил полную навигацию.
func (h *Handlers) Logout(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		log.Printf("[auth] logout: session get error: %v", err)
		return c.Redirect(defaultLogoutRedirect, fiber.StatusTemporaryRedirect)
	}

	email := GetAdminEmail(sess)

	if err := ClearAdminSession(sess); err != nil {
		log.Printf("[auth] logout: session destroy error: %v", err)
	}

	log.Printf("[auth] logout: admin %s logged out", email)

	// HTMX support: если запрос от HTMX, отправляем HX-Redirect.
	if isHTMXRequest(c) {
		c.Set("HX-Redirect", defaultLogoutRedirect)
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect(defaultLogoutRedirect, fiber.StatusTemporaryRedirect)
}

// ────────────────────────────────────────────────────────────────────────────
//  GET /auth/me — информация о текущем администраторе
// ────────────────────────────────────────────────────────────────────────────

// Me возвращает JSON с данными текущего аутентифицированного администратора.
// Полезно для фронтенда (отображение имени, аватарки).
// Если пользователь не аутентифицирован — 401.
func (h *Handlers) Me(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "not authenticated",
		})
	}

	email := GetAdminEmail(sess)
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "not authenticated",
		})
	}

	return c.JSON(fiber.Map{
		"email":      email,
		"name":       GetAdminName(sess),
		"avatar_url": GetAdminAvatarURL(sess),
	})
}

// ────────────────────────────────────────────────────────────────────────────
//  RegisterRoutes — регистрирует все auth-роуты в Fiber-приложении
// ────────────────────────────────────────────────────────────────────────────

// RegisterRoutes добавляет OAuth-роуты в Fiber app:
//
//   - GET  /auth/google          — начало OAuth-флоу
//   - GET  /auth/google/callback — callback от Google
//   - POST /auth/logout          — выход (уничтожение сессии)
//   - GET  /auth/me              — данные текущего администратора
func (h *Handlers) RegisterRoutes(app *fiber.App) {
	auth := app.Group("/auth")

	auth.Get("/google", h.BeginAuth)
	auth.Get("/google/callback", h.Callback)
	auth.Post("/logout", h.Logout)
	auth.Get("/me", h.Me)
}

// ────────────────────────────────────────────────────────────────────────────
//  Helpers
// ────────────────────────────────────────────────────────────────────────────

// generateState генерирует криптографически безопасный случайный state
// для CSRF-защиты OAuth-флоу (32 байта → 64 hex-символа).
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// isHTMXRequest проверяет, пришёл ли запрос от HTMX
// (по наличию заголовка HX-Request).
func isHTMXRequest(c *fiber.Ctx) bool {
	return c.Get("HX-Request") != ""
}

// urlParams реализует интерфейс goth.Params (url.Values).
// goth.Params ожидает метод Get(key) string.
type urlParams map[string][]string

// Get возвращает первое значение по ключу (аналог url.Values.Get).
func (p urlParams) Get(key string) string {
	vals, ok := p[key]
	if !ok || len(vals) == 0 {
		return ""
	}
	return vals[0]
}
