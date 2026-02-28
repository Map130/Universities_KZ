-- ============================================================
--  UniversitiesKZ — SurrealDB Schema  (Schemafull)
--  Граф: university ──offers──▶ specialty ──group──▶ specialty_group ──requires──▶ subject
--
--  Все DEFINE используют OVERWRITE для идемпотентности миграций
--  (SurrealDB 3.0 требует OVERWRITE или IF NOT EXISTS).
-- ============================================================


-- ─────────────────────────────────────────────────────────────
--  0.  Анализатор для полнотекстового поиска
--      Поддержка кириллицы и латиницы, ngram для prefix-match
-- ─────────────────────────────────────────────────────────────

DEFINE ANALYZER OVERWRITE name_analyzer
    TOKENIZERS blank, class
    FILTERS lowercase, edgengram(2, 15);


-- ═════════════════════════════════════════════════════════════
--  1.  TABLE: university  —  Вуз
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE university SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD OVERWRITE name           ON TABLE university TYPE object;
DEFINE FIELD OVERWRITE name.kz        ON TABLE university TYPE string;
DEFINE FIELD OVERWRITE name.ru        ON TABLE university TYPE string;
DEFINE FIELD OVERWRITE name.en        ON TABLE university TYPE string;

-- Аббревиатура ("МУИТ", "КазНУ", "SDU") --
DEFINE FIELD OVERWRITE abbr           ON TABLE university TYPE string;

-- Город --
DEFINE FIELD OVERWRITE city           ON TABLE university TYPE string;

-- Тип вуза: государственный / частный --
DEFINE FIELD OVERWRITE type           ON TABLE university TYPE string
    ASSERT $value IN ["public", "private"];

-- URL логотипа (MinIO / S3) --
DEFINE FIELD OVERWRITE logo_url       ON TABLE university TYPE option<string>;

-- Официальный сайт --
DEFINE FIELD OVERWRITE website        ON TABLE university TYPE option<string>;

-- Свободное описание --
DEFINE FIELD OVERWRITE description    ON TABLE university TYPE option<string>;

-- Кастомный CSS для премиум-вузов.
-- Хранит произвольный CSS, который инжектится на публичной странице вуза
-- внутри изолированного контейнера (см. комментарии по безопасности в хендлерах).
-- По умолчанию — пустая строка (не premium).
DEFINE FIELD OVERWRITE custom_css     ON TABLE university TYPE string
    DEFAULT "";

-- Временные метки --
DEFINE FIELD OVERWRITE created_at     ON TABLE university TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at     ON TABLE university TYPE datetime
    DEFAULT time::now();

-- Индекс по городу (обычный, для фильтрации) --
DEFINE INDEX OVERWRITE idx_university_city
    ON TABLE university
    FIELDS city;

