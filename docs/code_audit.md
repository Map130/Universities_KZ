Now I have a comprehensive view of the entire codebase. Let me compile the review.

---

# Code Review: Universities KZ (Fiber v2 + SurrealDB + HTMX)

Проект хорошо структурирован: чёткое разделение на слои (models → repository → admin/auth handlers), интерфейсы для тестируемости, аккуратная документация. Ниже — конкретные замечания по четырём критериям.

---

## 1. Fiber Best Practices

### ✅ Что сделано хорошо
- `c.Context()` корректно пробрасывается в репозитории — контекст живёт ровно столько, сколько запрос.
- `c.Locals()` используется в middleware для передачи данных админа — каноничный подход.
- Graceful shutdown с `ShutdownWithContext` реализован правильно.
- Compress middleware включён на уровне app.

### Замечания

| Строка кода | Проблема | Go-way решение |
|---|---|---|
| `main.go` L133–162: inline-хендлеры в API-роутах (`v1.Get("/universities", func(c *fiber.Ctx) error { ... })`) | Хендлеры определены как анонимные замыкания прямо в `main()`. Это раздувает `main`, затрудняет тестирование и нарушает принцип единой ответственности. | Вынести API-хендлеры в отдельный пакет (например, `internal/api`) по аналогии с `internal/admin`. Каждый хендлер — метод структуры с инжектированными зависимостями. |
| `main.go` L133–139: `return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})` | Ошибки из репозитория (потенциально содержащие внутренние детали БД) утекают клиенту через `err.Error()`. Также не используется `fiber.NewError()` для единообразной обработки. | Использовать `fiber.NewError(fiber.StatusInternalServerError, "failed to fetch universities")` и зарегистрировать `app.Config.ErrorHandler` для единообразного формата ошибок. Внутреннюю ошибку логировать, клиенту — generic-сообщение. |
| `main.go` L160–195: загрузка логотипа в API-роуте | Логика загрузки логотипа (~35 строк) вынесена прямо в `main.go`. Дублирует аналогичную логику в `admin/handlers.go`. | Извлечь в переиспользуемый сервис или вызывать `storage.UploadImage` + `uniRepo.Update` через общий use-case. |
| `main.go` L105: `app := fiber.New(fiber.Config{...})` — нет `ErrorHandler` | Без кастомного `ErrorHandler` все `fiber.NewError()` вернут дефолтный plain-text ответ, а не JSON. | Добавить `ErrorHandler` в `fiber.Config`, который возвращает JSON для `/api/*` и HTML для `/admin/*`. |
| `main.go` L105: нет `BodyLimit` в `fiber.Config` | Без явного ограничения размер тела по умолчанию 4 МБ. Для загрузки логотипов (до 5 МБ) запрос будет обрезан. | Установить `BodyLimit: 10 << 20` (10 МБ) или подходящее значение. |
| `admin/handlers.go` L737: `func (h *Handlers) adminData(c *fiber.Ctx) AdminData` и `routes.go` L153: `adminDataFromLocals(c)` | Дублирование одного и того же кода: `adminData()` вызывает `adminDataFromLocals()`. Каждый `*Handlers` struct имеет свой дубль (`adminData()`). | Оставить только `adminDataFromLocals(c)` как пакетную функцию и убрать методы-обёртки из каждого Handlers-типа. |
| `admin/render.go` L628–630: `func SetStatus(c *fiber.Ctx, status int) *fiber.Ctx` | Функция `SetStatus` объявлена, но нигде не используется. Мёртвый код. | Удалить или задействовать. |
| `session.go`: `sessionStore` не использует `sessionSecret` | `NewSessionStore` принимает `sessionSecret`, но нигде его не использует — Fiber `session.Store` по умолчанию не подписывает cookie значения. | Если нужна подпись cookie — использовать `fiber/middleware/encryptcookie` или `KeyGenerator`. Если не нужна — убрать параметр `sessionSecret` из `NewSessionStore`, чтобы не создавать ложное чувство безопасности. |

---

## 2. SurrealQL & Driver

### ✅ Что сделано хорошо
- **Параметры (bind)** используются повсеместно: `$search`, `$city`, `$type`, `$limit`, `$offset`, `$email` и т.д. — защита от инъекций и ускорение планирования запросов.
- `FETCH out` для графовых связей — идиоматичный SurrealQL.
- Миграции через `embed.FS` + `OVERWRITE` — идемпотентны.
- `surrealdb.Merge` для PATCH-семантики вместо полной перезаписи.

### Замечания

