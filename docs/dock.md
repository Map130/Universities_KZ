# Аудит: README.md vs Код (v2.0 — Data-as-Code)

> **Дата:** Июль 2025
> **Контекст:** Архитектурный рефакторинг v1.0 → v2.0. Удалена веб-админка (HTMX + SSR + OAuth), внедрён паттерн Data-as-Code (Git + JSON/MD + webhook + GitOps).

---

## 1. Endpoints

### REST API (`/api/v1`) — ✅ полное совпадение

Все 7 маршрутов из README присутствуют в коде (`cmd/api/main.go`), плюс `/health`:

| README                               | Код (`main.go`)                          | Статус |
| ------------------------------------ | ---------------------------------------- | ------ |
| `GET /api/v1/universities`           | `v1.Get("/universities", ...)`           | ✅     |
| `GET /api/v1/universities/:id`       | `v1.Get("/universities/:id", ...)`       | ✅     |
| `POST /api/v1/universities/:id/logo` | `v1.Post("/universities/:id/logo", ...)` | ✅     |
| `GET /api/v1/groups`                 | `v1.Get("/groups", ...)`                 | ✅     |
| `GET /api/v1/groups/:id`             | `v1.Get("/groups/:id", ...)`             | ✅     |
| `GET /api/v1/specialties`            | `v1.Get("/specialties", ...)`            | ✅     |
| `GET /api/v1/subjects`               | `v1.Get("/subjects", ...)`               | ✅     |
| `GET /health`                        | `app.Get("/health", ...)`                | ✅     |

### Админ-панель (`/admin/*`) — ✅ корректно отсутствует

README v2.0 не упоминает admin-маршруты. В коде (`cmd/api/main.go`) нет `/admin/*` маршрутов. Папки `internal/admin/` и `internal/auth/` отсутствуют в файловой системе.

**Вердикт:** Полное совпадение. Админка удалена и из кода, и из документации.

### Auth (`/auth/*`) — ✅ корректно отсутствует

README v2.0 не упоминает auth-маршруты. В коде нет `/auth/*` маршрутов. Папка `internal/auth/` отсутствует.

**Вердикт:** Полное совпадение.

---

## 2. Data Model

### Таблицы — ✅ совпадение (README ↔ schema.surql)

| README            | `schema.surql`                                                                      | Статус |
| ----------------- | ----------------------------------------------------------------------------------- | ------ |
| `university`      | `DEFINE TABLE OVERWRITE university SCHEMAFULL`                                      | ✅     |
| `specialty`       | `DEFINE TABLE OVERWRITE specialty SCHEMAFULL`                                       | ✅     |
| `specialty_group` | `DEFINE TABLE OVERWRITE specialty_group SCHEMAFULL`                                 | ✅     |
| `subject`         | `DEFINE TABLE OVERWRITE subject SCHEMAFULL`                                         | ✅     |
| `allowed_admins`  | `DEFINE TABLE OVERWRITE allowed_admins SCHEMAFULL`                                  | ✅     |
| `offers` (edge)   | `DEFINE TABLE OVERWRITE offers ... TYPE RELATION FROM university TO specialty`      | ✅     |
| `requires` (edge) | `DEFINE TABLE OVERWRITE requires ... TYPE RELATION FROM specialty_group TO subject` | ✅     |

### Поля — ✅ ключевые поля описаны корректно

README описывает ключевые поля каждой таблицы, включая `website`, `description` и `last_year_threshold`, которые были пропущены в документации v1.0.

### Граф — ✅ точно описан

```
university ──offers──▶ specialty ──group──▶ specialty_group ──requires──▶ subject
```

Совпадает со схемой в `schema.surql` и графовыми запросами в `university_repo.go` / `specialty_group_repo.go`.

---

## 3. Архитектурная парадигма

### Data-as-Code — ✅ описан в README, подтверждён артефактами

README описывает паттерн Data-as-Code с JSON-шаблонами. В `docs/` присутствуют:

| Файл                        | Описание                              | Статус |
| --------------------------- | ------------------------------------- | ------ |
| `docs/university.json`      | Шаблон данных о вузе (с инструкциями) | ✅     |
| `docs/specialty.json`       | Шаблон данных о специальности         | ✅     |
| `docs/specialty_group.json` | Шаблон данных о группе ОП             | ✅     |

### Webhook / GitOps — 🟡 описан, но не реализован

README описывает webhook / GitOps pipeline для синхронизации данных из Git в SurrealDB. В коде (`main.go`) отсутствует `/webhook/*` эндпоинт.

