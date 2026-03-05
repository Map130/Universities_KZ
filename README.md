# UniversitiesKZ

Агрегатор данных о вузах Казахстана — специальности, проходные баллы ЕНТ, гранты, медиа.

Графовая модель данных позволяет естественно выражать связи между вузами, специальностями,
группами образовательных программ и предметами ЕНТ.

## Стек

| Слой             | Технология                    |
| ---------------- | ----------------------------- |
| Язык             | Go 1.25                       |
| Веб-фреймворк    | Fiber v2                      |
| БД               | SurrealDB (graph, schemafull) |
| Хранилище файлов | MinIO (S3-compatible)         |
| OAuth 2.0        | Google (goth)                 |
| Фронтенд         | HTMX + Tailwind CSS 3         |
| Шаблонизация     | html/template                 |
| Контейнеры       | Docker Compose                |
| Dev reload       | Air                           |
| CI               | GitHub Actions                |

## Архитектура

```
university ──offers──▶ specialty ──group──▶ specialty_group ──requires──▶ subject
```

Граф в SurrealDB: вуз **предлагает** специальность, специальность принадлежит **группе ОП**,
группа ОП **требует** определённые предметы ЕНТ (профильный + второй).

Рёбра `offers` и `requires` несут собственные данные:

- **offers** — количество грантов, квотных грантов, стоимость обучения, мин. балл, порог прошлого года.
- **requires** — приоритет предмета (1 = профильный, 2 = второй).

## Структура проекта

```
cmd/api/
  main.go                        — точка входа, инициализация, маршруты API

internal/
  admin/
    routes.go                    — маршрутизация /admin/*
    handlers.go                  — CRUD-хендлеры для вузов
    specialty_handlers.go        — CRUD-хендлеры для специальностей
    group_handlers.go            — CRUD-хендлеры для групп ОП
    subject_handlers.go          — CRUD-хендлеры для предметов ЕНТ
    render.go                    — шаблонизатор (layout + фрагменты HTMX)
    render_test.go               — тесты шаблонизатора
  auth/
    config.go                    — конфиг Google OAuth
    handlers.go                  — /auth/google, /auth/google/callback, /auth/logout
    middleware.go                — AuthRequired middleware
    session.go                   — Fiber session store
    skip_default.go              — production-билд (OAuth включён)
    skip_noauth.go               — dev-билд с тегом noauth (OAuth отключён)
  db/
    database.go                  — подключение к SurrealDB, миграции
    schema.surql                 — SurrealQL-схема (SCHEMAFULL, индексы, FTS)
  models/
    common.go                    — LocalizedName, UniversityType, SubjectPriority
    university.go                — University, UniversityFilters, UniversityDetail
    specialty.go                 — Specialty, SpecialtyExpanded, SpecialtyFilters
    specialty_group.go           — SpecialtyGroup, SpecialtyGroupFilters
    subject.go                   — Subject
    edge.go                      — Offers, Requires, OfferWithSpecialty, RequiredSubject
    admin.go                     — AllowedAdmin (whitelist)
  repository/
    university_repo.go           — репозиторий вузов (SurrealQL)
    specialty_repo.go            — репозиторий специальностей
    specialty_group_repo.go      — репозиторий групп ОП
    subject_repo.go              — репозиторий предметов ЕНТ
    admin_repo.go                — репозиторий whitelist-а администраторов
  storage/
    storage.go                   — MinIO client, Uploader interface, валидация файлов

views/                           — HTML-шаблоны (html/template)
  layout.html                    — базовый layout (sidebar, header, HTMX)
  stub.html                      — заглушка для разделов в разработке
  universities/                  — шаблоны вузов (index, form, _row, _toast)
  specialties/                   — шаблоны специальностей (index, form)
  groups/                        — шаблоны групп ОП (index, form)
  subjects/                      — шаблоны предметов (index, form)

static/
  css/input.css                  — Tailwind CSS entrypoint
  css/output.css                 — скомпилированный CSS (gitignored)
  js/htmx.min.js                 — HTMX (vendored)

.github/workflows/ci.yml        — CI pipeline (build, test, lint, docker validate)
docker-compose.yml               — SurrealDB + MinIO
.air.toml                        — Air config (hot reload Go + Tailwind)
tailwind.config.js               — Tailwind config
```

## Модели данных

| Таблица           | Описание                           | Ключевые поля                                           |
| ----------------- | ---------------------------------- | ------------------------------------------------------- |
| `university`      | Вуз Казахстана                     | name (kz/ru/en), abbr, city, type, logo_url, custom_css |
| `specialty`       | Образовательная программа          | code, name (kz/ru/en), group → specialty_group          |
| `specialty_group` | Группа ОП (направление)            | code, name (kz/ru/en)                                   |
| `subject`         | Предмет ЕНТ                        | name (kz/ru/en)                                         |
| `allowed_admins`  | Whitelist email-ов администраторов | email, name                                             |
| `offers`          | Ребро: university → specialty      | grant_count, quota_grant_count, tuition_fee, min_score  |
| `requires`        | Ребро: specialty_group → subject   | priority (1 = профильный, 2 = второй)                   |

Все названия трёхъязычные (`kz`, `ru`, `en`). Все таблицы SCHEMAFULL с полнотекстовым поиском.

## API

### REST API (`/api/v1`)

