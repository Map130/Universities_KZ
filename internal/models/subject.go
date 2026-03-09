package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// Subject — предмет ЕНТ (Единое национальное тестирование).
// Соответствует таблице `subject` в SurrealDB (SCHEMAFULL).
// Примеры: Математика, Физика, География, История Казахстана и т. д.
type Subject struct {
	// ID — идентификатор записи в SurrealDB (например, subject:math).
	// При создании может быть nil — SurrealDB сгенерирует автоматически.
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// Уникальный код предмета ("math", "physics", "geography" и т. д.).
	// Используется фронтом как value в <select> и в параметрах калькулятора.
	Code string `json:"code" cbor:"code"`

	// Название предмета на трёх языках (kz / ru / en).
	Name LocalizedName `json:"name" cbor:"name"`

	// Временные метки (заполняются SurrealDB автоматически).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
	UpdatedAt *surrealmodels.CustomDateTime `json:"updated_at,omitempty" cbor:"updated_at,omitempty"`
}
