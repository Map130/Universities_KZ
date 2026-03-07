🔒 Security Audit: Universities KZ Backend (Go + Fiber + SurrealDB)

Я провёл полный анализ кодовой базы по четырём запрошенным векторам. Ниже — все находки, ранжированные по критичности.

---

## 1. SurrealQL Injection

### ✅ FINDING 1.1 — Параметризованные запросы (GOOD)

Все репозитории используют **параметризованные запросы** через `surrealdb.Query` с `map[string]any` для передачи переменных. Пользовательский ввод **никогда** не конкатенируется напрямую в строку запроса.

Пример из `university_repo.go`:

```universities/internal/repository/university_repo.go#L86-89
	if f.City != "" {
		clauses = append(clauses, "city = $city")
		vars["city"] = f.City
	}
```

Переменные `$city`, `$search`, `$type`, `$email`, `$code` и т.д. передаются через `vars`, что эквивалентно prepared statements.

### ✅ FINDING 1.2 — Whitelist для имени поля `lang` (GOOD)

Единственное место, где `fmt.Sprintf` используется в теле запроса — интерполяция имени поля языка:

```universities/internal/repository/university_repo.go#L85-98
	if f.Search != "" {
		lang := f.Lang
		if lang == "" {
			lang = "ru"
		}
		// Проверяем допустимость языка, чтобы не допустить инъекцию в имя поля.
		switch lang {
		case "kz", "ru", "en":
			// ok
		default:
			lang = "ru"
		}
		clauses = append(clauses, fmt.Sprintf("name.%s @@ $search", lang))
		vars["search"] = f.Search
	}
```

Whitelist `kz/ru/en` — корректная защита. Такая же логика повторяется в `specialty_repo.go` и `specialty_group_repo.go`. **Инъекция невозможна.**

### ✅ FINDING 1.3 — Нет DEFINE FUNCTION / embedded JS (GOOD)

В `schema.surql` нет конструкций `DEFINE FUNCTION` и нет встроенного JavaScript. Схема состоит только из `DEFINE TABLE`, `DEFINE FIELD`, `DEFINE INDEX` — безопасно.

### ⚠️ FINDING 1.4 — PERMISSIONS FULL на всех таблицах | **Критичность: Med**

Все таблицы в схеме имеют `PERMISSIONS FOR select, create, update, delete FULL`:

```universities/internal/db/schema.surql#L24-26
DEFINE TABLE OVERWRITE university SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;
```

Это повторяется для `specialty_group`, `specialty`, `subject`, `offers`, `requires`, `allowed_admins`.

**Проблема**: `FULL` означает, что **любой аутентифицированный пользователь SurrealDB** (с любым scope/role) имеет полный доступ ко всем операциям. Если вы когда-либо добавите scope-based доступ (например, через SurrealDB Record Users для публичного API), все таблицы будут открыты для всех.

**Смягчение**: Сейчас бэкенд подключается через root-credentials (`SURREAL_USER`/`SURREAL_PASS`), и SurrealDB не экспонирован наружу (только через Docker network). Поэтому актуальный risk — **Med** (architectural debt, не активная уязвимость).

**Рекомендация**: Ограничьте PERMISSIONS и добавьте явный deny-by-default:

```/dev/null/schema_fixed.surql#L1-13
-- Для таблиц, к которым обращается только бэкенд (root-credentials),
-- можно использовать NONE — root-пользователь обходит PERMISSIONS.
-- Это защитит данные, если в будущем добавятся scope-пользователи.

DEFINE TABLE OVERWRITE university SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete NONE;

-- Для allowed_admins — особенно критично:
DEFINE TABLE OVERWRITE allowed_admins SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete NONE;
```

> **Примечание**: Root-пользователь (`SURREAL_USER`) в SurrealDB **обходит PERMISSIONS**, поэтому ваш бэкенд продолжит работать. `NONE` закроет доступ для любых scope/namespace-пользователей.

---

## 2. XSS в HTMX

### ✅ FINDING 2.1 — `html/template` с автоэскейпингом (GOOD)

Renderer использует пакет `html/template` (не `text/template`):

```universities/internal/admin/render.go#L7
	"html/template"
```

