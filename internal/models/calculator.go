package models

// CalculatorRequest — входные данные калькулятора поступления.
// Абитуриент указывает свой балл ЕНТ и два профильных предмета.
type CalculatorRequest struct {
	// Score — общий балл ЕНТ абитуриента (0–140).
	Score int `json:"score" query:"score"`

	// Subject1 — код первого профильного предмета (например, "math").
	// Соответствует полю code в таблице subject.
	Subject1 string `json:"subject1" query:"subject1"`

	// Subject2 — код второго профильного предмета (например, "physics").
	// Соответствует полю code в таблице subject.
	Subject2 string `json:"subject2" query:"subject2"`

	// City — фильтр по городу (опционально).
	City string `json:"city,omitempty" query:"city"`

	// Lang — язык ответа ("kz", "ru", "en"). По умолчанию "ru".
	Lang string `json:"lang,omitempty" query:"lang"`
}

// CalculatorRow — "сырая" строка из SurrealDB-запроса калькулятора.
// Маппится на результат SELECT с алиасами (uni_name, spec_code и т.д.).
// Используется только внутри репозитория для десериализации.
type CalculatorRow struct {
	// Поля из ent_requirement
	MinScore          int `json:"min_score" cbor:"min_score"`
	LastYearThreshold int `json:"last_year_threshold" cbor:"last_year_threshold"`

	// Алиасы из SELECT — университет
	UniName LocalizedName `json:"uni_name" cbor:"uni_name"`
	UniAbbr string        `json:"uni_abbr" cbor:"uni_abbr"`
	UniCity string        `json:"uni_city" cbor:"uni_city"`
	UniType string        `json:"uni_type" cbor:"uni_type"`
	UniLogo *string       `json:"uni_logo" cbor:"uni_logo"`

	// Алиасы из SELECT — группа ОП
	GroupCode string        `json:"group_code" cbor:"group_code"`
	GroupName LocalizedName `json:"group_name" cbor:"group_name"`
}

// CalculatorResult — один элемент результата калькулятора.
// Содержит вуз, группу ОП и условия поступления.
type CalculatorResult struct {
	University UniversityShort    `json:"university"`
	Group      GroupShort         `json:"group"`
	EntReq     EntRequirementInfo `json:"ent_req"`
	Grant      bool               `json:"grant"`
}

// UniversityShort — краткая информация о вузе для калькулятора.
type UniversityShort struct {
	Name    LocalizedName `json:"name"`
	Abbr    string        `json:"abbr"`
	City    string        `json:"city"`
	Type    string        `json:"type"`
	LogoURL *string       `json:"logo_url,omitempty"`
}

// GroupShort — краткая информация о группе ОП для калькулятора.
type GroupShort struct {
	Code string        `json:"code"`
	Name LocalizedName `json:"name"`
}

// EntRequirementInfo — условия поступления из связи ent_requirement.
type EntRequirementInfo struct {
	MinScore          int `json:"min_score"`
	LastYearThreshold int `json:"last_year_threshold"`
}

// CalculatorResponse — полный ответ калькулятора.
type CalculatorResponse struct {
	Score   int                `json:"score"`
	Results []CalculatorResult `json:"results"`
	Total   int                `json:"total"`
}

// ToResult преобразует "сырую" строку из БД в клиентский CalculatorResult.
// Поле Grant вычисляется по формуле: score >= last_year_threshold.
func (row *CalculatorRow) ToResult(score int) CalculatorResult {
	return CalculatorResult{
		University: UniversityShort{
			Name:    row.UniName,
			Abbr:    row.UniAbbr,
			City:    row.UniCity,
			Type:    row.UniType,
			LogoURL: row.UniLogo,
		},
		Group: GroupShort{
			Code: row.GroupCode,
			Name: row.GroupName,
		},
		EntReq: EntRequirementInfo{
			MinScore:          row.MinScore,
			LastYearThreshold: row.LastYearThreshold,
		},
		Grant: score >= row.LastYearThreshold,
	}
}
