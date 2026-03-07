# Universities KZ — System Design Document

> **Version:** 2.0  
> **Date:** July 2025  
> **Status:** Living Document  
> **Stack:** Go 1.25 · Fiber v2 · SurrealDB · MinIO · Data-as-Code (Git + JSON/MD) · GitOps

---

## Содержание

| #   | Раздел                                                        | Файл                         |
| --- | ------------------------------------------------------------- | ---------------------------- |
| 1   | [Executive Summary](./01-executive-summary.md)                | `01-executive-summary.md`    |
| 2   | [Architectural Vision](./02-architectural-vision.md)          | `02-architectural-vision.md` |
| 3   | [Data Layer (Deep Dive)](./03-data-layer.md)                  | `03-data-layer.md`           |
| 4   | [Backend Services & API](./04-backend-services.md)            | `04-backend-services.md`     |
| 5   | [Security & AppSec](./05-security.md)                         | `05-security.md`             |
| 6   | [Infrastructure](./06-infrastructure.md)                      | `06-infrastructure.md`       |
| 7   | [Frontend Integration Strategy](./07-frontend-integration.md) | `07-frontend-integration.md` |
| 8   | [Roadmap](./08-roadmap.md)                                    | `08-roadmap.md`              |

---

## Как читать этот документ

Каждый раздел — самостоятельный Markdown-файл, который можно читать изолированно. Перекрёстные ссылки между разделами оформлены как relative links.

Диаграммы выполнены в формате **Mermaid** и рендерятся в GitHub, GitLab, Obsidian и большинстве Markdown-просмотрщиков.

Все ссылки на исходный код указывают на пути внутри репозитория относительно корня проекта (`universities/`).

---

## Архитектурная парадигма

Проект реализует паттерн **Data-as-Code** в сочетании с **GitOps**:

- **Данные** (вузы, специальности, группы ОП) хранятся как структурированные `.json` и `.md` файлы в Git-репозитории.
- **Бэкенд** — тонкая API-прослойка между клиентскими приложениями и графовой базой данных, отвечает за роутинг и обход графов.
- **Синхронизация** — изменения данных в Git доставляются в SurrealDB через webhook / GitOps pipeline.
- **Админка** — устранена как компонент. Управление данными осуществляется через Git (PR → review → merge → webhook → БД).

Это решение оптимизирует затраты мощностей (бэкенд обслуживает только read-heavy API) и радикально улучшает безопасность, отсекая целый класс атак (CSRF, XSS, session hijacking, OAuth abuse).

---

## Ключевые метрики проекта (snapshot)

| Метрика                      | Значение                                  |
| ---------------------------- | ----------------------------------------- |
| Язык / рантайм               | Go 1.25                                   |
| Веб-фреймворк                | Fiber v2.52                               |
| База данных                  | SurrealDB (latest, SCHEMAFULL)            |
| Объектное хранилище          | MinIO (S3-compatible)                     |
| Управление данными           | Data-as-Code (Git + JSON/MD + webhook)    |
| Таблиц в схеме               | 5 (+ 2 edge-таблицы)                      |
| REST API endpoints           | 8 (включая `/health`)                     |
| Admin endpoints              | 0 (устранены — data-as-code)              |
| Auth endpoints               | 0 (устранены — нет веб-админки)           |
| CI jobs                      | 4 (build, test, lint, docker)             |
| Вектора атак отсечены        | CSRF, XSS, OAuth hijack, session fixation |
| Внешние зависимости (go.mod) | 12 direct                                 |

---

## Отличия от v1.0

| Аспект                     | v1.0 (монолит + HTMX-админка)            | v2.0 (тонкий API + Data-as-Code)        |
| -------------------------- | ---------------------------------------- | --------------------------------------- |
| **Управление данными**     | Веб-админка (HTMX + SSR + OAuth)         | Git-репозиторий (JSON/MD + webhook)     |
| **Аутентификация**         | Google OAuth 2.0 + cookie session        | Не требуется (Git auth на уровне VCS)   |
| **Роль бэкенда**           | API + SSR-рендеринг + CRUD + auth        | Тонкая API-прослойка + графовые запросы |
| **Поверхность атаки**      | CSRF, XSS, OAuth, session, CSS injection | Только SurrealQL injection (защищено)   |
| **Затраты мощностей**      | SSR + шаблоны + Tailwind pipeline        | Только API-ответы (JSON)                |
| **Аудит изменений данных** | Логи в приложении                        | Полная история в git log                |
| **Фронтенд-зависимости**   | Tailwind CSS, htmx.min.js, Node.js       | Отсутствуют                             |
| **Количество кода**        | ~30 admin handlers + auth + views        | 0 (удалено)                             |
