#!/usr/bin/env bash
# ============================================================
#  seed.sh — заливает тестовые данные в SurrealDB
#
#  Использование:
#    chmod +x scripts/seed.sh
#    ./scripts/seed.sh
#
#  Переменные окружения (или дефолты для локальной разработки):
#    SURREAL_URL   — HTTP endpoint SurrealDB  (default: http://localhost:8000)
#    SURREAL_USER  — root user                (default: root)
#    SURREAL_PASS  — root password            (default: root)
#    SURREAL_NS    — namespace                (default: universities)
#    SURREAL_DB    — database                 (default: universities)
# ============================================================

set -euo pipefail

# ── Загрузка .env (если есть) ────────────────────────────────
# Подхватываем те же переменные, что использует бэкенд,
# чтобы seed гарантированно писал в тот же NS/DB.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -f "$PROJECT_ROOT/.env" ]]; then
    # Читаем .env, игнорируя комментарии и пустые строки.
    # export только те переменные, которые ещё не заданы в окружении.
    while IFS='=' read -r key value; do
        # Пропускаем комментарии и пустые строки
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        # Убираем пробелы вокруг ключа
        key=$(echo "$key" | xargs)
        # Убираем кавычки вокруг значения
        value=$(echo "$value" | sed -e "s/^['\"/]//" -e "s/['\"/]$//" | xargs)
        # Устанавливаем только если переменная ещё не задана
        if [[ -z "${!key:-}" ]]; then
            export "$key=$value"
        fi
    done < "$PROJECT_ROOT/.env"
fi

# ── Конфигурация ─────────────────────────────────────────────
# SURREAL_URL в .env может быть ws:// (для Go SDK), конвертируем в http:// для curl.
RAW_URL="${SURREAL_URL:-http://localhost:8000}"
URL=$(echo "$RAW_URL" | sed -e 's|^ws://|http://|' -e 's|^wss://|https://|' -e 's|/rpc$||')
USER="${SURREAL_USER:-root}"
PASS="${SURREAL_PASS:-root}"
NS="${SURREAL_NS:-universities}"
DB="${SURREAL_DB:-universities}"

# ── Цвета для вывода ────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ── Функция выполнения SurrealQL запроса ────────────────────
sql() {
    local query="$1"
    local description="${2:-}"

    if [[ -n "$description" ]]; then
        printf "${CYAN}▶ %s${NC}\n" "$description"
    fi

    local response
    response=$(curl -s -w "\n%{http_code}" \
        -X POST "${URL}/sql" \
        -H "Accept: application/json" \
        -H "surreal-ns: ${NS}" \
        -H "surreal-db: ${DB}" \
        -u "${USER}:${PASS}" \
        -d "$query" \
        2>&1)

    local http_code
    http_code=$(echo "$response" | tail -n1)
    local body
    body=$(echo "$response" | sed '$d')

    if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
        # Проверяем, есть ли ошибка внутри JSON-ответа SurrealDB
        if echo "$body" | grep -qi '"status":\s*"ERR"'; then
            printf "  ${RED}✗ SurrealDB error:${NC} %s\n" "$body"
            return 1
        fi
        printf "  ${GREEN}✓ OK${NC}\n"
    else
        printf "  ${RED}✗ HTTP %s:${NC} %s\n" "$http_code" "$body"
        return 1
    fi
}

echo ""
printf "${YELLOW}╔══════════════════════════════════════════════════╗${NC}\n"
printf "${YELLOW}║   🌱  Seed: тестовые данные UniversitiesKZ       ║${NC}\n"
printf "${YELLOW}╚══════════════════════════════════════════════════╝${NC}\n"
echo ""
printf "  URL:  %s\n" "$URL"
printf "  NS:   %s\n" "$NS"
printf "  DB:   %s\n" "$DB"
echo ""

# ── Проверка доступности SurrealDB ──────────────────────────
printf "${CYAN}▶ Проверка подключения к SurrealDB...${NC}\n"
if ! curl -sf "${URL}/health" > /dev/null 2>&1; then
    printf "  ${RED}✗ SurrealDB недоступен на %s${NC}\n" "$URL"
    printf "  Запусти: ${YELLOW}docker compose up -d${NC}\n"
    exit 1
fi
printf "  ${GREEN}✓ SurrealDB доступен${NC}\n"
echo ""

# ── Создание namespace и database (если не существуют) ───────
printf "${CYAN}▶ Создание namespace/database...${NC}\n"
INIT_RESP=$(curl -s -X POST "${URL}/sql" \
    -H "Accept: application/json" \
    -u "${USER}:${PASS}" \
    -d "DEFINE NAMESPACE IF NOT EXISTS ${NS}; USE NS ${NS}; DEFINE DATABASE IF NOT EXISTS ${DB};" \
    2>&1)

if echo "$INIT_RESP" | grep -qi '"status":\s*"ERR"'; then
    printf "  ${RED}✗ Не удалось создать NS/DB:${NC} %s\n" "$INIT_RESP"
    exit 1
