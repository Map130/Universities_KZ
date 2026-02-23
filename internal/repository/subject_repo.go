package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

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
	db *surrealdb.DB
}

func NewSubjectRepository(db *surrealdb.DB) SubjectRepository {
	return &surrealSubjectRepo{db: db}
}

func (r *surrealSubjectRepo) GetAll(ctx context.Context) ([]models.Subject, error) {
	result, err := surrealdb.Select[[]models.Subject](ctx, r.db, surrealmodels.Table("subject"))
	if err != nil {
		return nil, fmt.Errorf("subject.GetAll: %w", err)
	}
	if result == nil {
		return []models.Subject{}, nil
	}
	return *result, nil
}

func (r *surrealSubjectRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Subject, error) {
	result, err := surrealdb.Select[models.Subject](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("subject.GetByID: %w", err)
	}
	return result, nil
}

func (r *surrealSubjectRepo) Create(ctx context.Context, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"name": map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
	}
	result, err := surrealdb.Create[models.Subject](ctx, r.db, surrealmodels.Table("subject"), data)
	if err != nil {
		return nil, fmt.Errorf("subject.Create: %w", err)
	}
	return result, nil
}

func (r *surrealSubjectRepo) Update(ctx context.Context, id surrealmodels.RecordID, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"name": map[string]any{"kz": s.Name.KZ, "ru": s.Name.RU, "en": s.Name.EN},
	}
	result, err := surrealdb.Merge[models.Subject](ctx, r.db, id, data)
	if err != nil {
		return nil, fmt.Errorf("subject.Update: %w", err)
	}
	return result, nil
}

func (r *surrealSubjectRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.Subject](ctx, r.db, id); err != nil {
		return fmt.Errorf("subject.Delete: %w", err)
	}
	return nil
}

// FindUniversitiesBySubject — обратный обход графа:
//
//	subject <─requires─ specialty_group <─.group─ specialty <─offers─ university
func (r *surrealSubjectRepo) FindUniversitiesBySubject(ctx context.Context, subjectID surrealmodels.RecordID) ([]models.University, error) {
	query := `
		LET $group_ids = (SELECT VALUE in FROM requires WHERE out = $subject_id);
		LET $spec_ids  = (SELECT VALUE id FROM specialty WHERE ` + "`group`" + ` IN $group_ids);
		LET $uni_ids   = array::distinct((SELECT VALUE in FROM offers WHERE out IN $spec_ids));
		SELECT * FROM university WHERE id IN $uni_ids ORDER BY name.ru ASC;
	`

	results, err := surrealdb.Query[[]models.University](
		ctx, r.db, query,
		map[string]any{"subject_id": subjectID},
	)
	if err != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: query: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return []models.University{}, nil
	}

	// 4 оператора (LET, LET, LET, SELECT) — берём последний.
	last := (*results)[len(*results)-1]
	if last.Error != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: %w", last.Error)
	}

	if last.Result == nil {
		return []models.University{}, nil
	}

	return last.Result, nil
}
