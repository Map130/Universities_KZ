package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/db"
	"github.com/Map130/universities/internal/models"
)

// SubjectRepository описывает контракт для работы с предметами ЕНТ.
type SubjectRepository interface {
	GetAll(ctx context.Context) ([]models.Subject, error)
	GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Subject, error)
	Create(ctx context.Context, s models.Subject) (*models.Subject, error)
	Update(ctx context.Context, id surrealmodels.RecordID, s models.Subject) (*models.Subject, error)
	Delete(ctx context.Context, id surrealmodels.RecordID) error

	// FindUniversitiesBySubject выполняет обратный графовый запрос:
	//   subject <─requires─ specialty_group <─.group─ specialty <─offers─ university
	FindUniversitiesBySubject(ctx context.Context, subjectID surrealmodels.RecordID) ([]models.University, error)
}

type surrealSubjectRepo struct {
	pool *db.Pool
}

func NewSubjectRepository(pool *db.Pool) SubjectRepository {
	return &surrealSubjectRepo{pool: pool}
}

func (r *surrealSubjectRepo) GetAll(ctx context.Context) ([]models.Subject, error) {
	result, err := surrealdb.Select[[]models.Subject](ctx, r.pool.Get(), surrealmodels.Table("subject"))
	if err != nil {
		return nil, fmt.Errorf("subject.GetAll: %w", err)
	}
	if result == nil {
		return []models.Subject{}, nil
	}
	return *result, nil
}

func (r *surrealSubjectRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Subject, error) {
	result, err := surrealdb.Select[models.Subject](ctx, r.pool.Get(), id)
	if err != nil {
		return nil, fmt.Errorf("subject.GetByID: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("subject.GetByID: not found")
	}
	return result, nil
}

func (r *surrealSubjectRepo) Create(ctx context.Context, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"name": map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
	}
	// SurrealDB возвращает массив при CREATE на таблицу (Table),
	// даже если создаётся одна запись. Десериализуем как []models.Subject.
	result, err := surrealdb.Create[[]models.Subject](ctx, r.pool.Get(), surrealmodels.Table("subject"), data)
	if err != nil {
		return nil, fmt.Errorf("subject.Create: %w", err)
	}
	if result == nil || len(*result) == 0 {
		return nil, fmt.Errorf("subject.Create: empty result from DB")
	}
	created := (*result)[0]
	return &created, nil
}

func (r *surrealSubjectRepo) Update(ctx context.Context, id surrealmodels.RecordID, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"code": s.Code,
		"name": map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
	}

	// Используем UPSERT для идемпотентного обновления/создания (SurrealDB 1.0+).
	query := "UPSERT $id MERGE $data"
	vars := map[string]any{
		"id":   id,
		"data": data,
	}

	results, err := surrealdb.Query[[]models.Subject](ctx, r.pool.Get(), query, vars)
	if err != nil {
		return nil, fmt.Errorf("subject.Update: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("subject.Update: empty result from DB")
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("subject.Update: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, fmt.Errorf("subject.Update: failed to upsert record")
	}

	return &first.Result[0], nil
}

func (r *surrealSubjectRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.Subject](ctx, r.pool.Get(), id); err != nil {
		return fmt.Errorf("subject.Delete: %w", err)
	}
	return nil
}

// FindUniversitiesBySubject — обратный обход графа:
//
//	subject <─requires─ specialty_group <─ent_requirement─ university
func (r *surrealSubjectRepo) FindUniversitiesBySubject(ctx context.Context, subjectID surrealmodels.RecordID) ([]models.University, error) {
	query := `
		LET $group_ids = (SELECT VALUE in FROM requires WHERE out = $subject_id);
		LET $uni_ids   = array::distinct((SELECT VALUE in FROM ent_requirement WHERE out IN $group_ids));
		SELECT * FROM university WHERE id IN $uni_ids ORDER BY name.ru ASC;
	`

	results, err := surrealdb.Query[[]models.University](
		ctx, r.pool.Get(), query,
		map[string]any{"subject_id": subjectID},
	)
	if err != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: query: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return []models.University{}, nil
	}

	// 3 оператора (LET, LET, SELECT) — берём последний.
	last := (*results)[len(*results)-1]
	if last.Error != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: %w", last.Error)
	}

	if last.Result == nil {
		return []models.University{}, nil
	}

	return last.Result, nil
}
