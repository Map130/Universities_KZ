package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// ──────────────────────────────────────────────────────────────
//  Edge: offers   (university ──offers──▶ specialty)
// ──────────────────────────────────────────────────────────────

type Offers struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`
	Out       surrealmodels.RecordID        `json:"out" cbor:"out" yaml:"out"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// OfferWithSpecialty — результат `SELECT * FROM offers WHERE in = $id FETCH out`.
type OfferWithSpecialty struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`
	Out       Specialty                     `json:"out" cbor:"out" yaml:"out"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Edge: ent_requirement   (university ──ent_requirement──▶ specialty_group)
// ──────────────────────────────────────────────────────────────

type EntRequirement struct {
	ID                *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In                surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`
	Out               surrealmodels.RecordID        `json:"out" cbor:"out" yaml:"out"`
	MinScore          int                           `json:"min_score" cbor:"min_score" yaml:"min_score"`
	LastYearThreshold int                           `json:"last_year_threshold" cbor:"last_year_threshold" yaml:"last_year_threshold"`
	CreatedAt         *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt         *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type EntRequirementWithGroup struct {
	ID                *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In                surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`
	Out               SpecialtyGroup                `json:"out" cbor:"out" yaml:"out"`
	MinScore          int                           `json:"min_score" cbor:"min_score" yaml:"min_score"`
	LastYearThreshold int                           `json:"last_year_threshold" cbor:"last_year_threshold" yaml:"last_year_threshold"`
	CreatedAt         *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt         *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Edge: requires   (specialty_group ──requires──▶ subject)
//  «Для поступления на группу ОП нужен этот предмет ЕНТ»
// ──────────────────────────────────────────────────────────────

// Requires — графовая связь между группой ОП и предметом ЕНТ.
type Requires struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`    // specialty_group:...
	Out       surrealmodels.RecordID        `json:"out" cbor:"out" yaml:"out"` // subject:...
	Priority  SubjectPriority               `json:"priority" cbor:"priority" yaml:"priority"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// RequiredSubject — результат `SELECT * FROM requires WHERE in = $group_id FETCH out`.
// Поле Out развёрнуто в полный объект Subject.
type RequiredSubject struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	In        surrealmodels.RecordID        `json:"in" cbor:"in" yaml:"in"`
	Out       Subject                       `json:"out" cbor:"out" yaml:"out"`
	Priority  SubjectPriority               `json:"priority" cbor:"priority" yaml:"priority"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// ──────────────────────────────────────────────────────────────
//  Входные данные для создания связей
// ──────────────────────────────────────────────────────────────

// CreateEntRequirementInput — параметры для создания связи university -> specialty_group.
type CreateEntRequirementInput struct {
	MinScore          int `json:"min_score" cbor:"min_score" yaml:"min_score"`
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold" yaml:"last_year_threshold"`
}

// CreateRequiresInput — параметры для создания связи specialty_group -> subject.
type CreateRequiresInput struct {
	Priority SubjectPriority `json:"priority" cbor:"priority" yaml:"priority"`
}