Все данные (`{{.Name.RU}}`, `{{.City}}`, `{{.Abbr}}`, `{{.Search}}` и т.д.) автоматически HTML-экранируются при рендеринге. Это основная защита от XSS — **правильно реализовано**.

### ⚠️ FINDING 2.2 — `safeHTML` function зарегистрирована, но не используется | **Критичность: Low**

В `funcMap` зарегистрирована функция `safeHTML`:

```universities/internal/admin/render.go#L273-275
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
```

Grep по шаблонам показывает, что `safeHTML` **нигде не вызывается** в текущих views. Это хорошо, но сама регистрация — мина замедленного действия: любой разработчик может начать использовать `{{safeHTML .SomeUserInput}}` без понимания рисков.

**Рекомендация**: Удалите `safeHTML` из funcMap или переименуйте для явности:

```/dev/null/render_fix.go#L1-6
// Вместо generic safeHTML, создайте специализированные функции
// с валидацией, например:
"trustedAdminHTML": func(s string) template.HTML {
    // Использовать ТОЛЬКО для контента, созданного администратором
    return template.HTML(s)
},
```

### ⚠️ FINDING 2.3 — `safeCSS` + неполная санитизация CSS → CSS Injection | **Критичность: Med**

Кастомный CSS от администраторов передаётся через `safeCSS`, которая обходит авто-эскейпинг `html/template`:

```universities/internal/admin/render.go#L266-269
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s)
		},
```

`SanitizeCSS()` на серверной стороне удаляет `@import`, `url()`, `expression()`, но **не удаляет `position:fixed/absolute`** несмотря на заявление в комментариях:

```universities/internal/admin/render.go#L473-477
//   - Удаляем @import, @charset, @namespace (внешние ресурсы).
//   - Удаляем url() / expression() / -moz-binding (скрипты и ссылки).
//   - Удаляем position:fixed/absolute (предотвращаем overlay-атаки).
//   - Удаляем javascript: и data: URI-схемы.
```

Но в реализации `SanitizeCSS()` (строки 485–531) фильтрации `position` **нет** — `dangerousPatterns` не содержит `position:fixed` или `position:absolute`.

**Вектор атаки**: Скомпрометированный админ-аккаунт может инъектировать:
```/dev/null/attack.css#L1-6
/* Clickjacking overlay */
.evil { 
  position: fixed; 
  top: 0; left: 0; 
  width: 100vw; height: 100vh; 
  opacity: 0; z-index: 99999; 
}
```

**Исправленный код** — добавьте фильтрацию `position` в `SanitizeCSS`:

```universities/internal/admin/render.go#L509-517
	dangerousPatterns := []string{
		"expression(", "expression (",
		"-moz-binding",
		"javascript:", "data:",
		"behavior:",
		"position:fixed", "position: fixed",
		"position:absolute", "position: absolute",
		"position:sticky", "position: sticky",
	}
```

### ⚠️ FINDING 2.4 — `LogoURL` рендерится в `src` без валидации протокола | **Критичность: Low**

В шаблонах:
```universities/views/universities/index.html#L146-148
                                            <img class="h-10 w-10 rounded-lg object-cover border border-gray-200"
                                                 src="{{.LogoURL}}"
                                                 alt="{{.Name.RU}}"
```

`LogoURL` — это `*string`, значение которого формируется бэкендом через MinIO (`publicEndpoint + bucket + fileName`). В `html/template` значения `src` автоматически экранируются (опасные протоколы типа `javascript:` будут заблокированы), но поле `LogoURL` хранится в SurrealDB и может быть изменено напрямую через БД.

**Смягчение**: Пока доступ к БД — только через root, и `LogoURL` формируется сервером. Risk реальный, но **Low** при текущей архитектуре.

---

## 3. CSRF & CORS

### 🔴 FINDING 3.1 — Отсутствие CSRF-защиты для state-changing endpoints | **Критичность: High**

В `main.go` **нет CSRF middleware**. Ни один POST/PUT/DELETE endpoint в `/admin/*` не проверяет CSRF-токен:

```universities/cmd/api/main.go#L105-119
	app := fiber.New(fiber.Config{
		AppName:      "Universities KZ v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// ── Gzip/Deflate сжатие (on-the-fly, без файлового кэша) ─
	// Fiber middleware compress сжимает ответы на лету.
	// В отличие от fiber.Static{Compress: true}, который создаёт
	// .fiber.gz файлы рядом с оригиналами и НЕ обновляет их при
	// пересборке Tailwind → браузер получает устаревший CSS.
	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))
```

**Вектор атаки**: Если администратор заходит на вредоносный сайт, тот может отправить запрос на `POST /admin/universities` с `Content-Type: application/x-www-form-urlencoded`. Cookie `SameSite=Lax` защищает от простых POST (cross-site POST не получит cookie), **НО**:

1. `SameSite=Lax` пропускает cookie при top-level навигации (GET). Хотя ваши GET-эндпоинты не меняют состояние, это хрупкая защита.
2. Старые браузеры могут не поддерживать `SameSite`.
3. Если в будущем добавится API, принимающий GET-запросы с side-effects — CSRF станет эксплуатируемым.

**Рекомендация**: Для HTMX-based форм эффективна проверка кастомного заголовка. HTMX автоматически добавляет `HX-Request: true`, а cross-origin fetch/XHR не может установить произвольные заголовки без CORS preflight.

**Исправленный код** — добавьте CSRF-middleware для `/admin`:

```/dev/null/csrf_middleware.go#L1-51
package admin

import (
	"github.com/gofiber/fiber/v2"
)

// CSRFProtection проверяет, что state-changing запросы (POST/PUT/DELETE)
// приходят от нашего фронтенда, а не от стороннего сайта.
//
// Стратегия: Double-Submit — проверяем наличие custom header "X-Requested-With"
// или "HX-Request". Браузер не отправляет custom headers в cross-origin
// запросах без CORS preflight → атакующий сайт не сможет добавить этот заголовок.
//
// Для классических HTML-форм (без HTMX, graceful degradation)
// нужен CSRF-токен — используем Fiber csrf middleware как fallback.
func CSRFProtection() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Пропускаем safe methods.
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// HTMX-запросы: проверяем наличие заголовка HX-Request.
		// Этот заголовок не может быть установлен cross-origin без CORS preflight.
		if c.Get("HX-Request") != "" {
			return c.Next()
		}

		// Non-HTMX POST (graceful degradation): проверяем Origin/Referer.
		origin := c.Get("Origin")
		if origin == "" {
			origin = c.Get("Referer")
		}
		
		// Для production: сравнить origin с ожидаемым хостом.
		// Пока простая проверка — origin должен содержать наш хост.
		host := c.Hostname()
		if origin != "" && contains(origin, host) {
			return c.Next()
		}

		return c.Status(fiber.StatusForbidden).SendString("CSRF validation failed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}
```

Применение в `routes.go`:

```/dev/null/routes_fixed.go#L1-4
// В Setup(), после создания admin group:
admin := app.Group("/admin", authMiddleware)
admin.Use(CSRFProtection()) // <-- добавить
```

### 🔴 FINDING 3.2 — Отсутствие CORS middleware | **Критичность: Med**

CORS middleware не настроен нигде в приложении. Для публичного JSON API (`/api/v1/*`) это означает:

1. Fiber по умолчанию **не добавляет заголовки CORS** → API недоступно из стороннего фронтенда (что может быть и желаемым поведением).
2. Но также нет **явного запрета** — если в будущем потребуется CORS, его отсутствие может привести к небезопасной конфигурации при быстром добавлении.

**Рекомендация**: Добавьте явную CORS-конфигурацию:

```/dev/null/main_cors.go#L1-18
import "github.com/gofiber/fiber/v2/middleware/cors"

// В main(), перед регистрацией роутов:

// CORS для публичного API — разрешаем GET-запросы для фронтенда.
app.Use("/api/", cors.New(cors.Config{
	AllowOrigins: "*",                      // Публичный read-only API
	AllowMethods: "GET,OPTIONS",            // Только чтение
	AllowHeaders: "Content-Type,Accept",
	MaxAge:       3600,
}))

// Для /admin — CORS запрещён (same-origin only).
// Не добавляем CORS middleware для /admin/* —
// отсутствие Access-Control-Allow-Origin = implicit deny.
```

