#!/usr/bin/env bash
# ============================================================
#  stress.sh — стресс-тест бэкенда UniversitiesKZ (60 секунд)
#
#  Долбит все GET-эндпоинты параллельно через loopback.
#  Щадит Ryzen 5 5650U (12 воркеров), но бэку будет больно.
#
#  Использование:
#    chmod +x scripts/stress.sh
#    ./scripts/stress.sh
#
#  Переменные окружения:
#    BASE_URL     — адрес бэкенда   (default: http://127.0.0.1:8080)
#    DURATION     — длительность, с  (default: 60)
#    CONCURRENCY  — воркеров на эндпоинт (default: 2)
# ============================================================

set -uo pipefail

# ── Загрузка .env (если есть) ────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -f "$PROJECT_ROOT/.env" ]]; then
    while IFS='=' read -r key value; do
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        key=$(echo "$key" | xargs)
        value=$(echo "$value" | sed -e "s/^['\"/]//" -e "s/['\"/]$//" | xargs)
        if [[ -z "${!key:-}" ]]; then
            export "$key=$value"
        fi
    done < "$PROJECT_ROOT/.env"
fi

# ── Конфигурация ─────────────────────────────────────────────
APP_PORT="${APP_PORT:-8080}"
BASE="${BASE_URL:-http://127.0.0.1:${APP_PORT}}"
DURATION="${DURATION:-80}"
CONCURRENCY="${CONCURRENCY:-2}"

# ── Цвета ────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# ── Временные файлы для сбора метрик ────────────────────────
TMPDIR_STRESS=$(mktemp -d)

cleanup() {
    rm -rf "$TMPDIR_STRESS"
}

# ── Проверка доступности бэкенда ────────────────────────────
printf "${CYAN}▶ Проверка бэкенда на %s ...${NC}\n" "$BASE"
if ! curl -sf --max-time 3 "${BASE}/health" > /dev/null 2>&1; then
    printf "  ${RED}✗ Бэкенд недоступен на %s${NC}\n" "$BASE"
    printf "  Запусти сервер и попробуй снова.\n"
    cleanup
    exit 1
fi
printf "  ${GREEN}✓ Бэкенд жив${NC}\n\n"

# ── Preflight: проверяем, есть ли данные (seed) ──────────────
printf "${CYAN}▶ Проверка наличия данных (seed)...${NC}\n"
UNI_CHECK=$(curl -s --max-time 3 "${BASE}/api/v1/universities" 2>/dev/null || echo "")
if echo "$UNI_CHECK" | grep -q '"id"'; then
    printf "  ${GREEN}✓ Данные есть${NC}\n\n"
    HAS_SEED=true
else
    printf "  ${YELLOW}⚠  База пустая или бэк не вернул данные.${NC}\n"
    printf "  Ответ: %s\n" "$(echo "$UNI_CHECK" | head -c 200)"
    printf "  Запусти сначала: ${CYAN}./scripts/seed.sh${NC}\n"
    printf "  Стресс-тест будет бить только по спискам (без :id эндпоинтов).\n\n"
    HAS_SEED=false
fi

# ── Эндпоинты для теста ─────────────────────────────────────
# Микс лёгких и тяжёлых запросов с обходом графа.
# Эндпоинты с :id добавляем только если seed залит.
ENDPOINTS=(
    "/health"
    "/api/v1/universities"
    "/api/v1/groups"
    "/api/v1/specialties"
    "/api/v1/subjects"
)

if [[ "$HAS_SEED" == true ]]; then
    ENDPOINTS+=(
        "/api/v1/universities/muit"
        "/api/v1/universities/kaznu"
        "/api/v1/universities/sdu"
        "/api/v1/groups/b057"
        "/api/v1/groups/b058"
    )
fi