**Вердикт:** Не является ошибкой — README описывает целевую архитектуру, а Roadmap корректно помечает webhook как `[ ]` (не реализовано).

---

## 4. Стек

### README vs go.mod — 🟡 мелкие несоответствия

| Компонент                    | README | Код                       | Вердикт |
| ---------------------------- | ------ | ------------------------- | ------- |
| Go 1.25                      | ✅     | ✅ `go 1.25.0` в go.mod   | ✅      |
| Fiber v2                     | ✅     | ✅ `fiber/v2 v2.52.12`    | ✅      |
| SurrealDB                    | ✅     | ✅ `surrealdb.go v1.3.0`  | ✅      |
| MinIO                        | ✅     | ✅ `minio-go/v7 v7.0.98`  | ✅      |
| Docker Compose               | ✅     | ✅ `docker-compose.yml`   | ✅      |
| Air                          | ✅     | ✅ `.air.toml`            | ✅      |
| Data-as-Code (Git + JSON/MD) | ✅     | ✅ JSON-шаблоны в `docs/` | ✅      |

### Артефакты v1.0 — 🔴 присутствуют в коде, не упомянуты в README

| Артефакт                | Назначение (v1.0)     | В README? | В коде?                             | Проблема                   |
| ----------------------- | --------------------- | --------- | ----------------------------------- | -------------------------- |
| `package.json`          | npm (Tailwind CSS)    | ❌        | ✅ Есть                             | Мёртвый файл               |
| `package-lock.json`     | npm lock              | ❌        | ✅ Есть                             | Мёртвый файл               |
| `tailwind.config.js`    | Tailwind конфигурация | ❌        | ✅ Есть                             | Мёртвый файл               |
| `goth` в go.mod         | Google OAuth          | ❌        | ✅ В зависимостях                   | Неиспользуемая зависимость |
| `.air.toml` pre_cmd     | Tailwind build        | ❌        | ✅ Ссылается на `npx tailwindcss`   | Сломанная конфигурация     |
| `.air.toml` include_dir | `views`, `static`     | ❌        | ✅ Отслеживает несуществующие папки | Мёртвая конфигурация       |

**Рекомендация:** Удалить `package.json`, `package-lock.json`, `tailwind.config.js`. Выполнить `go mod tidy` для удаления `goth`. Упростить `.air.toml` (убрать Tailwind pre_cmd, views, static из include_dir).

---

## 5. Environment

### Переменные окружения — ✅ совпадение (с оговорками)

README перечисляет 12 переменных окружения. Сверяем с `main.go` (`requireEnv` / `os.Getenv`):

| Переменная            | В README?      | В коде?                                  | Статус    |
| --------------------- | -------------- | ---------------------------------------- | --------- |
| `SURREAL_URL`         | ✅             | ✅ `requireEnv`                          | ✅        |
| `SURREAL_USER`        | ✅             | ✅ `requireEnv`                          | ✅        |
| `SURREAL_PASS`        | ✅             | ✅ `requireEnv`                          | ✅        |
| `SURREAL_NS`          | ✅             | ✅ `requireEnv`                          | ✅        |
| `SURREAL_DB`          | ✅             | ✅ `requireEnv`                          | ✅        |
| `APP_PORT`            | ✅             | ✅ `requireEnv`                          | ✅        |
| `MINIO_ENDPOINT`      | ✅             | ✅ `requireEnv`                          | ✅        |
| `MINIO_ROOT_USER`     | ✅             | ✅ `requireEnv`                          | ✅        |
| `MINIO_ROOT_PASSWORD` | ✅             | ✅ `requireEnv`                          | ✅        |
| `MINIO_PUBLIC_URL`    | ✅             | ✅ `requireEnv`                          | ✅        |
| `MINIO_USE_SSL`       | ✅             | ✅ `os.Getenv`                           | ✅        |
| `APP_ENV`             | ❌ Не в README | ✅ `os.Getenv` (не используется активно) | 🟡 Мелочь |

### Удалённые переменные v1.0 — ✅ корректно не упомянуты

| Переменная (v1.0)      | В README v2.0? | В коде? | Статус       |
| ---------------------- | -------------- | ------- | ------------ |
| `GOOGLE_CLIENT_ID`     | ❌             | ❌      | ✅ Корректно |
| `GOOGLE_CLIENT_SECRET` | ❌             | ❌      | ✅ Корректно |
| `GOOGLE_CALLBACK_URL`  | ❌             | ❌      | ✅ Корректно |
| `SESSION_SECRET`       | ❌             | ❌      | ✅ Корректно |