| Метод | Путь                            | Описание                                         |
| ----- | ------------------------------- | ------------------------------------------------ |
| GET   | `/api/v1/universities`          | Список вузов (фильтры: city, type, search, lang) |
| GET   | `/api/v1/universities/:id`      | Вуз с развёрнутыми специальностями (FETCH)       |
| POST  | `/api/v1/universities/:id/logo` | Загрузка логотипа (multipart, до 5 МБ)           |
| GET   | `/api/v1/groups`                | Список групп ОП                                  |
| GET   | `/api/v1/groups/:id`            | Группа ОП + требуемые предметы ЕНТ               |
| GET   | `/api/v1/specialties`           | Список специальностей                            |
| GET   | `/api/v1/subjects`              | Список предметов ЕНТ                             |
| GET   | `/health`                       | Health check                                     |

### Админ-панель (`/admin`)

HTML-интерфейс на HTMX + Tailwind CSS. Полноценный CRUD для всех сущностей:

| Раздел         | Путь                  | Операции                                 |
| -------------- | --------------------- | ---------------------------------------- |
| Вузы           | `/admin/universities` | Index, New, Create, Edit, Update, Delete |
| Специальности  | `/admin/specialties`  | Index, New, Create, Edit, Update, Delete |
| Группы ОП      | `/admin/groups`       | Index, New, Create, Edit, Update, Delete |
| Предметы ЕНТ   | `/admin/subjects`     | Index, New, Create, Edit, Update, Delete |
| Администраторы | `/admin/admins`       | 🚧 В разработке (заглушка)               |

Для каждого раздела поддерживается graceful degradation — формы работают
и через стандартный POST (без JS), и через HTMX (PUT/DELETE с фрагментами).

## Аутентификация

**Google OAuth 2.0** с whitelist-ом email-адресов в таблице `allowed_admins`.

Поток:

1. `/auth/google` → редирект на Google
2. Google → `/auth/google/callback` → проверка email по whitelist → создание сессии
3. `/auth/logout` → уничтожение сессии

**Dev-режим (noauth):** при сборке с тегом `noauth` аутентификация полностью отключена.
Используется `NoAuthMiddleware`, подставляющий фейковые данные администратора.
Air по умолчанию собирает с `-tags noauth`.

## Хранилище файлов (MinIO)

- **logos** — публичный бакет (READ-ONLY policy), хранит логотипы вузов.
- **documents** — приватный бакет для внутренних документов.

Валидация при загрузке:

- Content-Type определяется по содержимому (первые 512 байт), не по заголовку.
- Допустимые типы: JPEG, PNG, WebP, SVG.
- Максимальный размер: 5 МБ.
- Имя файла — UUID + оригинальное расширение.

## Запуск

### Предварительные требования

- Go 1.25+
- Docker и Docker Compose
- Node.js (для Tailwind CSS)

### 1. Инфраструктура

```
docker compose up -d
```

SurrealDB будет на `:8000`, MinIO на `:9000` (консоль `:9001`).

### 2. Переменные окружения

Создайте `.env` файл:

```
# SurrealDB
SURREAL_URL=ws://localhost:8000/rpc
SURREAL_USER=root
SURREAL_PASS=root
SURREAL_NS=universities
SURREAL_DB=universities

# MinIO
MINIO_ENDPOINT=localhost:9000
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
MINIO_USE_SSL=false
MINIO_PUBLIC_URL=http://localhost:9000

# App
APP_PORT=3000
APP_ENV=development

# Google OAuth (не нужны при noauth)
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_CALLBACK_URL=http://localhost:3000/auth/google/callback
SESSION_SECRET=change-me-in-production
```

### 3. Установка зависимостей

```
go mod download
npm install
```

### 4. Запуск (dev)

```
air
```

Air автоматически:

- пересобирает Tailwind CSS при изменении шаблонов
- компилирует Go с тегом `noauth` (без Google OAuth)
- перезапускает сервер

### 5. Запуск (manual)

```
npm run css:build
go run ./cmd/api
```

### 6. Проверка

```
curl http://localhost:3000/health
```

## CI (GitHub Actions)

Pipeline `.github/workflows/ci.yml` запускается на push/PR в `main`:

| Job                         | Что делает                                                     |
| --------------------------- | -------------------------------------------------------------- |
| **Build & Vet**             | `go mod download` → `go mod verify` → `go vet` → `go build`    |
| **Test**                    | Поднимает SurrealDB в Docker, `go test -v -race -coverprofile` |
| **Lint**                    | `golangci-lint` (latest)                                       |
| **Docker Compose Validate** | Проверяет валидность `docker-compose.yml`                      |

## Roadmap

- [x] Docker Compose (SurrealDB + MinIO)
- [x] Go-сервис + подключение к БД
- [x] Схема данных (SurrealQL, SCHEMAFULL, FTS)
- [x] Модели данных (University, Specialty, SpecialtyGroup, Subject, Edges)
- [x] Репозитории (CRUD + фильтрация + пагинация)
- [x] REST API (universities, specialties, groups, subjects)
- [x] Загрузка логотипов через MinIO (валидация, UUID-имена, public policy)
- [x] Админ-панель (HTMX + Tailwind CSS, полный CRUD для всех сущностей)
- [x] Шаблонизатор с layout + фрагментами для HTMX
- [x] Google OAuth 2.0 + whitelist администраторов
- [x] Build tag `noauth` для dev без Google OAuth
- [x] Air config (Go + Tailwind hot reload)
- [x] GitHub Actions CI (build, test, lint, docker validate)
- [x] Graceful shutdown
- [x] Gzip/Deflate сжатие (on-the-fly)
- [ ] Управление whitelist-ом администраторов (CRUD в админке)
- [ ] Публичный фронтенд (поиск вузов, карточки специальностей)
- [ ] CRUD для рёбер offers / requires в админке
- [ ] Фильтрация и поиск в публичном API
- [ ] Интеграция с данными МОН РК