### ⚠️ FINDING 3.3 — `CookieSecure: false` | **Критичность: Med**

```universities/internal/auth/session.go#L58-61
		// Secure=false для dev-среды. В production следует выставить true
		// через reverse proxy (Caddy/Nginx) с TLS-терминацией.
		// Можно расширить конфиг, если потребуется.
		CookieSecure: false,
```

В production сессионная cookie будет отправляться по HTTP, что позволяет перехват через MITM.

**Исправленный код**:

```/dev/null/session_fixed.go#L1-9
func NewSessionStore(sessionSecret string, isProduction bool) *session.Store {
	store := session.New(session.Config{
		CookieName:     sessionCookieName,
		Expiration:     sessionExpiration,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   isProduction, // true в production, false в dev
		KeyLookup:      "cookie:" + sessionCookieName,
	})
	return store
}
```

### 🔴 FINDING 3.4 — API endpoint `POST /api/v1/universities/:id/logo` без аутентификации | **Критичность: High**

```universities/cmd/api/main.go#L219-222
	v1.Post("/universities/:id/logo", func(c *fiber.Ctx) error {
		// 1. Парсим ID вуза.
		id, err := parseRecordID("university", c.Params("id"))
```

Этот endpoint позволяет **любому** отправить POST-запрос на загрузку логотипа для произвольного вуза. Нет ни аутентификации, ни rate limiting.

**Вектор атаки**: Атакующий может:
1. Загрузить произвольные SVG-файлы (SVG может содержать `<script>` — хотя MinIO отдаёт их как `image/svg+xml`, браузер может исполнить скрипт при прямом открытии).
2. Заменить логотип любого вуза.
3. Засорить MinIO-хранилище.

**Исправленный код**:

```/dev/null/main_fix_logo.go#L1-6
// Перенести endpoint из публичного API в admin group:
// Удалить из v1:
// v1.Post("/universities/:id/logo", ...)

// Добавить в admin handlers (RegisterRoutes):
// uniGroup.Post("/:id/logo", h.UploadLogo)
```

Либо, если API-endpoint нужен для внешних интеграций, добавьте API-key authentication:

```/dev/null/main_fix_logo_apikey.go#L1-10
v1.Post("/universities/:id/logo", func(c *fiber.Ctx) error {
	// API Key authentication
	apiKey := c.Get("X-API-Key")
	if apiKey == "" || apiKey != os.Getenv("API_SECRET_KEY") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing or invalid API key",
		})
	}
	// ... остальной код ...
})
```

---

## 4. Access Control (Permissions)

### ✅ FINDING 4.1 — AuthRequired middleware (GOOD)

Все `/admin/*` роуты защищены `auth.AuthRequired(sessionStore)`:

```universities/internal/admin/routes.go#L76-79
	var authMiddleware fiber.Handler
	if auth.IsNoAuth() {
		authMiddleware = auth.NoAuthMiddleware()
	} else {
		authMiddleware = auth.AuthRequired(sessionStore)
	}
```

Middleware корректно проверяет сессию и поддерживает HTMX-redirect.

### ✅ FINDING 4.2 — Whitelist-модель админов (GOOD)

OAuth callback проверяет email по whitelist:

```universities/internal/auth/handlers.go#L241-250
	allowed, err := h.adminRepo.IsAllowed(c.Context(), email)
	if err != nil {
		log.Printf("[auth] callback: whitelist check error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "authorization check failed",
		})
	}

	if !allowed {
		log.Printf("[auth] callback: DENIED — email %s not in allowed_admins", email)
```

### ⚠️ FINDING 4.3 — Нет разграничения прав между администраторами | **Критичность: Low**

Все аутентифицированные администраторы имеют одинаковые права: могут создавать, редактировать и удалять **любые** записи (вузы, специальности, группы, предметы, других админов). Нет ролевой модели (например, "editor вуза X может редактировать только свой вуз").

**Смягчение**: При текущем масштабе (небольшой whitelist) — это приемлемо. При росте команды — рекомендую добавить `role` в `allowed_admins` (ENUM: `superadmin`, `editor`) и проверять в middleware.

