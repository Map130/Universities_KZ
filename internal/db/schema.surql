-- ============================================================
--  UniversitiesKZ — SurrealDB Schema  (Schemafull)
--  Граф: university ──offers──▶ specialty ──requires──▶ subject
-- ============================================================


-- ─────────────────────────────────────────────────────────────
--  0.  Анализатор для полнотекстового поиска
--      Поддержка кириллицы и латиницы, ngram для prefix-match
-- ─────────────────────────────────────────────────────────────

DEFINE ANALYZER IF NOT EXISTS name_analyzer
    TOKENIZERS blank, class
    FILTERS lowercase, edgengram(2, 15);


-- ═════════════════════════════════════════════════════════════
--  1.  TABLE: university  —  Вуз
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE university SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD name           ON TABLE university TYPE object;
DEFINE FIELD name.kz        ON TABLE university TYPE string;
DEFINE FIELD name.ru        ON TABLE university TYPE string;
DEFINE FIELD name.en        ON TABLE university TYPE string;

-- Аббревиатура ("МУИТ", "КазНУ", "SDU") --
DEFINE FIELD abbr           ON TABLE university TYPE string;

-- Город --
DEFINE FIELD city           ON TABLE university TYPE string;

-- Тип вуза: государственный / частный --
DEFINE FIELD type           ON TABLE university TYPE string
    ASSERT $value IN ["public", "private"];

-- URL логотипа (MinIO / S3) --
DEFINE FIELD logo_url       ON TABLE university TYPE option<string>;

-- Официальный сайт --
DEFINE FIELD website        ON TABLE university TYPE option<string>;

-- Свободное описание --
DEFINE FIELD description    ON TABLE university TYPE option<string>;

-- Временные метки --
DEFINE FIELD created_at     ON TABLE university TYPE datetime
    DEFAULT time::now();
DEFINE FIELD updated_at     ON TABLE university TYPE datetime
    DEFAULT time::now();

-- Индекс по городу (обычный, для фильтрации) --
DEFINE INDEX idx_university_city
    ON TABLE university
    FIELDS city;

-- Полнотекстовые индексы для поиска по каждому языку --
DEFINE INDEX idx_university_name_kz_search
    ON TABLE university
    FIELDS name.kz
    SEARCH ANALYZER name_analyzer BM25;

DEFINE INDEX idx_university_name_ru_search
    ON TABLE university
    FIELDS name.ru
    SEARCH ANALYZER name_analyzer BM25;