fi
printf "  ${GREEN}✓ NS=%s  DB=%s${NC}\n" "$NS" "$DB"
echo ""

# ════════════════════════════════════════════════════════════
#  1. ПРЕДМЕТЫ ЕНТ (subjects)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 1. Предметы ЕНТ ──────────────────────────────${NC}\n"

sql "UPSERT subject:math SET
    code = 'math',
    name = {
        kz: 'Математика',
        ru: 'Математика',
        en: 'Mathematics'
    };" "Математика"

sql "UPSERT subject:physics SET
    code = 'physics',
    name = {
        kz: 'Физика',
        ru: 'Физика',
        en: 'Physics'
    };" "Физика"

sql "UPSERT subject:geography SET
    code = 'geography',
    name = {
        kz: 'География',
        ru: 'География',
        en: 'Geography'
    };" "География"

sql "UPSERT subject:foreign_lang SET
    code = 'foreign_lang',
    name = {
        kz: 'Шет тілі',
        ru: 'Иностранный язык',
        en: 'Foreign Language'
    };" "Иностранный язык"

echo ""

# ════════════════════════════════════════════════════════════
#  2. ГРУППЫ ОП (specialty_groups)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 2. Группы образовательных программ ───────────${NC}\n"

sql "UPSERT specialty_group:b057 SET
    code = 'B057',
    name = {
        kz: 'Ақпараттық технологиялар',
        ru: 'Информационные технологии',
        en: 'Information Technology'
    };" "B057 — Информационные технологии"

sql "UPSERT specialty_group:b058 SET
    code = 'B058',
    name = {
        kz: 'Коммуникациялар және коммуникациялық технологиялар',
        ru: 'Коммуникации и коммуникационные технологии',
        en: 'Communications and Communication Technologies'
    };" "B058 — Коммуникации и комм. технологии"

echo ""

# ════════════════════════════════════════════════════════════
#  3. СПЕЦИАЛЬНОСТИ (specialties)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 3. Специальности ─────────────────────────────${NC}\n"

