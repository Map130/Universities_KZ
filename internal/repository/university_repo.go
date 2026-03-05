// Package repository реализует паттерн «Репозиторий» для доступа к данным
// в SurrealDB. Репозитории не знают об HTTP-слое и работают только с
// доменными моделями из пакета models.
package repository

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"

	"github.com/Map130/universities/internal/models"
)

// UniversityRepository описывает контракт для работы с вузами.
// Интерфейс позволяет подменять реализацию (например, для тестов).
type UniversityRepository interface {
	// GetAll возвращает список вузов с учётом фильтров (город, тип, поиск, пагинация).
	GetAll(ctx context.Context, f models.UniversityFilters) ([]models.University, error)

	// GetByID возвращает один вуз по его RecordID.
	GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.University, error)

	// GetWithSpecialties возвращает вуз вместе со списком предлагаемых
	// специальностей (графовый запрос через edge-таблицу offers).
	GetWithSpecialties(ctx context.Context, id surrealmodels.RecordID) (*models.UniversityDetail, error)

	// Create создаёт новый вуз и возвращает его (с заполненным ID и timestamps).
	Create(ctx context.Context, u models.University) (*models.University, error)

	// Update обновляет существующий вуз целиком (PUT-семантика).
	Update(ctx context.Context, id surrealmodels.RecordID, u models.University) (*models.University, error)

	// Delete удаляет вуз по ID.
	Delete(ctx context.Context, id surrealmodels.RecordID) error

	// CreateOffer создаёт графовую связь university -> specialty
	// с данными о грантах и стоимости обучения.
	CreateOffer(ctx context.Context, universityID, specialtyID surrealmodels.RecordID, input models.CreateOfferInput) (*models.Offers, error)

	// UpdateOffer обновляет данные существующей связи offers по её ID.
	UpdateOffer(ctx context.Context, id surrealmodels.RecordID, input models.CreateOfferInput) (*models.Offers, error)

	// DeleteOffer удаляет графовую связь offers по её ID.
	DeleteOffer(ctx context.Context, id surrealmodels.RecordID) error

	// DeleteAllOffers удаляет все связи offers для данного вуза.
	DeleteAllOffers(ctx context.Context, universityID surrealmodels.RecordID) error
}

// surrealUniversityRepo — реализация UniversityRepository поверх SurrealDB.
type surrealUniversityRepo struct {
	db *surrealdb.DB
}

// NewUniversityRepository создаёт репозиторий вузов.
func NewUniversityRepository(db *surrealdb.DB) UniversityRepository {
	return &surrealUniversityRepo{db: db}
}