DEFINE INDEX idx_university_name_en_search
    ON TABLE university
    FIELDS name.en
    SEARCH ANALYZER name_analyzer BM25;

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT ev_university_updated
    ON TABLE university
    WHEN $event = "UPDATE"
    THEN {
        UPDATE university SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  2.  TABLE: specialty  —  Специальность / Образовательная
--      программа
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE specialty SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Уникальный код программы ("B057", "B058" и т. д.) --
DEFINE FIELD code           ON TABLE specialty TYPE string;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD name           ON TABLE specialty TYPE object;
DEFINE FIELD name.kz        ON TABLE specialty TYPE string;
DEFINE FIELD name.ru        ON TABLE specialty TYPE string;
DEFINE FIELD name.en        ON TABLE specialty TYPE string;

-- Группа образовательных программ --
-- "group" — зарезервированное слово, экранируем бэктиками --
DEFINE FIELD `group`        ON TABLE specialty TYPE string;

-- Временные метки --
DEFINE FIELD created_at     ON TABLE specialty TYPE datetime
    DEFAULT time::now();
DEFINE FIELD updated_at     ON TABLE specialty TYPE datetime
    DEFAULT time::now();

-- Уникальный индекс на код специальности --
DEFINE INDEX idx_specialty_code
    ON TABLE specialty
    FIELDS code UNIQUE;

-- Полнотекстовые индексы для поиска по каждому языку --
DEFINE INDEX idx_specialty_name_kz_search
    ON TABLE specialty
    FIELDS name.kz
    SEARCH ANALYZER name_analyzer BM25;

DEFINE INDEX idx_specialty_name_ru_search
    ON TABLE specialty
    FIELDS name.ru
    SEARCH ANALYZER name_analyzer BM25;

DEFINE INDEX idx_specialty_name_en_search
    ON TABLE specialty
    FIELDS name.en
    SEARCH ANALYZER name_analyzer BM25;

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT ev_specialty_updated
    ON TABLE specialty
    WHEN $event = "UPDATE"
    THEN {
        UPDATE specialty SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  3.  TABLE: subject  —  Предмет ЕНТ
--      (Математика, Физика, География, …)
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE subject SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD name           ON TABLE subject TYPE object;
DEFINE FIELD name.kz        ON TABLE subject TYPE string;
DEFINE FIELD name.ru        ON TABLE subject TYPE string;
DEFINE FIELD name.en        ON TABLE subject TYPE string;

-- Временные метки --
DEFINE FIELD created_at     ON TABLE subject TYPE datetime
    DEFAULT time::now();
DEFINE FIELD updated_at     ON TABLE subject TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT ev_subject_updated
    ON TABLE subject
    WHEN $event = "UPDATE"
    THEN {
        UPDATE subject SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  4.  EDGE: offers
--      university ──offers──▶ specialty
--      «Вуз предлагает специальность с такими условиями»
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE offers SCHEMAFULL
    TYPE RELATION FROM university TO specialty
    PERMISSIONS
        FOR select, create, update, delete FULL;

DEFINE FIELD in                 ON TABLE offers TYPE record<university>;
DEFINE FIELD out                ON TABLE offers TYPE record<specialty>;

-- Количество грантов на текущий год приёма --
DEFINE FIELD grant_count        ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Количество грантов по сельской квоте --
DEFINE FIELD quota_grant_count  ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Стоимость обучения (тенге / год), не может быть отрицательной --
DEFINE FIELD tuition_fee        ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Минимальный балл ЕНТ для участия в конкурсе (0–140) --
DEFINE FIELD min_score          ON TABLE offers TYPE int
    ASSERT $value >= 0 AND $value <= 140;

-- Проходной балл гранта прошлого года (0–140) --
DEFINE FIELD last_year_threshold ON TABLE offers TYPE int
    ASSERT $value >= 0 AND $value <= 140;

-- Временные метки --
DEFINE FIELD created_at         ON TABLE offers TYPE datetime
    DEFAULT time::now();
DEFINE FIELD updated_at         ON TABLE offers TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT ev_offers_updated
    ON TABLE offers
    WHEN $event = "UPDATE"
    THEN {
        UPDATE offers SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  5.  EDGE: requires
--      specialty ──requires──▶ subject
--      «Для поступления на специальность нужен этот предмет ЕНТ»
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE requires SCHEMAFULL
    TYPE RELATION FROM specialty TO subject
    PERMISSIONS
        FOR select, create, update, delete FULL;

DEFINE FIELD in                 ON TABLE requires TYPE record<specialty>;
DEFINE FIELD out                ON TABLE requires TYPE record<subject>;

-- Приоритет предмета: 1 = профильный, 2 = второй --
DEFINE FIELD priority           ON TABLE requires TYPE int
    ASSERT $value IN [1, 2];

-- Временные метки --
DEFINE FIELD created_at         ON TABLE requires TYPE datetime
    DEFAULT time::now();
DEFINE FIELD updated_at         ON TABLE requires TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT ev_requires_updated
    ON TABLE requires
    WHEN $event = "UPDATE"
    THEN {
        UPDATE requires SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  6.  ПРИМЕРЫ ЗАПРОСОВ  (не выполняются, только справка)
-- ═════════════════════════════════════════════════════════════

/*
 * ── Создание вуза ───────────────────────────────────────────
 *
 *   CREATE university SET
 *       name   = { kz: "М.Әуезов атындағы ОҚМУ", ru: "ЮКГУ им. М. Ауэзова", en: "SKSU" },
 *       abbr   = "ЮКГУ",
 *       city   = "Шымкент",
 *       type   = "public",
 *       website = "https://ukgu.kz";
 *
 *
 * ── Создание специальности ──────────────────────────────────
 *
 *   CREATE specialty SET
 *       code    = "B057",
 *       name    = { kz: "Ақпараттық технологиялар", ru: "Информационные технологии", en: "Information Technology" },
 *       `group` = "B05 - Естественные науки";
 *
 *
 * ── Создание предмета ЕНТ ──────────────────────────────────
 *
 *   CREATE subject SET
 *       name = { kz: "Математика", ru: "Математика", en: "Mathematics" };
 *
 *
 * ── Связь «вуз предлагает специальность» ────────────────────
 *
 *   RELATE university:abc123 -> offers -> specialty:xyz789 SET
 *       grant_count        = 50,
 *       quota_grant_count  = 10,
 *       tuition_fee        = 1200000,
 *       min_score          = 50,
 *       last_year_threshold = 102;
 *
 *
 * ── Связь «специальность требует предмет» ───────────────────
 *
 *   RELATE specialty:xyz789 -> requires -> subject:math SET
 *       priority = 1;
 *
 *
 * ── Графовый запрос: все специальности вуза с предметами ─────
 *
 *   SELECT
 *       name.ru                                  AS university,
 *       ->offers.out.name.ru                     AS specialties,
 *       ->offers.out->requires.out.name.ru       AS ent_subjects
 *   FROM university
 *   WHERE city = "Алматы";
 *
 *
 * ── Полнотекстовый поиск вуза ───────────────────────────────
 *
 *   SELECT * FROM university
 *   WHERE name.ru @@ 'технический';
 *
 *
 * ── Найти вузы, которые принимают по предмету «Физика» ──────
 *
 *   SELECT
 *       <-requires<-specialty<-offers<-university.name.ru AS universities
 *   FROM subject
 *   WHERE name.ru = "Физика";
 */
