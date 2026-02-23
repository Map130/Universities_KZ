# UniversitiesKZ

Агрегатор данных о вузах Казахстана — специальности, проходные баллы ЕНТ, гранты, медиа.

## Стек

| Что              | Чем            |
| ---------------- | -------------- |
| Язык             | Go 1.25        |
| Веб              | Fiber v2       |
| БД               | SurrealDB      |
| Хранилище файлов | MinIO (S3)     |
| Контейнеры       | Docker Compose |
| Dev reload       | Air            |

## Структура

```
cmd/api/          — точка входа
internal/
  db/             — подключение и запросы к SurrealDB
  models/         — модели данных
configs/          — конфиги, .env
docker-compose.yml
```

## Запуск

1. Поднять инфраструктуру:

```
docker compose up -d
```

SurrealDB будет на `:8000`, MinIO на `:9000` (консоль `:9001`).

2. Задать переменные окружения (или положить в `.env`):

3. Запустить API:

```
go run ./cmd/api
```

Или через Air для hot reload:

```
air
```

4. Проверить:

```
curl http://localhost:8080/health
```

## Roadmap

- [x] Docker Compose (SurrealDB + MinIO)
- [x] Go-сервис + подключение к БД
- [ ] Схема данных (SurrealQL)
- [ ] API поиска вузов
- [ ] Загрузка медиа через MinIO
