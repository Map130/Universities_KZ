# UniversitiesKZ

Агрегатор данных о вузах Казахстана — специальности, проходные баллы ЕНТ, гранты, медиа.

## Архитектура

Проект реализует паттерн **Data-as-Code**: данные о вузах, специальностях и группах ОП хранятся как структурированные `.json` и `.md` файлы в Git-репозитории. Бэкенд выступает **тонкой API-прослойкой** между клиентскими приложениями и графовой базой данных, отвечая за роутинг и обход графов.

```
┌─────────────────────────────────────────────────────────┐
│  Git Repository (Data-as-Code)                          │
│  *.json — структурированные данные вузов/специальностей │
│  *.md   — описания вузов, контент                       │
│                  │  webhook / gitops                     │
├──────────────────▼──────────────────────────────────────┤
│  Go API (тонкая прослойка)                              │
│  cmd/api/main.go           — роутинг, API endpoints     │
│  internal/repository/*     — графовые запросы            │
│  internal/models/*         — доменные модели             │
├─────────────────────────────────────────────────────────┤
│  SurrealDB (граф + документы + FTS)                     │
│  internal/db/*             — подключение, миграции       │
├─────────────────────────────────────────────────────────┤
│  MinIO (S3-совместимое хранилище медиа)                  │
│  internal/storage/*        — загрузка/удаление файлов    │
└─────────────────────────────────────────────────────────┘
```

### Почему Data-as-Code

| Проблема классической админки          | Решение через Data-as-Code                        |
| -------------------------------------- | ------------------------------------------------- |
| Вектор атаки: OAuth, сессии, CSRF, XSS | Отсечён полностью — нет веб-интерфейса управления |
| Нужен сервер для CRUD                  | Данные редактируются в Git (PR → review → merge)  |
| Дублирование валидации (клиент+сервер) | Валидация через JSON Schema + CI                  |
| Затраты мощностей на SSR-админку       | Бэкенд обслуживает только read-only API           |
| Сложность аудита изменений             | Полная история в git log                          |

## Стек

| Компонент        | Технология     | Назначение                        |
| ---------------- | -------------- | --------------------------------- |
| Язык             | Go 1.25        | API-сервер                        |
| Веб-фреймворк    | Fiber v2       | Роутинг, middleware, сжатие       |
| БД               | SurrealDB      | Графовые запросы, FTS, SCHEMAFULL |
| Хранилище файлов | MinIO (S3)     | Логотипы вузов, документы         |
| Контейнеры       | Docker Compose | Оркестрация инфраструктуры        |
| Dev reload       | Air            | Hot reload при разработке         |
| Данные           | Git + JSON/MD  | Data-as-Code, webhook, GitOps     |

## Модели данных (граф)

```
university ──offers──▶ specialty ──group──▶ specialty_group ──requires──▶ subject
```

| Таблица           | Тип   | Ключевые поля                                                                                 |
| ----------------- | ----- | --------------------------------------------------------------------------------------------- |
| `university`      | Узел  | `name` (kz/ru/en), `abbr`, `city`, `type`, `logo_url`, `website`, `description`, `custom_css` |
| `specialty`       | Узел  | `code`, `name` (kz/ru/en), `group`                                                            |
| `specialty_group` | Узел  | `code`, `name` (kz/ru/en)                                                                     |
| `subject`         | Узел  | `name` (kz/ru/en)                                                                             |
| `allowed_admins`  | Узел  | `email`                                                                                       |
| `offers`          | Ребро | `grant_count`, `quota_grant_count`, `tuition_fee`, `min_score`, `last_year_threshold`         |
| `requires`        | Ребро | `in` (specialty_group), `out` (subject)                                                       |

Все таблицы определены как `SCHEMAFULL` с `ASSERT`-валидацией на уровне БД.

## API Endpoints

### REST API (`/api/v1`)

| Метод  | Путь                            | Описание                                   |
| ------ | ------------------------------- | ------------------------------------------ |
| `GET`  | `/api/v1/universities`          | Список вузов (фильтры: city, type, search) |
| `GET`  | `/api/v1/universities/:id`      | Детали вуза + специальности (граф offers)  |
| `POST` | `/api/v1/universities/:id/logo` | Загрузка логотипа вуза                     |
| `GET`  | `/api/v1/groups`                | Список групп ОП                            |
| `GET`  | `/api/v1/groups/:id`            | Группа ОП + предметы ЕНТ (граф requires)   |
| `GET`  | `/api/v1/specialties`           | Список специальностей                      |
| `GET`  | `/api/v1/subjects`              | Список предметов ЕНТ                       |

### Служебные

| Метод | Путь      | Описание     |
| ----- | --------- | ------------ |
| `GET` | `/health` | Health check |

## Data-as-Code: JSON-шаблоны

Данные о вузах и специальностях описываются в типовых JSON-файлах, которые хранятся в отдельном Git-репозитории и доставляются через webhook / GitOps пайплайн.

