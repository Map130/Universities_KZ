package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/models"
)

// CalculatorRepository описывает контракт для калькулятора поступления.
type CalculatorRepository interface {
	// Calculate возвращает список доступных вузов и специальностей
	// по баллу ЕНТ и двум профильным предметам абитуриента.
	Calculate(ctx context.Context, req models.CalculatorRequest) ([]models.CalculatorResult, error)
}

// surrealCalculatorRepo — реализация CalculatorRepository поверх SurrealDB.
type surrealCalculatorRepo struct {
	pool *db.Pool
}

// NewCalculatorRepository создаёт репозиторий калькулятора.
func NewCalculatorRepository(pool *db.Pool) CalculatorRepository {
	return &surrealCalculatorRepo{pool: pool}
}

// calculatorBaseQuery — единый графовый запрос калькулятора.
//
// Логика:
//  1. LET $s1, $s2 — находим предметы по уникальному коду (code).
//  2. LET $groups  — группы ОП, которые требуют ОБА предмета (HAVING count() = 2).
//  3. SELECT       — требования ЕНТ (ent_requirement), где min_score <= балл абитуриента,
//     с развёрнутыми данными вуза и группы через graph traversal.
//
// Placeholder {{CITY_FILTER}} заменяется на фильтр по городу при необходимости.
const calculatorBaseQuery = `
    LET $s1 = (SELECT VALUE id FROM subject WHERE code = $subject1 LIMIT 1);
    LET $s2 = (SELECT VALUE id FROM subject WHERE code = $subject2 LIMIT 1);

    LET $g1 = (SELECT VALUE in FROM requires WHERE out IN $s1);
    LET $g2 = (SELECT VALUE in FROM requires WHERE out IN $s2);

    LET $groups = array::intersect($g1, $g2);

    SELECT
        min_score,
        last_year_threshold,
        in.name       AS uni_name,
        in.abbr       AS uni_abbr,
        in.city       AS uni_city,
        in.type       AS uni_type,
        in.logo_url   AS uni_logo,
        out.code      AS group_code,
        out.name      AS group_name
    FROM ent_requirement
    WHERE out IN $groups
      AND min_score <= $score
      {{CITY_FILTER}}
    ORDER BY min_score ASC;
`

// ---------------------------------------------------------------------------
//  Calculate
// ---------------------------------------------------------------------------

func (r *surrealCalculatorRepo) Calculate(ctx context.Context, req models.CalculatorRequest) ([]models.CalculatorResult, error) {
	conn := r.pool.Get()

	vars := map[string]any{
		"subject1": req.Subject1,
		"subject2": req.Subject2,
		"score":    req.Score,
	}

	// Подставляем фильтр по городу, если указан.
	cityFilter := ""
	if req.City != "" {
		cityFilter = "AND in.city = $city"
		vars["city"] = req.City
	}

	query := strings.Replace(calculatorBaseQuery, "{{CITY_FILTER}}", cityFilter, 1)

	// LET создаёт несколько statements. surrealdb.Query возвращает
	// результат каждого statement-а. Нам нужен последний (SELECT).
	// С generic параметром []models.CalculatorRow мы получаем
	// *[]surrealdb.QueryResult[[]models.CalculatorRow].
	results, err := surrealdb.Query[[]models.CalculatorRow](ctx, conn, query, vars)
	if err != nil {
		return nil, fmt.Errorf("calculator.Calculate: query: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return []models.CalculatorResult{}, nil
	}

	// Последний элемент — результат финального SELECT.
	last := (*results)[len(*results)-1]
	if last.Error != nil {
		return nil, fmt.Errorf("calculator.Calculate: %w", last.Error)
	}

	// Маппим "сырые" строки в клиентский формат с вычислением grant.
	out := make([]models.CalculatorResult, 0, len(last.Result))
	for i := range last.Result {
		out = append(out, last.Result[i].ToResult(req.Score))
	}

	return out, nil
}