| Строка кода | Проблема | Go-way решение |
|---|---|---|
| `university_repo.go` L86–95: `fmt.Sprintf("name.%s @@ $search", lang)` | Имя языкового поля (`kz`, `ru`, `en`) интерполируется в запрос через `fmt.Sprintf`. Хотя значение валидируется через `switch`, это — конкатенация в SurrealQL. Если кто-то добавит новый язык без проверки, возникнет риск инъекции. | Определить `allowedLangs = map[string]string{"kz": "name.kz", "ru": "name.ru", "en": "name.en"}` и использовать готовый фрагмент запроса из map. Это исключает любую возможность инъекции. |
| `subject_repo.go` L104–109: `` "SELECT VALUE id FROM specialty WHERE ` + "`group`" + ` IN $group_ids" `` | Многошаговый запрос с `LET` + конкатенация backtick-escaping для поля `group`. Сам запрос корректен, но конкатенация строк затрудняет чтение. | Использовать raw string literal с backticks внутри одинарных кавычек SurrealQL, или вынести запрос в отдельную `.surql`-файл, встраиваемый через `embed`. |
| `specialty_group_repo.go` L141, `specialty_repo.go` L133: `GetWithSubjects` делает 2 последовательных запроса к БД | Сначала `GetByID()`, потом `SELECT * FROM requires WHERE in = $group_id`. Два сетевых round-trip'а. | Объединить в один SurrealQL-запрос: `SELECT *, (SELECT * FROM requires WHERE in = $parent.id FETCH out) AS subjects FROM specialty_group WHERE id = $id`. Один round-trip вместо двух. |
| `university_repo.go` L143: `GetWithSpecialties` — аналогично 2 запроса | `GetByID` + `SELECT * FROM offers WHERE in = $uni_id FETCH out`. | Объединить: `SELECT *, (SELECT * FROM offers WHERE in = $parent.id FETCH out) AS offers FROM university WHERE id = $id`. |
| `admin_repo.go` L107–115: `Create` — `data` map создаётся, но не используется | Строки 107–113 создают `data` map, но фактически используется inline `map[string]any` в `surrealdb.Query` (L115–120). `data` — мёртвый код. | Удалить неиспользуемый `data` map. |
| `db/database.go` L48: `db.SignIn(ctx, map[string]any{"user": ..., "pass": ...})` | Root-level auth без указания типа scope. Работает, но при переходе на scope-based auth потребуется рефакторинг. | Для текущего случая (root) — ОК. Для production рекомендуется создать scope-user с ограниченными правами и использовать scope auth. |
| `db/database.go` L70: `surrealdb.Query[any](ctx, db, string(schema), nil)` | Миграция выполняется как один огромный запрос. При ошибке в середине схемы — часть изменений применится, часть нет (SurrealDB не имеет транзакций для DDL). | Документировать, что миграции идемпотентны (уже сделано). В будущем — рассмотреть разбивку на отдельные файлы с порядковыми номерами. |

---

## 3. HTMX Integration

### ✅ Что сделано хорошо
- `isHTMXRequest(c)` проверяет заголовок `HX-Request` — стандартный способ определения HTMX-запроса.
- `HX-Redirect` используется для redirect из HTMX-контекста (в middleware и в хендлерах) — корректная реализация.
- `RenderPage` автоматически выбирает full page vs fragment на основе `HX-Request` — элегантный подход.
- Graceful Degradation через `_method=PUT` в POST — работает и без JS.

### Замечания

| Строка кода | Проблема | Go-way решение |
|---|---|---|
| `admin/handlers.go` L421–424: `Delete` возвращает `c.SendString("")` для HTMX | При успешном удалении HTMX получает пустой ответ для `hx-swap="outerHTML"`. Однако нет `HX-Trigger` заголовка для toast-уведомления на клиенте. | Добавить `c.Set("HX-Trigger", `{"showToast": "Вуз удалён"}`)` перед `c.SendString("")`. На клиенте — слушать `showToast` событие для отображения toast. |
| `admin/handlers.go` L583: `DeleteOffer` возвращает `c.SendString("")` | Аналогично — нет `HX-Trigger` с уведомлением об удалении offer. | Аналогично: `c.Set("HX-Trigger", `{"showToast": "Связь удалена"}`)`. |
| `admin/render.go` L720–724: `renderFormWithErrors` устанавливает 422 статус | HTMX по умолчанию **не** обрабатывает ответы с HTTP-кодами 4xx. Без настройки `htmx.config.responseHandling` или `hx-on::response-error` форма не обновится при ошибке валидации. | Убедиться, что на клиенте настроен `htmx.config.responseHandling` для обработки 422 (swap content), или отдавать 200 с визуальными ошибками внутри HTML. |
| Все `redirectToListWithFlash` / `redirectToEditWithFlash` передают flash через query params | Flash-сообщения видны в URL (`?flash=success&flash_msg=...`). При обновлении страницы сообщение показывается повторно. Не критично, но нежелательно. | Для HTMX-ответов использовать `HX-Trigger` с данными flash-сообщения. Для обычных запросов — cookie-based flash или серверное хранение. |
| `admin/render.go` L130: `RenderPage` не устанавливает `Vary: HX-Request` | Если CDN/proxy кэширует ответ, HTMX-фрагмент может быть отдан как полная страница (и наоборот). | Добавить `c.Set("Vary", "HX-Request")` в `RenderPage` перед отдачей ответа. Это стандартная практика при различении ответов по `HX-Request`. |
| `admin/specialty_handlers.go`, `group_handlers.go`, `subject_handlers.go`: `Delete` — идентичный паттерн | Во всех `Delete`-хендлерах для HTMX возвращается пустая строка без `HX-Trigger`. | Вынести в общую helper-функцию: `htmxDeleteResponse(c, message)`, которая ставит `HX-Trigger` и возвращает пустой ответ. |

