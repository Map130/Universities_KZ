package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// Specialty — специальность / образовательная программа.
// Соответствует таблице `specialty` в SurrealDB (SCHEMAFULL).
type Specialty struct {
	// ID — идентификатор записи в SurrealDB (например, specialty:xyz789).
	// При создании может быть nil — SurrealDB сгенерирует автоматически.
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// Уникальный код образовательной программы ("B057", "B058" и т. д.).
	Code string `json:"code" cbor:"code"`

	// Название специальности на трёх языках (kz / ru / en).
	Name LocalizedName `json:"name" cbor:"name"`

	// Группа образовательных программ (например, "B05 - Естественные науки").
	// В SurrealDB-схеме поле экранировано бэктиками (`group`), т. к. это
	// зарезервированное слово, но в Go/CBOR это обычное поле.
	Group string `json:"group" cbor:"group"`

	// Временные метки (заполняются SurrealDB автоматически).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}

// SpecialtyFilters содержит параметры фильтрации для списка специальностей.
type SpecialtyFilters struct {
	// Search — строка полнотекстового поиска (оператор @@ в SurrealQL).
	Search string

	// Lang — язык поиска ("kz", "ru", "en"). По умолчанию "ru".
	Lang string

	// Group — фильтр по группе образовательных программ. Пустая строка = без фильтра.
	Group string

	// Limit — максимальное число записей (0 = без ограничения).
	Limit int

	// Offset — смещение для пагинации.
	Offset int
}
