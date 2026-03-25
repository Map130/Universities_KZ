# Настройка репозитория universities-data

Этот документ описывает, как создать и настроить отдельный репозиторий для хранения данных приложения, реализующий подход **Data-as-Code**.

## 1. Структура репозитория

Создай новый репозиторий (например, `universities-data`) и воспроизведи в нём следующую структуру директорий:

```text
universities-data/
├── .github/
│   └── workflows/
│       └── validate.yml
├── schema/
│   ├── university.schema.json
│   ├── specialty.schema.json
│   └── specialty_group.schema.json
└── data/
    ├── subjects/
    │   └── example.yml
    ├── groups/
    │   └── example.yml
    ├── specialties/
    │   └── example.yml
    └── universities/
        └── example.yml
```

> **Важно:** Файлы схем (`*.schema.json`) нужно скопировать из оригинального репозитория бэкенда (`docs/templates/schema/`).

## 2. CI/CD Pipeline (GitHub Actions)

В файле `.github/workflows/validate.yml` размести следующий код. Этот пайплайн будет автоматически проверять все `.yml` файлы на соответствие JSON-схемам при каждом коммите и пулл-реквесте.

```yaml
name: Validate Data

on:
  push:
    branches: [ "main" ]
  pull_request:
    branches: [ "main" ]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install AJV
        run: npm install -g ajv-cli ajv-formats yaml

      - name: Validate Universities
        run: |
          shopt -s nullglob
          for file in data/universities/*.{yml,yaml}; do
            echo "Checking $file..."
            ajv validate -s schema/university.schema.json -d "$file" -c ajv-formats
          done

      - name: Validate Specialties
        run: |
          shopt -s nullglob
          for file in data/specialties/*.{yml,yaml}; do
            echo "Checking $file..."
            ajv validate -s schema/specialty.schema.json -d "$file" -c ajv-formats
          done

      - name: Validate Specialty Groups
        run: |
          shopt -s nullglob
          for file in data/groups/*.{yml,yaml}; do
            echo "Checking $file..."
            ajv validate -s schema/specialty_group.schema.json -d "$file" -c ajv-formats
          done
```

## 3. Настройка вебхуков в GitHub (опционально)

Чтобы данные автоматически синхронизировались с сервером при мерже в `main`:

1. В репозитории `universities-data` зайди в **Settings** -> **Webhooks** -> **Add webhook**.
2. **Payload URL**: `https://твой-домен.com/webhook/git` (или `/webhook/sync` в зависимости от реализации на бэкенде).
3. **Content type**: `application/json`.
4. **Secret**: Укажи тот же секретный ключ, который прописан в переменной `WEBHOOK_SECRET` на сервере.
5. Выбери **Just the push event** и сохрани.

## 4. Порядок заполнения данных

При добавлении новых данных, чтобы избежать ошибок связей (foreign keys), следуй порядку:
1. `data/subjects/` — предметы (математика, физика и т.д.)
2. `data/groups/` — группы ОП (ссылаются на предметы)
3. `data/specialties/` — специальности (ссылаются на группы ОП)
4. `data/universities/` — университеты и гранты (ссылаются на специальности)

Когда всё настроено, просто делай пуш в `main`. CI проверит данные, а вебхук отправит их в SurrealDB!