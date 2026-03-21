package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// Specialty — специальность / образовательная программа.
// Соответствует таблице `specialty` в SurrealDB (SCHEMAFULL).
type Specialty struct {
	// ID — идентификатор записи в SurrealDB (например, specialty:xyz789).
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`

	// Уникальный код образовательной программы ("6B06101", "6B07201" и т. д.).
	Code string `json:"code" cbor:"code" yaml:"code"`

	// Название специальности на трёх языках (kz / ru / en).
	Name LocalizedName `json:"name" cbor:"name" yaml:"name"`

	// Группа образовательных программ — record link на specialty_group.
	// При обычном SELECT возвращается как RecordID.
	// При SELECT ... FETCH `group` — развёрнутый объект (см. SpecialtyExpanded).
	Group surrealmodels.RecordID `json:"group" cbor:"group" yaml:"group"`

	// Временные метки (заполняются SurrealDB автоматически).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// SpecialtyExpanded — специальность с развёрнутой группой ОП (FETCH `group`).
// Используется для API-ответов, где нужно показать и группу, и её предметы.
type SpecialtyExpanded struct {
	ID        *surrealmodels.RecordID       `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`
	Code      string                        `json:"code" cbor:"code" yaml:"code"`
	Name      LocalizedName                 `json:"name" cbor:"name" yaml:"name"`
	Group     SpecialtyGroup                `json:"group" cbor:"group" yaml:"group"`
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// SpecialtyFilters содержит параметры фильтрации для списка специальностей.
type SpecialtyFilters struct {
	Search    string // полнотекстовый поиск
	Lang      string // язык ("kz", "ru", "en"), по умолчанию "ru"
	GroupCode string // фильтр по коду группы ОП (например, "B057")
	Limit     int
	Offset    int
}
