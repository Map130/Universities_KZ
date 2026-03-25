// Package models описывает доменные структуры проекта UniversitiesKZ.
// Все структуры используют теги json (для HTTP-ответов) и cbor (для SurrealDB).
package models

// LocalizedName хранит строку на трёх языках (kz, ru, en).
// Соответствует полю TYPE object в SurrealDB-схеме.
type LocalizedName struct {
	KZ string `json:"kz" cbor:"kz" yaml:"kz"`
	RU string `json:"ru" cbor:"ru" yaml:"ru"`
	EN string `json:"en" cbor:"en" yaml:"en"`
}

// UniversityType — тип вуза (государственный / частный).
type UniversityType string

const (
	UniversityTypePublic  UniversityType = "public"
	UniversityTypePrivate UniversityType = "private"
)

// IsValid проверяет допустимость значения (совпадает с ASSERT в схеме).
func (t UniversityType) IsValid() bool {
	return t == UniversityTypePublic || t == UniversityTypePrivate
}

// SubjectPriority — приоритет предмета ЕНТ (1 = профильный, 2 = второй).
type SubjectPriority int

const (
	SubjectPriorityProfile   SubjectPriority = 1
	SubjectPrioritySecondary SubjectPriority = 2
)

// IsValid проверяет допустимость значения (совпадает с ASSERT в схеме).
func (p SubjectPriority) IsValid() bool {
	return p == SubjectPriorityProfile || p == SubjectPrioritySecondary
}