sql "UPSERT specialty:is SET
    code = '6B06101',
    name = {
        kz: 'Ақпараттық жүйелер',
        ru: 'Информационные системы',
        en: 'Information Systems'
    },
    \`group\` = specialty_group:b057;" "6B06101 — Информационные системы"

sql "UPSERT specialty:se SET
    code = '6B06102',
    name = {
        kz: 'Бағдарламалық қамтамасыз ету',
        ru: 'Программная инженерия',
        en: 'Software Engineering'
    },
    \`group\` = specialty_group:b057;" "6B06102 — Программная инженерия"

sql "UPSERT specialty:cs SET
    code = '6B06103',
    name = {
        kz: 'Информатика',
        ru: 'Вычислительная техника и ПО',
        en: 'Computer Science'
    },
    \`group\` = specialty_group:b057;" "6B06103 — Вычислительная техника и ПО"

sql "UPSERT specialty:telecom SET
    code = '6B06201',
    name = {
        kz: 'Телекоммуникациялар жүйелері',
        ru: 'Системы телекоммуникаций',
        en: 'Telecommunication Systems'
    },
    \`group\` = specialty_group:b058;" "6B06201 — Системы телекоммуникаций"

echo ""

# ════════════════════════════════════════════════════════════
#  4. ВУЗЫ (universities)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 4. Вузы ──────────────────────────────────────${NC}\n"

sql "UPSERT university:muit SET
    name = {
        kz: 'Халықаралық ақпараттық технологиялар университеті',
        ru: 'Международный университет информационных технологий',
        en: 'International University of Information Technology'
    },
    abbr = 'МУИТ',
    city = 'Алматы',
    type = 'private',
    website = 'https://iitu.edu.kz',
    description = 'Один из ведущих IT-вузов Казахстана, основан в 2009 году.',
    custom_css = '';" "МУИТ (Алматы, частный)"

sql "UPSERT university:kaznu SET
    name = {
        kz: 'Әл-Фараби атындағы Қазақ ұлттық университеті',
        ru: 'Казахский национальный университет имени аль-Фараби',
        en: 'Al-Farabi Kazakh National University'
    },
    abbr = 'КазНУ',
    city = 'Алматы',
    type = 'public',
    website = 'https://kaznu.kz',
    description = 'Крупнейший вуз Казахстана, основан в 1934 году. Входит в QS World University Rankings.',
    custom_css = '';" "КазНУ (Алматы, государственный)"

sql "UPSERT university:sdu SET
    name = {
        kz: 'Сүлейман Демирел атындағы университет',
        ru: 'Университет имени Сулеймана Демиреля',
        en: 'Suleyman Demirel University'
    },
    abbr = 'SDU',
    city = 'Алматы',
    type = 'private',
    website = 'https://sdu.edu.kz',
    description = 'Частный университет в Каскелене, сильные IT и бизнес программы.',
    custom_css = '';" "SDU (Алматы, частный)"

echo ""

# ════════════════════════════════════════════════════════════
#  5. СВЯЗИ: requires (specialty_group → subject)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 5. Требования к предметам (requires) ─────────${NC}\n"

# Очищаем старые связи requires перед пересозданием
sql "DELETE FROM requires WHERE in = specialty_group:b057 OR in = specialty_group:b058;" "Очистка старых requires"

# B057 — Информационные технологии: Математика (профиль) + Физика (второй)
sql "RELATE specialty_group:b057->requires->subject:math SET
    priority = 1;" "B057 → Математика (профильный)"

sql "RELATE specialty_group:b057->requires->subject:physics SET
    priority = 2;" "B057 → Физика (второй)"

# B058 — Коммуникации: Физика (профиль) + Математика (второй)
sql "RELATE specialty_group:b058->requires->subject:physics SET
    priority = 1;" "B058 → Физика (профильный)"

sql "RELATE specialty_group:b058->requires->subject:math SET
    priority = 2;" "B058 → Математика (второй)"

echo ""

# ════════════════════════════════════════════════════════════
#  6. СВЯЗИ: offers (university → specialty)
# ════════════════════════════════════════════════════════════

printf "${YELLOW}── 6. Предложения вузов (offers) ─────────────────${NC}\n"

# Очищаем старые связи offers перед пересозданием
sql "DELETE FROM offers WHERE in IN [university:muit, university:kaznu, university:sdu];" "Очистка старых offers"

# ── МУИТ ──
sql "RELATE university:muit->offers->specialty:is SET
    grant_count = 50,
    quota_grant_count = 10,
    tuition_fee = 1800000,
    min_score = 75,
    last_year_threshold = 118;" "МУИТ → Информационные системы"

sql "RELATE university:muit->offers->specialty:se SET
    grant_count = 40,
    quota_grant_count = 8,
    tuition_fee = 1900000,
    min_score = 80,
    last_year_threshold = 122;" "МУИТ → Программная инженерия"

sql "RELATE university:muit->offers->specialty:telecom SET
    grant_count = 20,
    quota_grant_count = 5,
    tuition_fee = 1700000,
    min_score = 70,
    last_year_threshold = 105;" "МУИТ → Системы телекоммуникаций"

# ── КазНУ ──
sql "RELATE university:kaznu->offers->specialty:is SET
    grant_count = 100,
    quota_grant_count = 25,
    tuition_fee = 1200000,
    min_score = 65,
    last_year_threshold = 110;" "КазНУ → Информационные системы"

sql "RELATE university:kaznu->offers->specialty:se SET
    grant_count = 80,
    quota_grant_count = 20,
    tuition_fee = 1200000,
    min_score = 70,
    last_year_threshold = 115;" "КазНУ → Программная инженерия"

sql "RELATE university:kaznu->offers->specialty:cs SET
    grant_count = 60,
    quota_grant_count = 15,
    tuition_fee = 1200000,
    min_score = 65,
    last_year_threshold = 108;" "КазНУ → Вычислительная техника и ПО"

sql "RELATE university:kaznu->offers->specialty:telecom SET
    grant_count = 45,
    quota_grant_count = 10,
    tuition_fee = 1100000,
    min_score = 60,
    last_year_threshold = 100;" "КазНУ → Системы телекоммуникаций"

# ── SDU ──
sql "RELATE university:sdu->offers->specialty:is SET
    grant_count = 35,
    quota_grant_count = 7,
    tuition_fee = 2200000,
    min_score = 80,
    last_year_threshold = 125;" "SDU → Информационные системы"

sql "RELATE university:sdu->offers->specialty:se SET
    grant_count = 30,
    quota_grant_count = 6,
    tuition_fee = 2400000,
    min_score = 85,
    last_year_threshold = 130;" "SDU → Программная инженерия"

echo ""

# ════════════════════════════════════════════════════════════
#  ИТОГО
# ════════════════════════════════════════════════════════════

printf "${GREEN}╔══════════════════════════════════════════════════╗${NC}\n"
printf "${GREEN}║   ✅  Seed завершён!                             ║${NC}\n"
printf "${GREEN}╠══════════════════════════════════════════════════╣${NC}\n"
printf "${GREEN}║   Предметов ЕНТ:     4                           ║${NC}\n"
printf "${GREEN}║   Групп ОП:          2                           ║${NC}\n"
printf "${GREEN}║   Специальностей:    4                           ║${NC}\n"
printf "${GREEN}║   Вузов:             3  (МУИТ, КазНУ, SDU)       ║${NC}\n"
printf "${GREEN}║   Связей requires:   4                           ║${NC}\n"
printf "${GREEN}║   Связей offers:     9                           ║${NC}\n"
printf "${GREEN}╚══════════════════════════════════════════════════╝${NC}\n"
echo ""
printf "  Проверь: ${CYAN}curl %s/sql -u %s:%s -H 'surreal-ns: %s' -H 'surreal-db: %s' -d 'SELECT * FROM university'${NC}\n" \
    "$URL" "$USER" "$PASS" "$NS" "$DB"
echo ""
