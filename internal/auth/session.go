// Package auth реализует Google OAuth 2.0 аутентификацию для админ-панели.
package auth

import (
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
)

// ────────────────────────────────────────────────────────────────────────────
//  Константы сессии
// ────────────────────────────────────────────────────────────────────────────

const (
	// SessionKeyEmail — ключ, под которым email администратора хранится в сессии.
	SessionKeyEmail = "admin_email"

	// SessionKeyName — ключ для имени администратора (из Google-профиля).
	SessionKeyName = "admin_name"

	// SessionKeyAvatarURL — ключ для URL аватарки (из Google-профиля).
	SessionKeyAvatarURL = "admin_avatar_url"

	// SessionKeyAuthAt — ключ для timestamp момента аутентификации (RFC 3339).
	SessionKeyAuthAt = "admin_auth_at"

	// sessionCookieName — имя cookie, в которой хранится session ID.
	sessionCookieName = "universities_admin_session"

	// sessionExpiration — время жизни сессии.
	sessionExpiration = 24 * time.Hour
)

// ────────────────────────────────────────────────────────────────────────────
//  Фабрика session store
// ────────────────────────────────────────────────────────────────────────────

// NewSessionStore создаёт и настраивает Fiber session store.
// Используется как единственный источник правды о сессиях в приложении —
// один экземпляр передаётся и в хендлеры, и в middleware.
func NewSessionStore(sessionSecret string) *session.Store {
	store := session.New(session.Config{
		// Имя cookie для session ID.
		CookieName: sessionCookieName,

		// Время жизни сессии (24 часа).
		Expiration: sessionExpiration,

		// HTTPOnly — cookie недоступна из JavaScript (защита от XSS).
		CookieHTTPOnly: true,

		// SameSite=Lax — cookie отправляется при top-level навигации,
		// что необходимо для OAuth-редиректа из Google.
		CookieSameSite: "Lax",

		// Secure=false для dev-среды. В production следует выставить true
		// через reverse proxy (Caddy/Nginx) с TLS-терминацией.
		// Можно расширить конфиг, если потребуется.
		CookieSecure: false,

		// KeyLookup определяет, откуда брать session ID.
		// По умолчанию "cookie:<CookieName>", что нам и нужно.
		KeyLookup: "cookie:" + sessionCookieName,
	})

	return store
}

// ────────────────────────────────────────────────────────────────────────────
//  Helpers для работы с данными сессии
// ────────────────────────────────────────────────────────────────────────────

// SetAdminSession записывает данные администратора в сессию после
// успешной аутентификации и проверки whitelist.
func SetAdminSession(sess *session.Session, email, name, avatarURL string) {
	sess.Set(SessionKeyEmail, email)
	sess.Set(SessionKeyName, name)
	sess.Set(SessionKeyAvatarURL, avatarURL)
	sess.Set(SessionKeyAuthAt, time.Now().UTC().Format(time.RFC3339))
}

// GetAdminEmail извлекает email администратора из сессии.
// Возвращает пустую строку, если сессия не содержит email
// (пользователь не аутентифицирован).
func GetAdminEmail(sess *session.Session) string {
	val := sess.Get(SessionKeyEmail)
	if val == nil {
		return ""
	}
	email, ok := val.(string)
	if !ok {
		return ""
	}
	return email
}

// GetAdminName извлекает имя администратора из сессии.
func GetAdminName(sess *session.Session) string {
	val := sess.Get(SessionKeyName)
	if val == nil {
		return ""
	}
	name, ok := val.(string)
	if !ok {
		return ""
	}
	return name
}

// GetAdminAvatarURL извлекает URL аватарки из сессии.
func GetAdminAvatarURL(sess *session.Session) string {
	val := sess.Get(SessionKeyAvatarURL)
	if val == nil {
		return ""
	}
	url, ok := val.(string)
	if !ok {
		return ""
	}
	return url
}

// ClearAdminSession удаляет все данные администратора из сессии (logout).
func ClearAdminSession(sess *session.Session) error {
	return sess.Destroy()
}

// IsAuthenticated проверяет, содержит ли сессия email администратора.
// Это быстрая проверка без обращения к БД или Google.
func IsAuthenticated(sess *session.Session) bool {
	return GetAdminEmail(sess) != ""
}
