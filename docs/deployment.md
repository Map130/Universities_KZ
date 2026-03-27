# Инструкция по деплою на DigitalOcean Droplet

Этот документ содержит список всех секретов и шагов, необходимых для успешного автоматического деплоя бэкенда с использованием GitHub Actions.

## 1. Подготовка сервера (Droplet)

Перед первым деплоем убедись, что на твоем дроплете:
1. Установлен **Docker** и **Docker Compose**.
2. Добавлен твой **публичный SSH-ключ** в файл `~/.ssh/authorized_keys`.
3. Открыты порты `80` и `443` в файрволе (UFW):
   ```bash
   ufw allow 80/tcp
   ufw allow 443/tcp
   ufw allow 22/tcp
   ufw enable
   ```

## 2. Настройка GitHub Repository Secrets

Перейди в свой репозиторий на GitHub: **Settings** -> **Secrets and variables** -> **Actions** -> **New repository secret**.

Добавь следующие секреты (названия должны совпадать точно):

### Доступ к серверу (Инфраструктура)
*   `SSH_HOST`: IP-адрес твоего дроплета (например, `123.123.123.123`).
*   `SSH_USER`: Имя пользователя (обычно `root`).
*   `SSH_KEY`: Содержимое твоего **приватного** SSH-ключа (обычно файл `~/.ssh/id_rsa`). Скопируй всё содержимое, включая `-----BEGIN RSA PRIVATE KEY-----`.

### База данных (SurrealDB)
*   `SURREAL_USER`: Логин администратора БД (например, `admin`).
*   `SURREAL_PASS`: Сложный пароль для БД.

### Сеть и SSL (Caddy)
*   `DOMAIN_NAME`: Твой домен или поддомен (например, `api.universities.kz`). **Важно:** A-запись домена должна указывать на IP дроплета.
*   `TLS_EMAIL`: Твоя почта (нужна Let's Encrypt для уведомлений о статусе сертификатов).

### Интеграция с данными (Data-as-Code)
*   `GH_PAT`: Твой Personal Access Token (PAT) с правами `repo` (чтение приватных репозиториев). **Важно:** В настройках GitHub Secrets назови его именно `GH_PAT`, так как префикс `GITHUB_` запрещён. Скрипт деплоя автоматически запишет его в `.env` как `GITHUB_TOKEN`.
*   `WEBHOOK_SECRET`: Любая длинная случайная строка. Должна совпадать с секретом, который ты укажешь в настройках вебхука репозитория с данными.

---

## 3. Переменные окружения (Environment Variables)

Некоторые переменные прописаны в `.github/workflows/deploy.yml` по умолчанию, но ты можешь их изменить там же:
*   `SURREAL_NS`: `main` (Namespace в SurrealDB).
*   `SURREAL_DB`: `main` (Название базы данных).
*   `GITHUB_REPO`: `universities-data` (Имя репозитория с данными).
*   `GITHUB_BRANCH`: `main` (Ветка для синхронизации).

## 4. Как запустить деплой

Просто сделай пуш в ветку `main`:
```bash
git add .
git commit -m "feat: setup deployment infrastructure"
git push origin main
```

После этого перейди во вкладку **Actions** в GitHub, чтобы следить за процессом. Если всё настроено верно, через пару минут твое API будет доступно по HTTPS на твоем домене!

## 5. Мониторинг на сервере

Если что-то пошло не так, зайди на дроплет по SSH и проверь логи:
```bash
cd ~/app
docker compose logs -f api      # Логи бэкенда
docker compose logs -f caddy    # Логи веб-сервера (SSL ошибки здесь)
docker compose logs -f surrealdb # Логи базы данных
```