---

## 6. Repository Layer

### Интерфейсы — ✅ все присутствуют

| Интерфейс                  | Файл                      | В README (структура проекта)? | Статус |
| -------------------------- | ------------------------- | ----------------------------- | ------ |
| `UniversityRepository`     | `university_repo.go`      | ✅                            | ✅     |
| `SpecialtyRepository`      | `specialty_repo.go`       | ✅                            | ✅     |
| `SpecialtyGroupRepository` | `specialty_group_repo.go` | ✅                            | ✅     |
| `SubjectRepository`        | `subject_repo.go`         | ✅                            | ✅     |

### CRUD-методы сохранены — ✅ корректно для webhook

README упоминает, что CRUD-методы в репозиториях сохранены для использования webhook-handler'ом. В коде `university_repo.go` присутствуют `Create`, `Update`, `Delete`, `CreateOffer`, `UpdateOffer`, `DeleteOffer`, `DeleteAllOffers` — все доступны для будущего webhook.

---

## 7. CI Pipeline

### GitHub Actions — ✅ совпадение

| Джоб в README           | Джоб в `.github/workflows/ci.yml` | Статус |
| ----------------------- | --------------------------------- | ------ |
| Build & Vet             | `build`                           | ✅     |
| Test                    | `test`                            | ✅     |
| Lint                    | `lint`                            | ✅     |
| Docker Compose Validate | `docker`                          | ✅     |

### CI конфигурация — 🟡 air.toml вызовет ошибку

`.air.toml` содержит `pre_cmd` с `npx tailwindcss`, но `node_modules/` может отсутствовать (npm-зависимости из v1.0). При запуске `air` в dev-режиме без Node.js — ошибка.

**Рекомендация:** Упростить `.air.toml`:

```toml
[build]
  pre_cmd = []
  cmd = "go build -o ./tmp/main ./cmd/api"
  include_dir = ["cmd", "internal", "configs"]
  include_ext = ["go", "surql"]
```

---

## 8. Безопасность

### Устранённые вектора атак — ✅ корректно описаны в README

README перечисляет 5 устранённых вектора атак (CSRF, XSS, OAuth, brute-force, unauthorized admin CRUD). Все соответствуют фактическому удалению кода.

### Оставшиеся меры — ✅ корректно описаны

| Мера в README                       | В коде?                       | Статус                |
| ----------------------------------- | ----------------------------- | --------------------- |
| Параметризованные SurrealQL-запросы | ✅ `university_repo.go` и др. | ✅                    |
| Whitelist MIME-типов (файлы)        | ✅ `storage.go`               | ✅                    |
| Лимит размера файла (5 МБ)          | ✅ `storage.go`               | ✅                    |
| UUID-имена файлов                   | ✅ `storage.go`               | ✅                    |
| Двойная валидация (Go + SCHEMAFULL) | ✅ models + schema.surql      | ✅                    |
| CORS middleware (планируется)       | ❌ Не реализовано             | 🔲 Корректно помечено |
| Rate limiting (планируется)         | ❌ Не реализовано             | 🔲 Корректно помечено |

---

## 9. Roadmap

### Статусы — ✅ корректны

| Пункт Roadmap                        | README | Код                                      | Вердикт      |
| ------------------------------------ | ------ | ---------------------------------------- | ------------ |
| Docker Compose (SurrealDB + MinIO)   | `[x]`  | ✅ `docker-compose.yml`                  | ✅           |
| Go-сервис + подключение к БД         | `[x]`  | ✅ `cmd/api/main.go` + `internal/db/`    | ✅           |
| Схема данных (SurrealQL, SCHEMAFULL) | `[x]`  | ✅ `schema.surql`                        | ✅           |
| Repository-слой                      | `[x]`  | ✅ `internal/repository/`                | ✅           |
| REST API: 8 endpoints                | `[x]`  | ✅ 8 маршрутов в `main.go`               | ✅           |
| Загрузка медиа через MinIO           | `[x]`  | ✅ `internal/storage/` + `POST .../logo` | ✅           |
| CI pipeline                          | `[x]`  | ✅ `.github/workflows/ci.yml`            | ✅           |
| Data-as-Code: JSON-шаблоны           | `[x]`  | ✅ `docs/*.json`                         | ✅           |
| GitOps pipeline (webhook)            | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |
| JSON Schema валидация                | `[ ]`  | ❌ Не реализована                        | ✅ Корректно |
| CORS middleware                      | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |
| Rate limiting                        | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |
| OpenAPI-спецификация                 | `[ ]`  | ❌ Не реализована                        | ✅ Корректно |
| Dockerfile                           | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |
| React SPA / Next.js                  | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |
| Telegram Bot                         | `[ ]`  | ❌ Не реализован                         | ✅ Корректно |