// ---------------------------------------------------------------------------
//  GetAll
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) GetAll(ctx context.Context, f models.UniversityFilters) ([]models.University, error) {
	// Динамическое построение SurrealQL-запроса.
	query := "SELECT * FROM university"
	vars := map[string]any{}
	clauses := []string{}

	// Фильтр по городу (точный, используется индекс idx_university_city).
	if f.City != "" {
		clauses = append(clauses, "city = $city")
		vars["city"] = f.City
	}

	// Фильтр по типу вуза.
	if f.Type != "" && f.Type.IsValid() {
		clauses = append(clauses, "type = $type")
		vars["type"] = string(f.Type)
	}

	// Полнотекстовый поиск по названию (язык определяется параметром Lang).
	if f.Search != "" {
		lang := f.Lang
		if lang == "" {
			lang = "ru"
		}
		// Проверяем допустимость языка, чтобы не допустить инъекцию в имя поля.
		switch lang {
		case "kz", "ru", "en":
			// ok
		default:
			lang = "ru"
		}
		clauses = append(clauses, fmt.Sprintf("name.%s @@ $search", lang))
		vars["search"] = f.Search
	}

	// Сборка WHERE.
	if len(clauses) > 0 {
		query += " WHERE "
		for i, c := range clauses {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	query += " ORDER BY name.ru ASC"

	// Пагинация.
	if f.Limit > 0 {
		query += " LIMIT $limit"
		vars["limit"] = f.Limit
	}
	if f.Offset > 0 {
		query += " START $offset"
		vars["offset"] = f.Offset
	}

	results, err := surrealdb.Query[[]models.University](ctx, r.db, query, vars)
	if err != nil {
		return nil, fmt.Errorf("university.GetAll: query: %w", err)
	}

	if len(*results) == 0 {
		return []models.University{}, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("university.GetAll: %w", first.Error)
	}

	return first.Result, nil
}

// ---------------------------------------------------------------------------
//  GetByID
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) GetByID(ctx context.Context, id surrealmodels.RecordID) (*models.University, error) {
	result, err := surrealdb.Select[models.University](ctx, r.db, id)
	if err != nil {
		return nil, fmt.Errorf("university.GetByID: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  GetWithSpecialties
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) GetWithSpecialties(ctx context.Context, id surrealmodels.RecordID) (*models.UniversityDetail, error) {
	// 1. Получаем сам вуз.
	uni, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("university.GetWithSpecialties: %w", err)
	}

	// 2. Графовый запрос: все связи offers для этого вуза с
	//    развёрнутыми объектами специальностей (FETCH out).
	//
	//    SELECT * FROM offers WHERE in = $uni_id FETCH out
	//
	//    FETCH out заменяет RecordID в поле `out` на полный
	//    объект specialty, что маппится в OfferWithSpecialty.Out.
	offerResults, err := surrealdb.Query[[]models.OfferWithSpecialty](
		ctx, r.db,
		"SELECT * FROM offers WHERE in = $uni_id FETCH out",
		map[string]any{"uni_id": id},
	)
	if err != nil {
		return nil, fmt.Errorf("university.GetWithSpecialties: offers query: %w", err)
	}

	var offers []models.OfferWithSpecialty
	if len(*offerResults) > 0 {
		first := (*offerResults)[0]
		if first.Error != nil {
			return nil, fmt.Errorf("university.GetWithSpecialties: offers: %w", first.Error)
		}
		offers = first.Result
	}

	if offers == nil {
		offers = []models.OfferWithSpecialty{}
	}

	return &models.UniversityDetail{
		University: *uni,
		Offers:     offers,
	}, nil
}

// ---------------------------------------------------------------------------
//  Create
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) Create(ctx context.Context, u models.University) (*models.University, error) {
	// При создании ID, created_at, updated_at генерируются SurrealDB.
	// Мы передаём данные через map, чтобы не отправлять nil-поля,
	// которые SCHEMAFULL-таблица может отвергнуть.
	data := map[string]any{
		"name": map[string]any{
			"kz": u.Name.KZ,
			"ru": u.Name.RU,
			"en": u.Name.EN,
		},
		"abbr": u.Abbr,
		"city": u.City,
		"type": string(u.Type),
	}

	if u.LogoURL != nil {
		data["logo_url"] = *u.LogoURL
	}
	if u.Website != nil {
		data["website"] = *u.Website
	}
	if u.Description != nil {
		data["description"] = *u.Description
	}

	// Кастомный CSS для премиум-вузов (всегда передаём, DEFAULT "" в схеме).
	data["custom_css"] = u.CustomCSS

	// SurrealDB возвращает массив при CREATE на таблицу (Table),
	// даже если создаётся одна запись. Поэтому десериализуем как []models.University
	// и берём первый элемент.
	result, err := surrealdb.Create[[]models.University](ctx, r.db, surrealmodels.Table("university"), data)
	if err != nil {
		return nil, fmt.Errorf("university.Create: %w", err)
	}
	if result == nil || len(*result) == 0 {
		return nil, fmt.Errorf("university.Create: empty result from DB")
	}

	created := (*result)[0]
	return &created, nil
}

// ---------------------------------------------------------------------------
//  Update
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) Update(ctx context.Context, id surrealmodels.RecordID, u models.University) (*models.University, error) {
	data := map[string]any{
		"name": map[string]any{
			"kz": u.Name.KZ,
			"ru": u.Name.RU,
			"en": u.Name.EN,
		},
		"abbr": u.Abbr,
		"city": u.City,
		"type": string(u.Type),
	}

	if u.LogoURL != nil {
		data["logo_url"] = *u.LogoURL
	}
	if u.Website != nil {
		data["website"] = *u.Website
	}
	if u.Description != nil {
		data["description"] = *u.Description
	}

	// Кастомный CSS для премиум-вузов.
	data["custom_css"] = u.CustomCSS

	result, err := surrealdb.Merge[models.University](ctx, r.db, id, data)
	if err != nil {
		return nil, fmt.Errorf("university.Update: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  Delete
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) Delete(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.University](ctx, r.db, id); err != nil {
		return fmt.Errorf("university.Delete: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
//  CreateOffer  (university ──offers──▶ specialty)
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) CreateOffer(
	ctx context.Context,
	universityID, specialtyID surrealmodels.RecordID,
	input models.CreateOfferInput,
) (*models.Offers, error) {
	rel := &surrealdb.Relationship{
		In:       universityID,
		Out:      specialtyID,
		Relation: surrealmodels.Table("offers"),
		Data: map[string]any{
			"grant_count":         input.GrantCount,
			"quota_grant_count":   input.QuotaGrantCount,
			"tuition_fee":         input.TuitionFee,
			"min_score":           input.MinScore,
			"last_year_threshold": input.LastYearThreshold,
		},
	}

	result, err := surrealdb.Relate[models.Offers](ctx, r.db, rel)
	if err != nil {
		return nil, fmt.Errorf("university.CreateOffer: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
//  UpdateOffer  (обновление данных связи offers)
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) UpdateOffer(
	ctx context.Context,
	id surrealmodels.RecordID,
	input models.CreateOfferInput,
) (*models.Offers, error) {
	data := map[string]any{
		"grant_count":         input.GrantCount,
		"quota_grant_count":   input.QuotaGrantCount,
		"tuition_fee":         input.TuitionFee,
		"min_score":           input.MinScore,
		"last_year_threshold": input.LastYearThreshold,
	}

	result, err := surrealdb.Merge[models.Offers](ctx, r.db, id, data)
	if err != nil {
		return nil, fmt.Errorf("university.UpdateOffer: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
//  DeleteOffer  (удаление одной связи offers по ID)
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) DeleteOffer(ctx context.Context, id surrealmodels.RecordID) error {
	if _, err := surrealdb.Delete[models.Offers](ctx, r.db, id); err != nil {
		return fmt.Errorf("university.DeleteOffer: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
//  DeleteAllOffers  (удаление всех связей offers для вуза)
// ---------------------------------------------------------------------------

func (r *surrealUniversityRepo) DeleteAllOffers(ctx context.Context, universityID surrealmodels.RecordID) error {
	_, err := surrealdb.Query[any](
		ctx, r.db,
		"DELETE FROM offers WHERE in = $uni_id",
		map[string]any{"uni_id": universityID},
	)
	if err != nil {
		return fmt.Errorf("university.DeleteAllOffers: %w", err)
	}
	return nil
}
