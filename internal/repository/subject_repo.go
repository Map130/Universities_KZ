package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
)

// SubjectRepository описывает контракт для работы с предметами ЕНТ.
// Интерфейс позволяет подменять реализацию (например, для тестов).
type SubjectRepository interface {
	// GetAll возвращает полный список предметов ЕНТ.
	GetAll(ctx context.Context) ([]models.Subject, error)

	// GetByID возвращает один предмет по его RecordID.
	GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Subject, error)

	// Create создаёт новый предмет ЕНТ и возвращает его
	// (с заполненным ID и timestamps).
	Create(ctx context.Context, s models.Subject) (*models.Subject, error)

	// Update обновляет существующий предмет (MERGE-семантика).
	Update(ctx context.Context, id surrealmodels.RecordID, s models.Subject) (*models.Subject, error)

	// Delete удаляет предмет по ID.
	Delete(ctx context.Context, id surrealmodels.RecordID) error

	// FindUniversitiesBySubject выполняет обратный графовый запрос:
	//   subject <─requires─ specialty <─offers─ university
	// и возвращает список вузов, которые принимают по данному предмету.
	FindUniversitiesBySubject(ctx context.Context, subjectID surrealmodels.RecordID) ([]models.University, error)
}

// surrealSubjectRepo — реализация SubjectRepository поверх SurrealDB.
type surrealSubjectRepo struct {
	db *surrealdb.DB
}

// NewSubjectRepository создаёт репозиторий предметов ЕНТ.
func NewSubjectRepository(db *surrealdb.DB) SubjectRepository {
	return &surrealSubjectRepo{db: db}
}

// ---------------------------------------------------------------------------
//  GetAll
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
//  GetByID
// ---------------------------------------------------------------------------

func (r *surrealSubjectRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.Subject, error) {
	result, err := surrealdb.Select[models.Subject](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("subject.GetByID: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  Create
// ---------------------------------------------------------------------------

func (r *surrealSubjectRepo) Create(ctx context.Context, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"name": map[string]any{
			"kz": s.Name.KZ,
			"ru": s.Name.RU,
			"en": s.Name.EN,
		},
	}

	result, err := surrealdb.Create[models.Subject](ctx, r.db, surrealmodels.Table("subject"), data)
	if err != nil {
		return nil, fmt.Errorf("subject.Create: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  Update
// ---------------------------------------------------------------------------

func (r *surrealSubjectRepo) Update(ctx context.Context, id surrealmodels.RecordID, s models.Subject) (*models.Subject, error) {
	data := map[string]any{
		"name": map[string]any{
			"kz": s.Name.KZ,
			"ru": s.Name.RU,
			"en": s.Name.EN,
		},
	}

	result, err := surrealdb.Merge[models.Subject](ctx, r.db, id, data)
	if err != nil {
		return nil, fmt.Errorf("subject.Update: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  Delete
// ---------------------------------------------------------------------------

func (r *surrealSubjectRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.Subject](ctx, r.db, id); err != nil {
		return fmt.Errorf("subject.Delete: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
//  FindUniversitiesBySubject
// ---------------------------------------------------------------------------

// FindUniversitiesBySubject выполняет обратную навигацию по графу:
//
//	subject <─requires─ specialty <─offers─ university
//
// SurrealQL:
//
//	SELECT
//	    <-requires<-specialty<-offers<-university.* AS universities
//	FROM $subject_id
//
// Альтернативный (более простой для десериализации) вариант — два перехода
// через промежуточные запросы. Здесь используется один составной запрос,
// который сначала собирает ID специальностей, затем ID вузов, и наконец
// возвращает полные записи вузов.
func (r *surrealSubjectRepo) FindUniversitiesBySubject(ctx context.Context, subjectID surrealmodels.RecordID) ([]models.University, error) {
	// Используем цепочку подзапросов для надёжной десериализации:
	// 1. Находим specialty-ы, которым нужен данный предмет.
	// 2. Находим university-ы, которые предлагают эти specialty-ы.
	// 3. Дедуплицируем (DISTINCT) — один вуз может предлагать несколько
	//    специальностей с одним и тем же предметом.
	query := `
		LET $spec_ids = (SELECT VALUE in FROM requires WHERE out = $subject_id);
		LET $uni_ids  = array::distinct((SELECT VALUE in FROM offers WHERE out IN $spec_ids));
		SELECT * FROM university WHERE id IN $uni_ids ORDER BY name.ru ASC;
	`

	results, err := surrealdb.Query[[]models.University](
		ctx, r.db, query,
		map[string]any{"subject_id": subjectID},
	)
	if err != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: query: %w", err)
	}

	// Запрос содержит три оператора (LET, LET, SELECT).
	// Результат SELECT — последний элемент среза QueryResult.
	if results == nil || len(*results) == 0 {
		return []models.University{}, nil
	}

	// Берём последний QueryResult (третий оператор — SELECT).
	last := (*results)[len(*results)-1]
	if last.Error != nil {
		return nil, fmt.Errorf("subject.FindUniversitiesBySubject: %w", last.Error)
	}

	if last.Result == nil {
		return []models.University{}, nil
	}

	return last.Result, nil
}