**Вердикт:** Ни один `[x]` не врёт. Ни один `[ ]` не реализован. Roadmap полностью актуален.

---

## 10. Итоговая таблица

| Аспект                                                | README          | Код                        | Вердикт      |
| ----------------------------------------------------- | --------------- | -------------------------- | ------------ |
| Стек (Go, Fiber, SurrealDB, MinIO, Air, Data-as-Code) | ✅              | ✅                         | ✅ Совпадает |
| Архитектура (тонкий API + Data-as-Code)               | ✅              | ✅                         | ✅ Совпадает |
| Граф-модель (7 таблиц + 2 ребра)                      | ✅              | ✅                         | ✅ Совпадает |
| REST API — 8 маршрутов (7 + health)                   | ✅              | ✅                         | ✅ Совпадает |
| Админка / Auth — отсутствуют                          | ✅              | ✅                         | ✅ Совпадает |
| JSON-шаблоны (university, specialty, group)           | ✅              | ✅                         | ✅ Совпадает |
| Переменные окружения (12 шт)                          | ✅              | ✅                         | ✅ Совпадает |
| CI (4 джоба)                                          | ✅              | ✅                         | ✅ Совпадает |
| Безопасность (устранённые вектора)                    | ✅              | ✅                         | ✅ Совпадает |
| Roadmap (статусы `[x]` / `[ ]`)                       | ✅              | ✅                         | ✅ Совпадает |
| Артефакты v1.0 (package.json, tailwind, goth)         | ❌ Не упомянуты | ✅ Присутствуют            | 🔴 **Долг**  |
| `.air.toml` (ссылки на views, static, Tailwind)       | ❌ Не упомянут  | ✅ Устаревшая конфигурация | 🔴 **Долг**  |
| `configs/` директория                                 | ❌ Не упомянута | ✅ Пустая                  | 🟡 Мелочь    |
| `APP_ENV` переменная                                  | ❌ Не в README  | ✅ `os.Getenv`             | 🟡 Мелочь    |

---

## 11. Оставшиеся долги (от 🔴 к 🟡)

### 🔴 1. Артефакты v1.0 не удалены

Файлы `package.json`, `package-lock.json`, `tailwind.config.js` и зависимость `goth` в `go.mod` — остатки HTMX-админки. Они вводят в заблуждение (проект выглядит как Node.js + CSS-pipeline), хотя в v2.0 фронтенд-ассетов нет.

**Действие:**

1. Удалить `package.json`, `package-lock.json`, `tailwind.config.js`.
2. Запустить `go mod tidy` для удаления `goth` и связанных зависимостей.
3. Время: ~5 мин.

### 🔴 2. `.air.toml` ссылается на несуществующие папки и Tailwind

`pre_cmd` пытается запустить `npx tailwindcss` и очистить `.fiber.gz` файлы. `include_dir` отслеживает `views` и `static`, которых нет.

**Действие:**

1. Убрать `pre_cmd` (или заменить на `[]`).
2. Убрать `views` и `static` из `include_dir`.
3. Убрать `include_file = ["tailwind.config.js"]`.
4. Убрать ссылки на `output.css` из `exclude_file`.
5. Время: ~10 мин.

### 🟡 3. `configs/` директория пуста

Упоминается в `.air.toml` как отслеживаемая. Можно положить туда `.env.example` или удалить из Air.

### 🟡 4. `APP_ENV` не документирована в README

`os.Getenv("APP_ENV")` есть в `main.go`, но в README v2.0 переменная не перечислена. Не критично, т.к. переменная не используется активно (нет условного поведения в v2.0 — build tag `noauth` удалён).

---

## Резюме

**Документация v2.0 на ~95% совпадает с кодом.** Все ключевые архитектурные изменения (удаление админки, Data-as-Code, тонкий API) корректно отражены в README и System Design Document.

Единственный реальный долг — **артефакты v1.0** (`package.json`, `tailwind.config.js`, `goth`, устаревший `.air.toml`), которые не удалены из репозитория. Исправляется за ~15 минут.
