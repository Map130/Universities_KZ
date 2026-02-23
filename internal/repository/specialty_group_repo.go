package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
)

// SpecialtyGroupRepository описывает контракт для работы с группами ОП.
type SpecialtyGroupRepository interface {
	// GetAll возвращает список групп ОП с учётом фильтров.
	GetAll(ctx context.Context, f models.SpecialtyGroupFilters) ([]models.SpecialtyGroup, error)

	// GetByID возвращает одну группу по RecordID.
	GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.SpecialtyGroup, error)

	// GetByCode возвращает группу по уникальному коду ("B057").
	GetByCode(ctx context.Context, code string) (*models.SpecialtyGroup, error)

	// GetWithSubjects возвращает группу ОП вместе с привязанными
	// предметами ЕНТ (графовый запрос через edge requires).
	GetWithSubjects(ctx context.Context, id surrealmodels.RecordID) (*models.SpecialtyGroup, []models.RequiredSubject, error)

	// Create создаёт новую группу ОП.
	Create(ctx context.Context, g models.SpecialtyGroup) (*models.SpecialtyGroup, error)

	// Update обновляет существующую группу (MERGE-семантика).
	Update(ctx context.Context, id surrealmodels.RecordID, g models.SpecialtyGroup) (*models.SpecialtyGroup, error)

	// Delete удаляет группу по ID.
	Delete(ctx context.Context, id surrealmodels.RecordID) error

	// CreateRequires создаёт графовую связь specialty_group -> subject
	// с указанием приоритета предмета (1 = основной, 2 = второй).
	CreateRequires(ctx context.Context, groupID, subjectID surrealmodels.RecordID, input models.CreateRequiresInput) (*models.Requires, error)
}

// surrealSpecialtyGroupRepo — реализация поверх SurrealDB.
type surrealSpecialtyGroupRepo struct {
	db *surrealdb.DB
}

// NewSpecialtyGroupRepository создаёт репозиторий групп ОП.
func NewSpecialtyGroupRepository(db *surrealdb.DB) SpecialtyGroupRepository {
	return &surrealSpecialtyGroupRepo{db: db}
}

// ---------------------------------------------------------------------------
//  GetAll
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) GetAll(ctx context.Context, f models.SpecialtyGroupFilters) ([]models.SpecialtyGroup, error) {
	query := "SELECT * FROM specialty_group"
	vars := map[string]any{}
	clauses := []string{}

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

	results, err := surrealdb.Query[[]models.SpecialtyGroup](ctx, r.db, query, vars)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.GetAll: query: %w", err)
	}

	if len(*results) == 0 {
		return []models.SpecialtyGroup{}, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("specialtyGroup.GetAll: %w", first.Error)
	}

	return first.Result, nil
}

// ---------------------------------------------------------------------------
//  GetByID
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.SpecialtyGroup, error) {
	result, err := surrealdb.Select[models.SpecialtyGroup](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.GetByID: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
//  GetByCode
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) GetByCode(ctx context.Context, code string) (*models.SpecialtyGroup, error) {
	results, err := surrealdb.Query[[]models.SpecialtyGroup](
		ctx, r.db,
		"SELECT * FROM specialty_group WHERE code = $code LIMIT 1",
		map[string]any{"code": code},
	)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.GetByCode: query: %w", err)
	}

	if len(*results) == 0 {
		return nil, fmt.Errorf("specialtyGroup.GetByCode: no query results")
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("specialtyGroup.GetByCode: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, fmt.Errorf("specialtyGroup.GetByCode: group with code %q not found", code)
	}

	return &first.Result[0], nil
}

// ---------------------------------------------------------------------------
//  GetWithSubjects
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) GetWithSubjects(ctx context.Context, id surrealmodels.RecordID) (*models.SpecialtyGroup, []models.RequiredSubject, error) {
	group, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("specialtyGroup.GetWithSubjects: %w", err)
	}

	subjectResults, err := surrealdb.Query[[]models.RequiredSubject](
		ctx, r.db,
		"SELECT * FROM requires WHERE in = $group_id ORDER BY priority ASC FETCH out",
		map[string]any{"group_id": id},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("specialtyGroup.GetWithSubjects: requires query: %w", err)
	}

	var subjects []models.RequiredSubject
	if len(*subjectResults) > 0 {
		first := (*subjectResults)[0]
		if first.Error != nil {
			return nil, nil, fmt.Errorf("specialtyGroup.GetWithSubjects: requires: %w", first.Error)
		}
		subjects = first.Result
	}

	if subjects == nil {
		subjects = []models.RequiredSubject{}
	}

	return group, subjects, nil
}

// ---------------------------------------------------------------------------
//  Create
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) Create(ctx context.Context, g models.SpecialtyGroup) (*models.SpecialtyGroup, error) {
	data := map[string]any{
		"code": g.Code,
		"name": map[string]any{"kz": g.Name.KZ, "ru": g.Name.RU, "en": g.Name.EN},
	}

	result, err := surrealdb.Create[models.SpecialtyGroup](ctx, r.db, surrealmodels.Table("specialty_group"), data)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.Create: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
//  Update
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) Update(ctx context.Context, id surrealmodels.RecordID, g models.SpecialtyGroup) (*models.SpecialtyGroup, error) {
	data := map[string]any{
		"code": g.Code,
		"name": map[string]any{"kz": g.Name.KZ, "ru": g.Name.RU, "en": g.Name.EN},
	}

	result, err := surrealdb.Merge[models.SpecialtyGroup](ctx, r.db, id, data)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.Update: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
//  Delete
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.SpecialtyGroup](ctx, r.db, id); err != nil {
		return fmt.Errorf("specialtyGroup.Delete: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
//  CreateRequires  (specialty_group ──requires──▶ subject)
// ---------------------------------------------------------------------------

func (r *surrealSpecialtyGroupRepo) CreateRequires(
	ctx context.Context,
	groupID, subjectID surrealmodels.RecordID,
	input models.CreateRequiresInput,
) (*models.Requires, error) {
	rel := &surrealdb.Relationship{
		In:       groupID,
		Out:      subjectID,
		Relation: surrealmodels.Table("requires"),
		Data: map[string]any{
			"priority": int(input.Priority),
		},
	}

	result, err := surrealdb.Relate[models.Requires](ctx, r.db, rel)
	if err != nil {
		return nil, fmt.Errorf("specialtyGroup.CreateRequires: %w", err)
	}
	return result, nil
}
