// Package models — Swagger-specific response/request types.
// These types exist solely for swag annotation purposes and
// are not used in business logic. They mirror the real types
// but replace SurrealDB-specific fields (RecordID, CustomDateTime)
// with plain strings so that swag can generate a clean OpenAPI spec.
package models

// ────────────────────────────────────────────────────────────────────────────
//  Generic response wrappers
// ────────────────────────────────────────────────────────────────────────────

// ErrorResponse — стандартный ответ при ошибке.
// @Description Стандартный ответ при ошибке
type ErrorResponse struct {
	Error string `json:"error" example:"record not found"`
}

// HealthResponse — ответ эндпоинта /health.
// @Description Ответ healthcheck-эндпоинта
type HealthResponse struct {
	Status string `json:"status" example:"online"`
	DB     string `json:"db" example:"connected"`
}

// ────────────────────────────────────────────────────────────────────────────
//  Swagger-safe mirrors of domain models
// ────────────────────────────────────────────────────────────────────────────

// SwaggerLocalizedName — название на трёх языках.
// @Description Локализованное название (kz / ru / en)
type SwaggerLocalizedName struct {
	KZ string `json:"kz" example:"Ақпараттық технологиялар"`
	RU string `json:"ru" example:"Информационные технологии"`
	EN string `json:"en" example:"Information Technologies"`
}

// SwaggerUniversity — вуз (Swagger-safe версия без SurrealDB типов).
// @Description Университет Казахстана
type SwaggerUniversity struct {
	ID          string               `json:"id" example:"university:abc123"`
	Name        SwaggerLocalizedName `json:"name"`
	Abbr        string               `json:"abbr" example:"МУИТ"`
	City        string               `json:"city" example:"Алматы"`
	Type        string               `json:"type" example:"public" enums:"public,private"`
	LogoURL     *string              `json:"logo_url,omitempty" example:"http://localhost:9000/logos/abc.png"`
	Website     *string              `json:"website,omitempty" example:"https://muit.edu.kz"`
	Description *string              `json:"description,omitempty" example:"Ведущий IT-вуз Казахстана"`
	CustomCSS   string               `json:"custom_css" example:""`
	CreatedAt   string               `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt   string               `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// SwaggerSpecialtyGroup — группа образовательных программ (Swagger-safe).
// @Description Группа образовательных программ
type SwaggerSpecialtyGroup struct {
	ID        string               `json:"id" example:"specialty_group:b057"`
	Code      string               `json:"code" example:"B057"`
	Name      SwaggerLocalizedName `json:"name"`
	CreatedAt string               `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt string               `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// SwaggerSpecialty — специальность / образовательная программа (Swagger-safe).
// @Description Специальность / образовательная программа
type SwaggerSpecialty struct {
	ID        string               `json:"id" example:"specialty:xyz789"`
	Code      string               `json:"code" example:"6B06101"`
	Name      SwaggerLocalizedName `json:"name"`
	Group     string               `json:"group" example:"specialty_group:b057"`
	CreatedAt string               `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt string               `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// SwaggerSubject — предмет ЕНТ (Swagger-safe).
// @Description Предмет ЕНТ
type SwaggerSubject struct {
	ID        string               `json:"id" example:"subject:math"`
	Name      SwaggerLocalizedName `json:"name"`
	CreatedAt string               `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt string               `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// ────────────────────────────────────────────────────────────────────────────
//  Composite / nested response types
// ────────────────────────────────────────────────────────────────────────────

// SwaggerOfferWithSpecialty — связь university→specialty с развёрнутой специальностью.
// @Description Предложение вуза (связь offers) с развёрнутой специальностью
type SwaggerOfferWithSpecialty struct {
	ID                string           `json:"id" example:"offers:abc123"`
	In                string           `json:"in" example:"university:abc123"`
	Out               SwaggerSpecialty `json:"out"`
	GrantCount        int              `json:"grant_count" example:"25"`
	QuotaGrantCount   int              `json:"quota_grant_count" example:"5"`
	TuitionFee        int              `json:"tuition_fee" example:"1500000"`
	MinScore          int              `json:"min_score" example:"80"`
	LastYearThreshold int              `json:"last_year_threshold" example:"85"`
	CreatedAt         string           `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt         string           `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// SwaggerUniversityDetail — вуз со списком предлагаемых специальностей.
// @Description Детальная информация о вузе с его специальностями
type SwaggerUniversityDetail struct {
	University SwaggerUniversity           `json:"university"`
	Offers     []SwaggerOfferWithSpecialty `json:"offers"`
}

// SwaggerRequiredSubject — связь requires с развёрнутым предметом.
// @Description Требуемый предмет ЕНТ для группы ОП
type SwaggerRequiredSubject struct {
	ID        string         `json:"id" example:"requires:abc123"`
	In        string         `json:"in" example:"specialty_group:b057"`
	Out       SwaggerSubject `json:"out"`
	Priority  int            `json:"priority" example:"1" enums:"1,2"`
	CreatedAt string         `json:"created_at,omitempty" example:"2025-01-15T10:30:00Z"`
	UpdatedAt string         `json:"updated_at,omitempty" example:"2025-01-15T10:30:00Z"`
}

// SwaggerGroupWithSubjects — ответ GET /groups/:id (группа + предметы).
// @Description Группа ОП с привязанными предметами ЕНТ
type SwaggerGroupWithSubjects struct {
	Group    SwaggerSpecialtyGroup    `json:"group"`
	Subjects []SwaggerRequiredSubject `json:"subjects"`
}