-- Полнотекстовые индексы для поиска по каждому языку --
-- Синтаксис SurrealDB 3.0: FULLTEXT ANALYZER (вместо SEARCH ANALYZER)
DEFINE INDEX OVERWRITE idx_university_name_kz_search
    ON TABLE university
    FIELDS name.kz
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_university_name_ru_search
    ON TABLE university
    FIELDS name.ru
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_university_name_en_search
    ON TABLE university
    FIELDS name.en
    FULLTEXT ANALYZER name_analyzer BM25;

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_university_updated
    ON TABLE university
    WHEN $event = "UPDATE"
    THEN {
        UPDATE university SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  2.  TABLE: specialty_group  —  Группа образовательных программ
--      Требования к предметам ЕНТ задаются на уровне группы.
--      Пример: "B057" — Информационные технологии → Математика + Физика
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE specialty_group SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Уникальный код группы ОП ("B057", "B058" и т. д.) --
DEFINE FIELD OVERWRITE code           ON TABLE specialty_group TYPE string;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD OVERWRITE name           ON TABLE specialty_group TYPE object;
DEFINE FIELD OVERWRITE name.kz        ON TABLE specialty_group TYPE string;
DEFINE FIELD OVERWRITE name.ru        ON TABLE specialty_group TYPE string;
DEFINE FIELD OVERWRITE name.en        ON TABLE specialty_group TYPE string;

-- Временные метки --
DEFINE FIELD OVERWRITE created_at     ON TABLE specialty_group TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at     ON TABLE specialty_group TYPE datetime
    DEFAULT time::now();

-- Уникальный индекс на код группы --
DEFINE INDEX OVERWRITE idx_specialty_group_code
    ON TABLE specialty_group
    FIELDS code UNIQUE;

-- Полнотекстовые индексы для поиска по каждому языку --
DEFINE INDEX OVERWRITE idx_specialty_group_name_kz_search
    ON TABLE specialty_group
    FIELDS name.kz
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_specialty_group_name_ru_search
    ON TABLE specialty_group
    FIELDS name.ru
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_specialty_group_name_en_search
    ON TABLE specialty_group
    FIELDS name.en
    FULLTEXT ANALYZER name_analyzer BM25;

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_specialty_group_updated
    ON TABLE specialty_group
    WHEN $event = "UPDATE"
    THEN {
        UPDATE specialty_group SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  3.  TABLE: specialty  —  Специальность / Образовательная
--      программа
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE specialty SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Уникальный код программы ("6B06101", "6B07201" и т. д.) --
DEFINE FIELD OVERWRITE code           ON TABLE specialty TYPE string;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD OVERWRITE name           ON TABLE specialty TYPE object;
DEFINE FIELD OVERWRITE name.kz        ON TABLE specialty TYPE string;
DEFINE FIELD OVERWRITE name.ru        ON TABLE specialty TYPE string;
DEFINE FIELD OVERWRITE name.en        ON TABLE specialty TYPE string;

-- Группа образовательных программ — record link на specialty_group --
-- "group" — зарезервированное слово, экранируем бэктиками --
DEFINE FIELD OVERWRITE `group`        ON TABLE specialty TYPE record<specialty_group>;

-- Временные метки --
DEFINE FIELD OVERWRITE created_at     ON TABLE specialty TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at     ON TABLE specialty TYPE datetime
    DEFAULT time::now();

-- Уникальный индекс на код специальности --
DEFINE INDEX OVERWRITE idx_specialty_code
    ON TABLE specialty
    FIELDS code UNIQUE;

-- Полнотекстовые индексы для поиска по каждому языку --
DEFINE INDEX OVERWRITE idx_specialty_name_kz_search
    ON TABLE specialty
    FIELDS name.kz
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_specialty_name_ru_search
    ON TABLE specialty
    FIELDS name.ru
    FULLTEXT ANALYZER name_analyzer BM25;

DEFINE INDEX OVERWRITE idx_specialty_name_en_search
    ON TABLE specialty
    FIELDS name.en
    FULLTEXT ANALYZER name_analyzer BM25;

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_specialty_updated
    ON TABLE specialty
    WHEN $event = "UPDATE"
    THEN {
        UPDATE specialty SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  4.  TABLE: subject  —  Предмет ЕНТ
--      (Математика, Физика, География, …)
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE subject SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Название (локализация: kz / ru / en) --
DEFINE FIELD OVERWRITE name           ON TABLE subject TYPE object;
DEFINE FIELD OVERWRITE name.kz        ON TABLE subject TYPE string;
DEFINE FIELD OVERWRITE name.ru        ON TABLE subject TYPE string;
DEFINE FIELD OVERWRITE name.en        ON TABLE subject TYPE string;

-- Временные метки --
DEFINE FIELD OVERWRITE created_at     ON TABLE subject TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at     ON TABLE subject TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_subject_updated
    ON TABLE subject
    WHEN $event = "UPDATE"
    THEN {
        UPDATE subject SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  5.  EDGE: offers
--      university ──offers──▶ specialty
--      «Вуз предлагает специальность с такими условиями»
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE offers SCHEMAFULL
    TYPE RELATION FROM university TO specialty
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Поля in/out создаются автоматически из TYPE RELATION FROM...TO...

-- Количество грантов на текущий год приёма --
DEFINE FIELD OVERWRITE grant_count        ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Количество грантов по сельской квоте --
DEFINE FIELD OVERWRITE quota_grant_count  ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Стоимость обучения (тенге / год), не может быть отрицательной --
DEFINE FIELD OVERWRITE tuition_fee        ON TABLE offers TYPE int
    ASSERT $value >= 0;

-- Минимальный балл ЕНТ для участия в конкурсе (0–140) --
DEFINE FIELD OVERWRITE min_score          ON TABLE offers TYPE int
    ASSERT $value >= 0 AND $value <= 140;

-- Проходной балл гранта прошлого года (0–140) --
DEFINE FIELD OVERWRITE last_year_threshold ON TABLE offers TYPE int
    ASSERT $value >= 0 AND $value <= 140;

-- Временные метки --
DEFINE FIELD OVERWRITE created_at         ON TABLE offers TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at         ON TABLE offers TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_offers_updated
    ON TABLE offers
    WHEN $event = "UPDATE"
    THEN {
        UPDATE offers SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  6.  EDGE: requires
--      specialty_group ──requires──▶ subject
--      «Для поступления на группу ОП нужен этот предмет ЕНТ»
--
--      Требования к предметам задаются на уровне группы ОП,
--      а не на уровне отдельной специальности. Это исключает
--      дублирование: изменил требования один раз на группу →
--      автоматически применится к всем специальностям группы.
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE requires SCHEMAFULL
    TYPE RELATION FROM specialty_group TO subject
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Поля in/out создаются автоматически из TYPE RELATION FROM...TO...

-- Приоритет предмета: 1 = профильный, 2 = второй --
DEFINE FIELD OVERWRITE priority           ON TABLE requires TYPE int
    ASSERT $value IN [1, 2];

-- Временные метки --
DEFINE FIELD OVERWRITE created_at         ON TABLE requires TYPE datetime
    DEFAULT time::now();
DEFINE FIELD OVERWRITE updated_at         ON TABLE requires TYPE datetime
    DEFAULT time::now();

-- Событие: автообновление updated_at при UPDATE --
DEFINE EVENT OVERWRITE ev_requires_updated
    ON TABLE requires
    WHEN $event = "UPDATE"
    THEN {
        UPDATE requires SET updated_at = time::now() WHERE id = $after.id;
    };


-- ═════════════════════════════════════════════════════════════
--  7.  TABLE: allowed_admins  —  Whitelist email-адресов
--      администраторов, которым разрешён доступ через Google OAuth.
-- ═════════════════════════════════════════════════════════════

DEFINE TABLE OVERWRITE allowed_admins SCHEMAFULL
    PERMISSIONS
        FOR select, create, update, delete FULL;

-- Email администратора (уникальный, нижний регистр) --
DEFINE FIELD OVERWRITE email          ON TABLE allowed_admins TYPE string
    ASSERT string::is_email($value);

-- Имя администратора (необязательное, для удобства идентификации) --
DEFINE FIELD OVERWRITE name           ON TABLE allowed_admins TYPE option<string>;

-- Временные метки --
DEFINE FIELD OVERWRITE created_at     ON TABLE allowed_admins TYPE datetime
    DEFAULT time::now();

-- Уникальный индекс на email — гарантирует отсутствие дублей --
DEFINE INDEX OVERWRITE idx_allowed_admins_email
    ON TABLE allowed_admins
    FIELDS email UNIQUE;


-- ═════════════════════════════════════════════════════════════
--  8.  ПРИМЕРЫ ЗАПРОСОВ  (не выполняются, только справка)
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
 * ── Создание группы ОП ──────────────────────────────────────
 *
 *   CREATE specialty_group SET
 *       code = "B057",
 *       name = { kz: "Ақпараттық технологиялар", ru: "Информационные технологии", en: "Information Technology" };
 *
 *
 * ── Создание специальности (с record link на группу) ────────
 *
 *   CREATE specialty SET
 *       code    = "6B06101",
 *       name    = { kz: "Ақпараттық жүйелер", ru: "Информационные системы", en: "Information Systems" },
 *       `group` = specialty_group:abc123;
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
 * ── Связь «группа ОП требует предмет» ───────────────────────
 *
 *   RELATE specialty_group:abc123 -> requires -> subject:math SET
 *       priority = 1;
 *
 *
 * ── Графовый запрос: все специальности вуза с предметами ─────
 *
 *   SELECT
 *       name.ru                                                AS university,
 *       ->offers.out.name.ru                                   AS specialties,
 *       ->offers.out.`group`->requires.out.name.ru             AS ent_subjects
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
 *   (обратный обход через группы ОП)
 *
 *   LET $group_ids = (SELECT VALUE in FROM requires WHERE out = subject:physics);
 *   LET $spec_ids  = (SELECT VALUE id FROM specialty WHERE `group` IN $group_ids);
 *   LET $uni_ids   = array::distinct((SELECT VALUE in FROM offers WHERE out IN $spec_ids));
 *   SELECT * FROM university WHERE id IN $uni_ids ORDER BY name.ru ASC;
 */
