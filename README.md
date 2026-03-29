# UniversitiesKZ

Агрегатор данных о вузах Казахстана — специальности, проходные баллы ЕНТ, гранты, медиа.

## Архитектура

Проект реализует паттерн **Data-as-Code**: данные о вузах, специальностях и группах ОП хранятся как структурированные **YAML** и **Markdown** файлы в Git-репозитории. Бэкенд выступает тонкой прослойкой, которая синхронизирует данные из Git в графовую базу данных и рендерит интерфейс.

```
┌──────────────────────────────────────────────────────────┐
│  Git Repository (Data-as-Code)                           │
│  *.yml — структурированные данные вузов/специальностей   │
│  *.md  — описания вузов, контент                         │
│                  │  webhook / tarball sync               │
├──────────────────▼───────────────────────────────────────┤
│  Go Backend (Thin Layer)                                 │
│  internal/webhook    — синхронизация с Git               │
│  internal/views      — SSR рендеринг (Templ + HTMX)      │
│  internal/repository — графовые запросы                  │
├──────────────────────────────────────────────────────────┤
│  SurrealDB (Graph + Document + FTS)                      │
│  internal/db         — подключение, миграции, пул        │
└──────────────────────────────────────────────────────────┘
```

### Почему Data-as-Code

| Проблема классической админки          | Решение через Data-as-Code                        |
| -------------------------------------- | ------------------------------------------------- |
| Вектор атаки: OAuth, сессии, CSRF, XSS | Отсечён полностью — нет веб-интерфейса управления |
| Нужен сервер для CRUD                  | Данные редактируются в Git (PR → review → merge)  |
| Дублирование валидации (клиент+сервер) | Валидация через JSON Schema + CI                  |
| Сложность аудита изменений             | Полная история в git log                          |
| Восстановление данных                  | Мгновенный Full Sync из Git в чистую БД           |

## Стек

| Компонент      | Технология     | Назначение                        |
| -------------- | -------------- | --------------------------------- |
| Язык           | Go 1.25        | Основной сервис                   |
| Веб-фреймворк  | Fiber v2       | Роутинг, middleware               |
| Frontend       | Templ + HTMX   | SSR, интерактивность без тяжелых JS-фреймворков |
| БД             | SurrealDB      | Графовые запросы, FTS, SCHEMAFULL |
| Прокси / SSL   | Caddy          | Автоматический HTTPS, Reverse Proxy |
| Контейнеры     | Docker Compose | Оркестрация                       |
| Данные         | Git + YAML/MD  | Data-as-Code, webhook, GitOps     |

## Модели данных (граф)

```
university ──offers──▶ specialty ──group──▶ specialty_group ──requires──▶ subject
```

## API & Web Endpoints

### Публичный интерфейс (Web)

| Путь            | Описание                                    |
| --------------- | ------------------------------------------- |
| `/`             | Главная страница                            |
| `/universities` | Каталог вузов (фильтры, поиск)              |
| `/groups`       | Список групп образовательных программ       |
| `/calculator`   | Калькулятор шансов на грант                 |

### REST API (`/api/v1`)

| Метод | Путь                   | Описание                                   |
| ----- | ---------------------- | ------------------------------------------ |
| `GET` | `/api/v1/universities` | Список вузов (фильтры: city, type, search) |
| `GET` | `/api/v1/groups`       | Список групп ОП                            |
| `GET` | `/api/v1/specialties`  | Список специальностей                      |
| `GET` | `/api/v1/subjects`     | Список предметов ЕНТ                       |

## Переменные окружения

```env
# SurrealDB
SURREAL_URL=ws://surrealdb:8000/rpc
SURREAL_USER=root
SURREAL_PASS=your_secure_password
SURREAL_NS=main
SURREAL_DB=main

# GitHub Integration (Data-as-Code)
GITHUB_TOKEN=your_pat_token
GITHUB_OWNER=your_org_or_user
GITHUB_REPO=universities-data
GITHUB_BRANCH=main
WEBHOOK_SECRET=your_webhook_secret

# App
APP_PORT=8080
DOMAIN_NAME=api.universities.kz
TLS_EMAIL=admin@example.com
```

## Запуск

### 1. Поднять инфраструктуру

```sh
docker compose up -d
```

### 2. Установка зависимостей

```sh
go mod download
# Установка templ CLI (для разработки)
go install github.com/a-h/templ/cmd/templ@latest
```

### 3. Запуск (dev)

```sh
# Генерация шаблонов и запуск с hot-reload
air
```

## Структура проекта

```
cmd/api/
  main.go                — точка входа, инициализация пула БД и роутинга

internal/
  db/                    — пул подключений и SurrealQL миграции
  handlers/              — обработчики JSON API
  web/                   — обработчики веб-страниц (SSR)
  views/                 — Templ-шаблоны фронтенда
  webhook/               — логика синхронизации с Git (Tarball/Push)
  repository/            — графовые запросы к SurrealDB
  models/                — доменные модели (Go)

static/                  — статика (CSS, HTMX)
docs/                    — документация, архитектурные решения
```

## Roadmap

- [x] Docker Compose (SurrealDB + Caddy)
- [x] Схема данных (SurrealQL, SCHEMAFULL)
- [x] Repository-слой (Графовые запросы)
- [x] Data-as-Code: Синхронизация через GitHub Tarball
- [x] Публичный фронтенд (Templ + HTMX)
- [x] Калькулятор шансов (MVP)
- [ ] Инкрементальная синхронизация (обработка конкретных файлов из Push event)
- [ ] Валидация YAML схем в CI репозитория данных
- [ ] Structured logging (slog)
- [ ] Telegram Bot
- [ ] Мобильное приложение