TOTAL_WORKERS=$(( ${#ENDPOINTS[@]} * CONCURRENCY ))

# Предсоздаём файлы метрик для каждого воркера
WORKER_NUM=0
for endpoint in "${ENDPOINTS[@]}"; do
    for ((c = 0; c < CONCURRENCY; c++)); do
        : > "${TMPDIR_STRESS}/worker_${WORKER_NUM}.csv"
        WORKER_NUM=$(( WORKER_NUM + 1 ))
    done
done

# ── Баннер ───────────────────────────────────────────────────
printf "${RED}${BOLD}"
printf "╔══════════════════════════════════════════════════════╗\n"
printf "║   🔥  STRESS TEST — UniversitiesKZ Backend            ║\n"
printf "╠══════════════════════════════════════════════════════╣\n"
printf "║   Длительность:    %-4s сек                          ║\n" "$DURATION"
printf "║   Эндпоинтов:      %-4s                              ║\n" "${#ENDPOINTS[@]}"
printf "║   Воркеров/эндп:   %-4s                              ║\n" "$CONCURRENCY"
printf "║   Всего воркеров:  %-4s                              ║\n" "$TOTAL_WORKERS"
printf "╚══════════════════════════════════════════════════════╝\n"
printf "${NC}\n"

printf "${DIM}  Нажми Ctrl+C чтобы остановить раньше.${NC}\n\n"

# ── Воркер ───────────────────────────────────────────────────
# Каждый воркер долбит один эндпоинт в тугом цикле,
# пишет время ответа и HTTP-код в файл метрик.
# Внутри воркера отключаем set -e, чтобы curl-ошибки не убивали процесс.
worker() {
    set +e
    local endpoint="$1"
    local worker_id="$2"
    local url="${BASE}${endpoint}"
    local metrics_file="${TMPDIR_STRESS}/worker_${worker_id}.csv"
    local deadline=$(( $(date +%s) + DURATION ))

    while [[ $(date +%s) -lt $deadline ]]; do
        result=$(curl -s -o /dev/null \
            -w "%{http_code} %{time_total}" \
            --max-time 10 \
            "$url" 2>/dev/null) || result="000 10.000"

        echo "${endpoint} ${result}" >> "$metrics_file"
    done
}

# ── Запуск воркеров ──────────────────────────────────────────
printf "${YELLOW}▶ Запуск %d воркеров на %d эндпоинтов...${NC}\n\n" "$TOTAL_WORKERS" "${#ENDPOINTS[@]}"

PIDS=()
WORKER_NUM=0

for endpoint in "${ENDPOINTS[@]}"; do
    for ((c = 0; c < CONCURRENCY; c++)); do
        worker "$endpoint" "$WORKER_NUM" &
        PIDS+=($!)
        WORKER_NUM=$(( WORKER_NUM + 1 ))
    done
done

# ── Прогресс-бар ────────────────────────────────────────────
START_TIME=$(date +%s)
while true; do
    NOW=$(date +%s)
    ELAPSED=$(( NOW - START_TIME ))
    REMAINING=$(( DURATION - ELAPSED ))

    if [[ $REMAINING -le 0 ]]; then
        break
    fi

    # Считаем текущий RPS по количеству строк в метриках
    TOTAL_REQS=0
    for f in "${TMPDIR_STRESS}"/worker_*.csv; do
        [[ -f "$f" ]] || continue
        COUNT=$(wc -l < "$f" | tr -d ' ')
        TOTAL_REQS=$(( TOTAL_REQS + COUNT ))
    done

    if [[ $ELAPSED -gt 0 ]]; then
        RPS=$(( TOTAL_REQS / ELAPSED ))
    else
        RPS=0
    fi

    # Прогресс-бар
    PCT=$(( ELAPSED * 100 / DURATION ))
    BAR_LEN=30
    FILLED=$(( PCT * BAR_LEN / 100 ))
    EMPTY=$(( BAR_LEN - FILLED ))

    BAR=""
    for ((i = 0; i < FILLED; i++)); do BAR+="█"; done
    BAR_E=""
    for ((i = 0; i < EMPTY; i++)); do BAR_E+="░"; done

    printf "\r  ${CYAN}[%s%s]${NC} %3d%% │ %3ds left │ ${GREEN}%d reqs${NC} │ ${YELLOW}~%d rps${NC}   " \
        "$BAR" "$BAR_E" "$PCT" "$REMAINING" "$TOTAL_REQS" "$RPS"

    sleep 1
done

# Финальная полоска
FULL_BAR=""
for ((i = 0; i < 30; i++)); do FULL_BAR+="█"; done
printf "\r  ${CYAN}[%s]${NC} 100%% │   0s left │ finishing...                    \n\n" "$FULL_BAR"

# ── Ждём завершения всех воркеров ───────────────────────────
printf "${DIM}  Ожидание завершения воркеров...${NC}\n"
for pid in "${PIDS[@]}"; do
    wait "$pid" 2>/dev/null || true
done

# ── Подсчёт метрик ──────────────────────────────────────────
printf "\n${BOLD}${GREEN}"
printf "╔══════════════════════════════════════════════════════╗\n"
printf "║   📊  РЕЗУЛЬТАТЫ                                     ║\n"
printf "╚══════════════════════════════════════════════════════╝\n"
printf "${NC}\n"

# Объединяем все метрики
MERGED="${TMPDIR_STRESS}/all.csv"
cat "${TMPDIR_STRESS}"/worker_*.csv > "$MERGED" 2>/dev/null || true

TOTAL=$(wc -l < "$MERGED" | tr -d ' ')
END_TIME=$(date +%s)
ACTUAL_DURATION=$(( END_TIME - START_TIME ))

if [[ $ACTUAL_DURATION -gt 0 ]]; then
    FINAL_RPS=$(( TOTAL / ACTUAL_DURATION ))
else
    FINAL_RPS=$TOTAL
fi

# Подсчёт по статус-кодам через awk (надёжнее grep -c, без "0\n0" багов)
read -r OK_2XX ERR_4XX ERR_5XX TIMEOUTS <<< "$(awk '
{
    code = $2 + 0
    if      (code >= 200 && code < 300) ok++
    else if (code >= 400 && code < 500) e4++
    else if (code >= 500 && code < 600) e5++
    else                                 tout++
}
END {
    printf "%d %d %d %d", ok+0, e4+0, e5+0, tout+0
}
' "$MERGED")"

# Латентность (в миллисекундах)
if [[ $TOTAL -gt 0 ]]; then
    LATENCIES_FILE="${TMPDIR_STRESS}/latencies_ms.txt"
    awk '{printf "%.0f\n", $3 * 1000}' "$MERGED" | sort -n > "$LATENCIES_FILE"

    AVG_MS=$(awk '{ sum += $1; n++ } END { if (n>0) printf "%.0f", sum/n; else print 0 }' "$LATENCIES_FILE")
    MIN_MS=$(head -1 "$LATENCIES_FILE")
    MAX_MS=$(tail -1 "$LATENCIES_FILE")

    # Перцентили
    P50_LINE=$(( (TOTAL * 50 + 99) / 100 ))
    P95_LINE=$(( (TOTAL * 95 + 99) / 100 ))
    P99_LINE=$(( (TOTAL * 99 + 99) / 100 ))
    P50_MS=$(sed -n "${P50_LINE}p" "$LATENCIES_FILE")
    P95_MS=$(sed -n "${P95_LINE}p" "$LATENCIES_FILE")
    P99_MS=$(sed -n "${P99_LINE}p" "$LATENCIES_FILE")
else
    AVG_MS=0; MIN_MS=0; MAX_MS=0; P50_MS=0; P95_MS=0; P99_MS=0
fi

printf "  ${BOLD}Общие:${NC}\n"
printf "    Длительность:        %s сек\n" "$ACTUAL_DURATION"
printf "    Всего запросов:      ${BOLD}%s${NC}\n" "$TOTAL"
printf "    RPS (ср.):           ${BOLD}${YELLOW}%s${NC}\n" "$FINAL_RPS"
echo ""

printf "  ${BOLD}Статус-коды:${NC}\n"
printf "    ${GREEN}2xx OK:${NC}              %s\n" "$OK_2XX"
printf "    ${YELLOW}4xx Client Error:${NC}    %s\n" "$ERR_4XX"
printf "    ${RED}5xx Server Error:${NC}    %s\n" "$ERR_5XX"
printf "    ${RED}Timeout/Fail:${NC}        %s\n" "$TIMEOUTS"
echo ""
printf "  ${BOLD}Латентность (мс):${NC}\n"
printf "    min:    %s\n" "$MIN_MS"
printf "    avg:    %s\n" "$AVG_MS"
printf "    p50:    %s\n" "$P50_MS"
printf "    p95:    ${YELLOW}%s${NC}\n" "$P95_MS"
printf "    p99:    ${RED}%s${NC}\n" "$P99_MS"
printf "    max:    ${RED}%s${NC}\n" "$MAX_MS"

# Ошибки > 1%?
if [[ $TOTAL -gt 0 ]]; then
    ERROR_TOTAL=$(( ERR_5XX + TIMEOUTS ))
    ERROR_PCT=$(( ERROR_TOTAL * 100 / TOTAL ))
    echo ""
    if [[ $ERROR_PCT -gt 1 ]]; then
        printf "  ${RED}⚠  Процент ошибок (5xx+timeout): %d%% — бэк не выдержал нагрузку${NC}\n" "$ERROR_PCT"
    else
        printf "  ${GREEN}✓  Серверных ошибок: %d (%.1f%%) — бэк держится${NC}\n" "$ERROR_TOTAL" "$(awk "BEGIN { printf \"%.1f\", $ERROR_TOTAL * 100.0 / $TOTAL }")"
    fi
    if [[ $ERR_4XX -gt 0 ]]; then
        ERR_4XX_PCT=$(awk "BEGIN { printf \"%.1f\", $ERR_4XX * 100.0 / $TOTAL }")
        printf "  ${YELLOW}⚠  4xx ошибок: %d (%s%%) — проверь seed и эндпоинты${NC}\n" "$ERR_4XX" "$ERR_4XX_PCT"
    fi
fi

# ── Разбивка по эндпоинтам ──────────────────────────────────
echo ""
printf "  ${BOLD}По эндпоинтам:${NC}\n"
printf "  %-35s %7s %7s %7s %7s %7s\n" "ENDPOINT" "REQS" "AVG ms" "P95 ms" "ERR 4xx" "ERR 5xx"
printf "  %-35s %7s %7s %7s %7s %7s\n" "-----------------------------------" "-------" "-------" "-------" "-------" "-------"

for endpoint in "${ENDPOINTS[@]}"; do
    EP_FILE="${TMPDIR_STRESS}/ep_$(echo "$endpoint" | tr '/' '_').csv"
    grep "^${endpoint} " "$MERGED" > "$EP_FILE" 2>/dev/null || true

    EP_COUNT=$(wc -l < "$EP_FILE" | tr -d ' ')

    if [[ "$EP_COUNT" -gt 0 ]]; then
        # Подсчёт через awk — одна команда, никаких проблем с grep -c
        read -r EP_AVG EP_4XX EP_5XX <<< "$(awk '
        {
            ms = $3 * 1000
            sum += ms
            n++
            code = $2 + 0
            if (code >= 400 && code < 500) e4++
            else if (code >= 500 && code < 600) e5++
        }
        END {
            printf "%.0f %d %d", (n>0 ? sum/n : 0), e4+0, e5+0
        }
        ' "$EP_FILE")"

        EP_P95_LINE=$(( (EP_COUNT * 95 + 99) / 100 ))
        EP_P95=$(awk '{printf "%.0f\n", $3 * 1000}' "$EP_FILE" | sort -n | sed -n "${EP_P95_LINE}p")
        EP_P95="${EP_P95:-0}"
    else
        EP_AVG=0; EP_P95=0; EP_4XX=0; EP_5XX=0
    fi

    # Цветовая индикация
    if [[ "$EP_5XX" -gt 0 ]]; then
        C5="${RED}"
    else
        C5="${GREEN}"
    fi
    if [[ "$EP_4XX" -gt 0 ]]; then
        C4="${YELLOW}"
    else
        C4="${GREEN}"
    fi

    printf "  %-35s %7s %7s %7s ${C4}%7s${NC} ${C5}%7s${NC}\n" \
        "$endpoint" "$EP_COUNT" "$EP_AVG" "$EP_P95" "$EP_4XX" "$EP_5XX"
done

echo ""

# ── Очистка ──────────────────────────────────────────────────
cleanup
printf "${DIM}  Временные файлы очищены.${NC}\n\n"
