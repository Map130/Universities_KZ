package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/models"
)

// SpecialtyRepository описывает контракт для работы со специальностями.
type SpecialtyRepository interface {
	// GetAll возвращает список специальностей с учётом фильтров.
	GetAll(ctx context.Context, f models.SpecialtyFilters) ([]models.Specialty, error)

	// GetByID возвращает одну специальность по её RecordID.
	GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Specialty, error)

	// GetByCode возвращает специальность по уникальному коду ("6B06101").
	GetByCode(ctx context.Context, code string) (*models.Specialty, error)

	// GetWithSubjects возвращает специальность вместе с необходимыми
	// предметами ЕНТ. Предметы берутся из группы ОП (specialty.group -> requires -> subject).
	GetWithSubjects(ctx context.Context, id surrealmodels.RecordID) (*models.Specialty, []models.RequiredSubject, error)

	// Create создаёт новую специальность.
	Create(ctx context.Context, s models.Specialty) (*models.Specialty, error)

	// Update обновляет существующую специальность (MERGE-семантика).
	Update(ctx context.Context, id surrealmodels.RecordID, s models.Specialty) (*models.Specialty, error)

	// Delete удаляет специальность по ID.
	Delete(ctx context.Context, id surrealmodels.RecordID) error
}

// surrealSpecialtyRepo — реализация SpecialtyRepository поверх SurrealDB.
type surrealSpecialtyRepo struct {
	pool *db.Pool
}

// NewSpecialtyRepository создаёт репозиторий специальностей.
func NewSpecialtyRepository(pool *db.Pool) SpecialtyRepository {
	return &surrealSpecialtyRepo{pool: pool}
}

// ---------------------------------------------------------------------------
//  GetAll
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) GetAll(ctx context.Context, f models.SpecialtyFilters) ([]models.Specialty, error) {
	query := "SELECT * FROM specialty"
	vars := map[string]any{}
	clauses := []string{}

	// Полнотекстовый поиск по названию.
	if f.Search != "" {
		lang := f.Lang
		if lang == "" {
			lang = "ru"
		}
		switch lang {
		case "kz", "ru", "en":
		default:
			lang = "ru"
		}
		clauses = append(clauses, fmt.Sprintf("name.%s @@ $search", lang))
		vars["search"] = f.Search
	}

	// Фильтр по коду группы ОП.
	// SurrealDB резолвит record link: `group`.code обращается
	// к полю code связанной записи specialty_group.
	if f.GroupCode != "" {
		clauses = append(clauses, "`group`.code = $group_code")
		vars["group_code"] = f.GroupCode
	}

	if len(clauses) > 0 {
		query += " WHERE "
		for i, c := range clauses {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	query += " ORDER BY code ASC"

	if f.Limit > 0 {
		query += " LIMIT $limit"
		vars["limit"] = f.Limit
	}
	if f.Offset > 0 {
		query += " START $offset"
		vars["offset"] = f.Offset
	}

	results, err := surrealdb.Query[[]models.Specialty](ctx, r.pool.Get(), query, vars)
	if err != nil {
		return nil, fmt.Errorf("specialty.GetAll: query: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return []models.Specialty{}, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("specialty.GetAll: %w", first.Error)
	}

	return first.Result, nil
}

// ---------------------------------------------------------------------------
//  GetByID
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Specialty, error) {
	result, err := surrealdb.Select[models.Specialty](ctx, r.pool.Get(), id)
	if err != nil {
		return nil, fmt.Errorf("specialty.GetByID: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("specialty.GetByID: not found")
	}
	return result, nil
}

// ---------------------------------------------------------------------------
//  GetByCode
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) GetByCode(ctx context.Context, code string) (*models.Specialty, error) {
	results, err := surrealdb.Query[[]models.Specialty](
		ctx, r.pool.Get(),
		"SELECT * FROM specialty WHERE code = $code LIMIT 1",
		map[string]any{"code": code},
	)
	if err != nil {
		return nil, fmt.Errorf("specialty.GetByCode: query: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("specialty.GetByCode: no query results")
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("specialty.GetByCode: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, fmt.Errorf("specialty.GetByCode: specialty with code %q not found", code)
	}

	return &first.Result[0], nil
}

// ---------------------------------------------------------------------------
//  GetWithSubjects
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) GetWithSubjects(ctx context.Context, id surrealmodels.RecordID) (*models.Specialty, []models.RequiredSubject, error) {
	// 1. Получаем специальность (включая group как RecordID).
	spec, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("specialty.GetWithSubjects: %w", err)
	}

	// 2. Предметы ЕНТ привязаны к группе ОП, а не к специальности.
	//    Запрашиваем requires через spec.Group (record link на specialty_group).
	subjectResults, err := surrealdb.Query[[]models.RequiredSubject](
		ctx, r.pool.Get(),
		"SELECT * FROM requires WHERE in = $group_id ORDER BY priority ASC FETCH out",
		map[string]any{"group_id": spec.Group},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("specialty.GetWithSubjects: requires query: %w", err)
	}

	var subjects []models.RequiredSubject
	if subjectResults != nil && len(*subjectResults) > 0 {
		first := (*subjectResults)[0]
		if first.Error != nil {
			return nil, nil, fmt.Errorf("specialty.GetWithSubjects: requires: %w", first.Error)
		}
		subjects = first.Result
	}

	if subjects == nil {
		subjects = []models.RequiredSubject{}
	}

	return spec, subjects, nil
}

// ---------------------------------------------------------------------------
//  Create
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) Create(ctx context.Context, s models.Specialty) (*models.Specialty, error) {
	data := map[string]any{
		"code":  s.Code,
		"name":  map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
		"group": s.Group, // RecordID → specialty_group:...
	}

	// SurrealDB возвращает массив при CREATE на таблицу (Table),
	// даже если создаётся одна запись. Десериализуем как []models.Specialty.
	result, err := surrealdb.Create[[]models.Specialty](ctx, r.pool.Get(), surrealmodels.Table("specialty"), data)
	if err != nil {
		return nil, fmt.Errorf("specialty.Create: %w", err)
	}
	if result == nil || len(*result) == 0 {
		return nil, fmt.Errorf("specialty.Create: empty result from DB")
	}
	created := (*result)[0]
	return &created, nil
}

// ---------------------------------------------------------------------------
//  Update
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) Update(ctx context.Context, id surrealmodels.RecordID, s models.Specialty) (*models.Specialty, error) {
	data := map[string]any{
		"code":  s.Code,
		"name":  map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
		"group": s.Group,
	}

	// Используем UPSERT для идемпотентного обновления/создания (SurrealDB 1.0+).
	query := "UPSERT $id MERGE $data"
	vars := map[string]any{
		"id":   id,
		"data": data,
	}

	results, err := surrealdb.Query[[]models.Specialty](ctx, r.pool.Get(), query, vars)
	if err != nil {
		return nil, fmt.Errorf("specialty.Update: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("specialty.Update: empty result from DB")
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("specialty.Update: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, fmt.Errorf("specialty.Update: failed to upsert record")
	}

	return &first.Result[0], nil
}

// ---------------------------------------------------------------------------
//  Delete
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.Specialty](ctx, r.pool.Get(), id); err != nil {
		return fmt.Errorf("specialty.Delete: %w", err)
	}
	return nil
}
