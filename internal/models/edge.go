package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// ──────────────────────────────────────────────────────────────
//  Edge: offers   (university ──offers──▶ specialty)
// ──────────────────────────────────────────────────────────────

type Offers struct {
	ID                *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty"`
	In                surrealmodels.RecordID        `json:"in" cbor:"in"`
	Out               surrealmodels.RecordID        `json:"out" cbor:"out"`
	GrantCount        int                           `json:"grant_count" cbor:"grant_count"`
	QuotaGrantCount   int                           `json:"quota_grant_count" cbor:"quota_grant_count"`
	TuitionFee        int                           `json:"tuition_fee" cbor:"tuition_fee"`
	MinScore          int                           `json:"min_score" cbor:"min_score"`
	LastYearThreshold int                           `json:"last_year_threshold" cbor:"last_year_threshold"`
	CreatedAt         *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt         *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// OfferWithSpecialty — результат `SELECT * FROM offers WHERE in = $id FETCH out`.
type OfferWithSpecialty struct {
	ID                *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty"`
	In                surrealmodels.RecordID        `json:"in" cbor:"in"`
	Out               Specialty                     `json:"out" cbor:"out"`
	GrantCount        int                           `json:"grant_count" cbor:"grant_count"`
	QuotaGrantCount   int                           `json:"quota_grant_count" cbor:"quota_grant_count"`
	TuitionFee        int                           `json:"tuition_fee" cbor:"tuition_fee"`
	MinScore          int                           `json:"min_score" cbor:"min_score"`
	LastYearThreshold int                           `json:"last_year_threshold" cbor:"last_year_threshold"`
	CreatedAt         *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt         *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Edge: requires   (specialty_group ──requires──▶ subject)
//  «Для поступления на группу ОП нужен этот предмет ЕНТ»
// ──────────────────────────────────────────────────────────────

// Requires — графовая связь между группой ОП и предметом ЕНТ.
type Requires struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in"`   // specialty_group:...
	Out       surrealmodels.RecordID        `json:"out" cbor:"out"` // subject:...
	Priority  SubjectPriority               `json:"priority" cbor:"priority"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// RequiredSubject — результат `SELECT * FROM requires WHERE in = $group_id FETCH out`.
// Поле Out развёрнуто в полный объект Subject.
type RequiredSubject struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in"`
	Out       Subject                       `json:"out" cbor:"out"`
	Priority  SubjectPriority               `json:"priority" cbor:"priority"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Входные данные для создания связей
// ──────────────────────────────────────────────────────────────

// CreateOfferInput — параметры для создания связи university -> specialty.
type CreateOfferInput struct {
	GrantCount        int `json:"grant_count" cbor:"grant_count"`
	QuotaGrantCount   int `json:"quota_grant_count" cbor:"quota_grant_count"`
	TuitionFee        int `json:"tuition_fee" cbor:"tuition_fee"`
	MinScore          int `json:"min_score" cbor:"min_score"`
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold"`
}

// CreateRequiresInput — параметры для создания связи specialty_group -> subject.
type CreateRequiresInput struct {
	Priority SubjectPriority `json:"priority" cbor:"priority"`
}
