package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// University — вуз Казахстана.
// Соответствует таблице `university` в SurrealDB (SCHEMAFULL).
type University struct {
	// ID — идентификатор записи в SurrealDB (например, university:abc123).
	// При создании может быть nil — SurrealDB сгенерирует автоматически.
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty" yaml:"id,omitempty"`

	// Название вуза на трёх языках (kz / ru / en).
	Name LocalizedName `json:"name" cbor:"name" yaml:"name"`

	// Аббревиатура ("МУИТ", "КазНУ", "SDU").
	Abbr string `json:"abbr" cbor:"abbr" yaml:"abbr"`

	// Город, в котором расположен вуз.
	City string `json:"city" cbor:"city" yaml:"city"`

	// Тип вуза: "public" (государственный) или "private" (частный).
	Type UniversityType `json:"type" cbor:"type" yaml:"type"`

	// URL логотипа (MinIO / S3). Может отсутствовать.
	LogoURL *string `json:"logo_url,omitempty" cbor:"logo_url,omitempty" yaml:"logo_url,omitempty"`

	// Официальный сайт вуза. Может отсутствовать.
	Website *string `json:"website,omitempty" cbor:"website,omitempty" yaml:"website,omitempty"`

	// Свободное описание вуза. Может отсутствовать.
	Description *string `json:"description,omitempty" cbor:"description,omitempty" yaml:"description,omitempty"`

	// Кастомный CSS для премиум-вузов.
	// Инжектится на публичной странице вуза внутри изолированного контейнера.
	// Пустая строка означает отсутствие кастомного оформления.
	CustomCSS string `json:"custom_css" cbor:"custom_css" yaml:"custom_css"`

	// Временные метки (заполняются SurrealDB автоматически).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// UniversityFilters содержит параметры фильтрации для списка вузов.
type UniversityFilters struct {
	// City — фильтр по городу (точное совпадение). Пустая строка = без фильтра.
	City string

	// Type — фильтр по типу вуза ("public" / "private"). Пустая строка = без фильтра.
	Type UniversityType

	// Search — строка полнотекстового поиска (оператор @@ в SurrealQL).
	Search string

	// Lang — язык поиска ("kz", "ru", "en"). По умолчанию "ru".
	Lang string

	// Limit — максимальное число записей (0 = без ограничения).
	Limit int

	// Offset — смещение для пагинации.
	Offset int
}

// UniversityDetail — развёрнутое представление вуза со списком
// предлагаемых специальностей и требований к ЕНТ.
type UniversityDetail struct {
	University      University                `json:"university"`
	EntRequirements []EntRequirementWithGroup `json:"ent_requirements"`
	Offers          []OfferWithSpecialty      `json:"offers"`
}
