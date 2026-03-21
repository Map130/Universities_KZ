package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// SpecialtyGroup — группа образовательных программ.
// Именно к группе привязаны требования к предметам ЕНТ
// (связка из двух профильных предметов).
// Соответствует таблице `specialty_group` в SurrealDB (SCHEMAFULL).
// Пример: "B057" — Информационные технологии → Математика + Физика.
type SpecialtyGroup struct {
	// ID — идентификатор записи (например, specialty_group:abc123).
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`

	// Уникальный код группы ОП ("B057", "B058" и т. д.).
	Code string `json:"code" cbor:"code" yaml:"code"`

	// Название группы на трёх языках (kz / ru / en).
	Name LocalizedName `json:"name" cbor:"name" yaml:"name"`

	// Временные метки (заполняются SurrealDB автоматически).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// SpecialtyGroupFilters содержит параметры фильтрации для списка групп ОП.
type SpecialtyGroupFilters struct {
	// Search — строка полнотекстового поиска (оператор @@ в SurrealQL).
	Search string

	// Lang — язык поиска ("kz", "ru", "en"). По умолчанию "ru".
	Lang string

	// Limit — максимальное число записей (0 = без ограничения).
	Limit int

	// Offset — смещение для пагинации.
	Offset int
}
