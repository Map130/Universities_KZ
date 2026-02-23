package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// ──────────────────────────────────────────────────────────────
//  Edge: offers   (university ──offers──▶ specialty)
//  «Вуз предлагает специальность с конкретными условиями приёма»
// ──────────────────────────────────────────────────────────────

// Offers — графовая связь между вузом и специальностью.
// Соответствует edge-таблице `offers` в SurrealDB (TYPE RELATION FROM university TO specialty).
type Offers struct {
	// ID — идентификатор связи (например, offers:abc123).
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// In — ссылка на вуз (university:...).
	In surrealmodels.RecordID `json:"in" cbor:"in"`

	// Out — ссылка на специальность (specialty:...).
	Out surrealmodels.RecordID `json:"out" cbor:"out"`

	// GrantCount — количество грантов на текущий год приёма.
	GrantCount int `json:"grant_count" cbor:"grant_count"`

	// QuotaGrantCount — количество грантов по сельской квоте.
	QuotaGrantCount int `json:"quota_grant_count" cbor:"quota_grant_count"`

	// TuitionFee — стоимость обучения (тенге/год). Не может быть отрицательной.
	TuitionFee int `json:"tuition_fee" cbor:"tuition_fee"`

	// MinScore — минимальный балл ЕНТ для участия в конкурсе (0–140).
	MinScore int `json:"min_score" cbor:"min_score"`

	// LastYearThreshold — проходной балл гранта прошлого года (0–140).
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold"`

	// Временные метки.
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// OfferWithSpecialty — результат графового запроса
// `SELECT * FROM offers WHERE in = $id FETCH out`.
// Поле Out (обычно RecordID) развёрнуто в полный объект Specialty
// благодаря ключевому слову FETCH.
type OfferWithSpecialty struct {
	// ID — идентификатор связи.
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// In — ссылка на вуз (остаётся RecordID).
	In surrealmodels.RecordID `json:"in" cbor:"in"`

	// Out — развёрнутый объект специальности (FETCH out).
	Out Specialty `json:"out" cbor:"out"`

	// Данные приёма.
	GrantCount        int `json:"grant_count" cbor:"grant_count"`
	QuotaGrantCount   int `json:"quota_grant_count" cbor:"quota_grant_count"`
	TuitionFee        int `json:"tuition_fee" cbor:"tuition_fee"`
	MinScore          int `json:"min_score" cbor:"min_score"`
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold"`

	// Временные метки.
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Edge: requires   (specialty ──requires──▶ subject)
//  «Для поступления на специальность нужен этот предмет ЕНТ»
// ──────────────────────────────────────────────────────────────

// Requires — графовая связь между специальностью и предметом ЕНТ.
// Соответствует edge-таблице `requires` в SurrealDB (TYPE RELATION FROM specialty TO subject).
type Requires struct {
	// ID — идентификатор связи (например, requires:abc123).
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// In — ссылка на специальность (specialty:...).
	In surrealmodels.RecordID `json:"in" cbor:"in"`

	// Out — ссылка на предмет ЕНТ (subject:...).
	Out surrealmodels.RecordID `json:"out" cbor:"out"`

	// Priority — приоритет предмета: 1 = профильный, 2 = второй.
	Priority SubjectPriority `json:"priority" cbor:"priority"`

	// Временные метки.
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// RequiredSubject — результат графового запроса
// `SELECT * FROM requires WHERE in = $id FETCH out`.
// Поле Out развёрнуто в полный объект Subject (FETCH out).
type RequiredSubject struct {
	// ID — идентификатор связи.
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// In — ссылка на специальность (остаётся RecordID).
	In surrealmodels.RecordID `json:"in" cbor:"in"`

	// Out — развёрнутый объект предмета ЕНТ (FETCH out).
	Out Subject `json:"out" cbor:"out"`

	// Priority — приоритет предмета: 1 = профильный, 2 = второй.
	Priority SubjectPriority `json:"priority" cbor:"priority"`

	// Временные метки.
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Входные данные для создания связей
// ──────────────────────────────────────────────────────────────

// CreateOfferInput — параметры для создания связи university -> specialty.
// Используется в репозитории; ID записей передаются отдельно.
type CreateOfferInput struct {
	GrantCount        int `json:"grant_count" cbor:"grant_count"`
	QuotaGrantCount   int `json:"quota_grant_count" cbor:"quota_grant_count"`
	TuitionFee        int `json:"tuition_fee" cbor:"tuition_fee"`
	MinScore          int `json:"min_score" cbor:"min_score"`
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold"`
}

// CreateRequiresInput — параметры для создания связи specialty -> subject.
type CreateRequiresInput struct {
	Priority SubjectPriority `json:"priority" cbor:"priority"`
}