---

## 4. Error Handling

### ✅ Что сделано хорошо
- Ошибки из репозиториев оборачиваются через `fmt.Errorf("context: %w", err)` — можно анализировать цепочку через `errors.Is/As`.
- В admin-хендлерах ошибки разделяются: валидация → форма с ошибками, DB-ошибка → flash-сообщение.
- В auth-хендлерах каждый шаг OAuth проверяется и логируется.

### Замечания

| Строка кода | Проблема | Go-way решение |
|---|---|---|
| `main.go` L135: `c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})` (и все API-роуты) | Нигде в API-хендлерах не используется `fiber.NewError()`. Ошибки возвращаются вручную через `c.Status().JSON()`. Это обходит `ErrorHandler` Fiber и делает невозможным централизованную обработку ошибок. | Использовать `return fiber.NewError(fiber.StatusInternalServerError, "internal error")` и настроить `app.Config.ErrorHandler` для формирования JSON/HTML ответов. |
| `main.go` L143: `return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})` | `parseRecordID` возвращает `fmt.Errorf("empty record ID")` — это внутренняя ошибка, утекающая клиенту. | `return fiber.NewError(fiber.StatusBadRequest, "invalid university ID")`. |
| `main.go` L148: `return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})` | Ошибка из `GetWithSpecialties` может содержать детали DB-запроса. | Логировать `err`, клиенту — `fiber.NewError(fiber.StatusNotFound, "university not found")`. |
| `admin/handlers.go` L143–145: `errs["_global"] = "Ошибка при сохранении..."` | Ошибки БД не типизированы. Нельзя отличить "дубликат записи" от "connection lost" — для обоих показывается одинаковый generic-текст. | Определить sentinel-ошибки в repository (`ErrDuplicateCode`, `ErrNotFound`) и обрабатывать их в хендлерах через `errors.Is()`. |
| `specialty_group_repo.go` L130: `return nil, fmt.Errorf("specialtyGroup.GetByCode: group with code %q not found", code)` | "Not found" — это не `error` в Go-way. Возврат ошибки для "запись не найдена" смешивает отсутствие данных с реальной ошибкой. | Возвращать `(nil, nil)` для "не найдено" (как в `admin_repo.go` L96) или определить `ErrNotFound = errors.New("not found")`. Текущий код несогласован: `AdminRepo.GetByEmail` возвращает `nil, nil`, а `SpecialtyGroupRepo.GetByCode` — `nil, error`. |
| `storage/storage.go` L180–186: ошибки `UploadImage` | Ошибки валидации (размер, MIME-тип) и ошибки инфраструктуры (MinIO unreachable) возвращаются одинаково — через `fmt.Errorf`. Хендлер не может отличить 400 от 500. | Определить типизированные ошибки: `type ValidationError struct{ Msg string }`. В хендлере: `if errors.As(err, &valErr) → 400, else → 500`. |
| `auth/handlers.go` L89–95: `BeginAuth` возвращает `c.Status(500).JSON(...)` | OAuth-ошибки возвращаются как JSON. Но пользователь инициирует OAuth через браузерную навигацию — он ожидает HTML-страницу, а не JSON-ответ. | Для OAuth-хендлеров рендерить HTML-страницу ошибки (или редирект на `/login?error=...`). |

---

## Общие рекомендации

1. **Централизованный `ErrorHandler`**. Самое важное изменение: один `ErrorHandler` в `fiber.Config`, который отличает API (`/api/*` → JSON) от HTML (`/admin/*` → rendered error page) и логирует внутренние ошибки отдельно от клиентских сообщений.

2. **`session.Store` и `sessionSecret`**. Параметр `sessionSecret` передаётся, но не используется. Fiber `session.Store` по умолчанию хранит session ID в cookie, но **не подписывает** его. Для production нужен `encryptcookie` middleware или подпись через `KeyGenerator`.

3. **`Vary: HX-Request`**. Добавить в `RenderPage` — обязательно при использовании proxy/CDN, чтобы кэш различал фрагменты и полные страницы.

4. **Единый подход к "not found"**. Привести все репозитории к одному стилю: либо `(nil, nil)` для отсутствия записи + `(nil, err)` для реальной ошибки, либо `(nil, ErrNotFound)` — но не `fmt.Errorf("not found")`.

5. **`HX-Trigger` при удалении**. Все `Delete`-хендлеры для HTMX возвращают пустое тело без `HX-Trigger`. Это лишает клиента возможности показать toast-уведомление.

6. **`BodyLimit` в `fiber.Config`**. Без явного указания лимит 4 МБ, а `maxLogoSize` = 5 МБ. Загрузка логотипа между 4 и 5 МБ будет обрезана Fiber до того, как дойдёт до валидации в `storage.UploadImage`.