### `university.json` — Шаблон вуза

Содержит: название (kz/ru/en), аббревиатуру, город, тип, сайт, описание, список offers (специальности с грантами и баллами), источники данных.

### `specialty.json` — Шаблон специальности

Содержит: код ОП, название (kz/ru/en), код группы ОП, источники данных.

### `specialty_group.json` — Шаблон группы ОП

Содержит: код группы, название (kz/ru/en), два профильных предмета ЕНТ, источники данных.

## Переменные окружения

```env
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
```

Все переменные кроме `MINIO_USE_SSL` являются обязательными — приложение завершается с ошибкой при их отсутствии (fail-fast).

## Запуск

### 1. Поднять инфраструктуру

```sh
docker compose up -d
```

SurrealDB будет на `:8000`, MinIO на `:9000` (консоль `:9001`).

### 2. Задать переменные окружения

Скопировать пример выше в `.env` файл в корне проекта.

### 3. Запустить API

```sh
go run ./cmd/api
```

Или через Air для hot reload:

```sh
air
```

### 4. Проверить

```sh
curl http://localhost:8080/health
```

## Структура проекта

```
cmd/api/
  main.go                — точка входа, роутинг, API endpoints

internal/
  db/
    database.go          — подключение к SurrealDB, миграции
    schema.surql         — SurrealQL схема (SCHEMAFULL, индексы, FTS)
  models/
    common.go            — общие типы (LocalizedName, UniversityType)
    edge.go              — модели рёбер (Offers, Requires)
    specialty.go         — модель специальности
    specialty_group.go   — модель группы ОП
    subject.go           — модель предмета ЕНТ
    university.go        — модель вуза
  repository/
    university_repo.go   — CRUD + графовые запросы для вузов
    specialty_repo.go    — CRUD для специальностей
    specialty_group_repo.go — CRUD + граф для групп ОП
    subject_repo.go      — CRUD + обратный обход графа для предметов
  storage/
    storage.go           — MinIO: загрузка/удаление файлов, бакеты

docs/                    — документация, system design, аудиты
  system-design/         — архитектурная документация (8 разделов)
  *.json                 — JSON-шаблоны для data-as-code
  *.md                   — описания, аудиты

docker-compose.yml       — SurrealDB + MinIO
.github/workflows/ci.yml — CI: build, test, lint, docker validate
```

## CI Pipeline

Четыре джоба в GitHub Actions (`.github/workflows/ci.yml`):

| Джоб                        | Что делает                                   |
| --------------------------- | -------------------------------------------- |
| **Build & Vet**             | `go mod verify` → `go vet` → `go build`      |
| **Test**                    | SurrealDB in Docker → `go test -race -cover` |
| **Lint**                    | `golangci-lint` (latest)                     |
| **Docker Compose Validate** | `docker compose config --quiet`              |

## Безопасность

Переход на Data-as-Code **отсекает** следующие вектора атак:

| Вектор (старая архитектура)        | Статус                      |
| ---------------------------------- | --------------------------- |
| CSRF на admin endpoints            | ✅ Устранён — нет админки   |
| XSS через SSR-шаблоны              | ✅ Устранён — нет шаблонов  |
| OAuth hijacking / session fixation | ✅ Устранён — нет OAuth     |
| Brute-force на login               | ✅ Устранён — нет логина    |
| Unauthorized CRUD через admin API  | ✅ Устранён — нет admin API |

**Оставшиеся меры:**

- Параметризованные SurrealQL-запросы (защита от injection)
- Whitelist MIME-типов и размера при загрузке файлов
- Валидация данных: Go-типы + SurrealDB SCHEMAFULL ASSERT
- CORS middleware (планируется)
- Rate limiting для публичного API (планируется)

## Roadmap

- [x] Docker Compose (SurrealDB + MinIO)
- [x] Go-сервис + подключение к БД
- [x] Схема данных (SurrealQL, SCHEMAFULL)
- [x] Repository-слой (CRUD + графовые запросы)
- [x] REST API: 8 read endpoints + health
- [x] Загрузка медиа через MinIO
- [x] CI pipeline (build, test, lint, docker)
- [x] Data-as-Code: JSON-шаблоны для вузов/специальностей
- [ ] GitOps pipeline: webhook → валидация JSON → синхронизация с БД
- [ ] JSON Schema валидация в CI для data-репозитория
- [ ] CORS middleware для публичного API
- [ ] Rate limiting
- [ ] OpenAPI-спецификация
- [ ] Стандартизация ответов API (`{data, meta}` envelope)
- [ ] Structured logging (slog)
- [ ] Dockerfile (multi-stage build)
- [ ] Production docker-compose (+ Caddy reverse proxy)
- [ ] Публичный фронтенд (React SPA / Next.js)
- [ ] Telegram Bot MVP
