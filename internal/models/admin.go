// Package models описывает доменные структуры проекта UniversitiesKZ.
package models

import (
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// AllowedAdmin — запись из таблицы allowed_admins (whitelist).
// Содержит email администратора, которому разрешён доступ через Google OAuth.
type AllowedAdmin struct {
	// ID — идентификатор записи в SurrealDB (например, allowed_admins:abc123).
	ID *surrealmodels.RecordID `json:"id,omitempty" cbor:"id,omitempty"`

	// Email — email администратора (уникальный, нижний регистр).
	Email string `json:"email" cbor:"email"`

	// Name — имя администратора (необязательное, для удобства идентификации).
	Name *string `json:"name,omitempty" cbor:"name,omitempty"`

	// CreatedAt — временная метка создания записи (заполняется SurrealDB).
	CreatedAt *surrealmodels.CustomDateTime `json:"created_at,omitempty" cbor:"created_at,omitempty"`
}
