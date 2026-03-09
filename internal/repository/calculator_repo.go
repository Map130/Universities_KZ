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
//  1. LET $s1, $s2 — RecordID двух профильных предметов.
//  2. LET $groups  — группы ОП, которые требуют ОБА предмета (HAVING count() = 2).
//  3. LET $specs   — специальности, принадлежащие этим группам.
//  4. SELECT       — офферы вузов, где min_score <= балл абитуриента,
//     с развёрнутыми данными вуза, специальности и группы через graph traversal.
//
// Placeholder {{CITY_FILTER}} заменяется на фильтр по городу при необходимости.
const calculatorBaseQuery = `
    LET $s1 = type::thing("subject", $subject1);
    LET $s2 = type::thing("subject", $subject2);

    LET $groups = (
        SELECT VALUE in FROM requires
        WHERE out = $s1 OR out = $s2
        GROUP BY in
        HAVING count() = 2
    );

    LET $specs = (
        SELECT VALUE id FROM specialty
        WHERE ` + "`group`" + ` IN $groups
    );

    SELECT
        grant_count,
        quota_grant_count,
        tuition_fee,
        min_score,
        last_year_threshold,
        in.name       AS uni_name,
        in.abbr       AS uni_abbr,
        in.city       AS uni_city,
        in.type       AS uni_type,
        in.logo_url   AS uni_logo,
        out.code      AS spec_code,
        out.name      AS spec_name,
        out.` + "`group`" + `.code AS group_code,
        out.` + "`group`" + `.name AS group_name
    FROM offers
    WHERE out IN $specs
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