### ⚠️ FINDING 4.4 — Графовые связи не проверяют ownership | **Критичность: Low**

Endpoints `AddOffer`, `DeleteOffer`, `CreateRequires`, `DeleteRequires` не проверяют, что связь принадлежит корректным сущностям. Например, `DeleteOffer` принимает ID связи напрямую и удаляет её:

```universities/internal/admin/handlers.go#L563-584
func (h *Handlers) DeleteOffer(c *fiber.Ctx) error {
```

Это не уязвимость в текущей модели (все админы имеют полный доступ), но при добавлении ролей — станет Broken Access Control.

---

## 5. Бонусные находки

### ⚠️ FINDING 5.1 — Error messages раскрывают внутренности | **Критичность: Low**

```universities/cmd/api/main.go#L156-158
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
```

`err.Error()` может содержать SurrealDB-специфичные ошибки, имена таблиц, структуру запросов. В production нужно возвращать generic error.

**Исправленный код**:

```/dev/null/main_error_fix.go#L1-6
if err != nil {
	log.Printf("[api] GetAll error: %v", err) // логируем полную ошибку
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": "internal server error",
	})
}
```

### ⚠️ FINDING 5.2 — Отсутствие Security Headers | **Критичность: Med**

Нет заголовков `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`.

**Исправленный код** — добавьте middleware:

```/dev/null/security_headers.go#L1-18
import "github.com/gofiber/fiber/v2/middleware/helmet"

// В main(), после создания app:
app.Use(helmet.New(helmet.Config{
	XSSProtection:         "0",  // Отключён (устаревший, html/template достаточно)
	ContentTypeNosniff:    "nosniff",
	XFrameOptions:         "DENY",
	ReferrerPolicy:        "strict-origin-when-cross-origin",
	CrossOriginEmbedderPolicy: "require-corp",
	CrossOriginOpenerPolicy:   "same-origin",
	CrossOriginResourcePolicy: "same-origin",
	// CSP — добавьте после тестирования:
	// ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data: " + minioPublicURL,
}))
```

### ⚠️ FINDING 5.3 — SurrealDB in-memory без persistence | **Критичность: Low (operational)**

```universities/docker-compose.yml#L7
    command: start --log debug --user ${SURREAL_USER} --pass ${SURREAL_PASS} memory # Для начала в памяти, потом прикрутишь volume
```

`memory` означает потерю всех данных при рестарте контейнера. Это нормально для dev, но убедитесь, что в production используется `file:` или `rocksdb:`.

---

## Сводная таблица

| # | Находка | Критичность | Статус |
|---|---------|-------------|--------|
| 3.4 | `POST /api/v1/.../logo` без аутентификации | **🔴 High** | Исправить немедленно |
| 3.1 | Нет CSRF middleware для admin endpoints | **🔴 High** | Исправить |
| 1.4 | PERMISSIONS FULL на всех таблицах | **🟡 Med** | Плановое |
| 2.3 | CSS sanitization не блокирует `position:fixed/absolute` | **🟡 Med** | Исправить |
| 3.2 | Нет явной CORS-конфигурации | **🟡 Med** | Плановое |
| 3.3 | `CookieSecure: false` захардкожен | **🟡 Med** | Исправить для prod |
| 5.2 | Отсутствие security headers | **🟡 Med** | Плановое |
| 2.2 | `safeHTML` зарегистрирован, но не используется | **🟢 Low** | Удалить |
| 2.4 | `LogoURL` в `src=` без валидации протокола | **🟢 Low** | Мониторинг |
| 4.3 | Нет ролевой модели для администраторов | **🟢 Low** | По мере роста |
| 4.4 | Графовые связи без ownership check | **🟢 Low** | По мере роста |
| 5.1 | Error messages раскрывают внутренности | **🟢 Low** | Исправить для prod |
| 5.3 | SurrealDB in-memory | **🟢 Low** | Для prod — file: |

**Топ-3 приоритета:**
1. Перенести `/api/v1/.../logo` под auth или добавить API-key
2. Добавить CSRF-защиту (проверка `HX-Request` + Origin для non-HTMX)
3. Добавить `position:fixed/absolute/sticky` в CSS sanitizer