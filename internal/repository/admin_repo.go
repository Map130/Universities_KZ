// Package repository реализует паттерн «Репозиторий» для доступа к данным
// в SurrealDB.
package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go"

	"github.com/Map130/universities/internal/models"
)

// AdminRepository описывает контракт для работы с whitelist-ом администраторов.
type AdminRepository interface {
	// IsAllowed проверяет, есть ли email в таблице allowed_admins.
	// Email приводится к нижнему регистру перед проверкой.
	IsAllowed(ctx context.Context, email string) (bool, error)

	// GetByEmail возвращает запись администратора по email.
	// Если запись не найдена, возвращает nil без ошибки.
	GetByEmail(ctx context.Context, email string) (*models.AllowedAdmin, error)

	// Create добавляет новый email в whitelist.
	Create(ctx context.Context, admin models.AllowedAdmin) (*models.AllowedAdmin, error)

	// GetAll возвращает все записи из таблицы allowed_admins.
	GetAll(ctx context.Context) ([]models.AllowedAdmin, error)

	// Delete удаляет запись администратора по email.
	Delete(ctx context.Context, email string) error
}

// surrealAdminRepo — реализация AdminRepository поверх SurrealDB.
type surrealAdminRepo struct {
	db *surrealdb.DB
}

// NewAdminRepository создаёт репозиторий администраторов.
func NewAdminRepository(db *surrealdb.DB) AdminRepository {
	return &surrealAdminRepo{db: db}
}

// ---------------------------------------------------------------------------
//  IsAllowed
// ---------------------------------------------------------------------------

func (r *surrealAdminRepo) IsAllowed(ctx context.Context, email string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false, nil
	}

	results, err := surrealdb.Query[[]models.AllowedAdmin](
		ctx, r.db,
		"SELECT * FROM allowed_admins WHERE email = $email LIMIT 1",
		map[string]any{"email": email},
	)
	if err != nil {
		return false, fmt.Errorf("admin.IsAllowed: query: %w", err)
	}

	if len(*results) == 0 {
		return false, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return false, fmt.Errorf("admin.IsAllowed: %w", first.Error)
	}

	return len(first.Result) > 0, nil
}

// ---------------------------------------------------------------------------
//  GetByEmail
// ---------------------------------------------------------------------------

func (r *surrealAdminRepo) GetByEmail(ctx context.Context, email string) (*models.AllowedAdmin, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}

	results, err := surrealdb.Query[[]models.AllowedAdmin](
		ctx, r.db,
		"SELECT * FROM allowed_admins WHERE email = $email LIMIT 1",
		map[string]any{"email": email},
	)
	if err != nil {
		return nil, fmt.Errorf("admin.GetByEmail: query: %w", err)
	}

	if len(*results) == 0 {
		return nil, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("admin.GetByEmail: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, nil
	}

	return &first.Result[0], nil
}

// ---------------------------------------------------------------------------
//  Create
// ---------------------------------------------------------------------------

func (r *surrealAdminRepo) Create(ctx context.Context, admin models.AllowedAdmin) (*models.AllowedAdmin, error) {
	admin.Email = strings.ToLower(strings.TrimSpace(admin.Email))
	if admin.Email == "" {
		return nil, fmt.Errorf("admin.Create: email must not be empty")
	}

	data := map[string]any{
		"email": admin.Email,
	}
	if admin.Name != nil {
		data["name"] = *admin.Name
	}

	results, err := surrealdb.Query[[]models.AllowedAdmin](
		ctx, r.db,
		"CREATE allowed_admins SET email = $email, name = $name",
		map[string]any{
			"email": admin.Email,
			"name":  admin.Name,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("admin.Create: %w", err)
	}

	if len(*results) == 0 {
		return nil, fmt.Errorf("admin.Create: empty result")
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("admin.Create: %w", first.Error)
	}

	if len(first.Result) == 0 {
		return nil, fmt.Errorf("admin.Create: no record returned")
	}

	return &first.Result[0], nil
}

// ---------------------------------------------------------------------------
//  GetAll
// ---------------------------------------------------------------------------

func (r *surrealAdminRepo) GetAll(ctx context.Context) ([]models.AllowedAdmin, error) {
	results, err := surrealdb.Query[[]models.AllowedAdmin](
		ctx, r.db,
		"SELECT * FROM allowed_admins ORDER BY email ASC",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("admin.GetAll: query: %w", err)
	}

	if len(*results) == 0 {
		return []models.AllowedAdmin{}, nil
	}

	first := (*results)[0]
	if first.Error != nil {
		return nil, fmt.Errorf("admin.GetAll: %w", first.Error)
	}

	if first.Result == nil {
		return []models.AllowedAdmin{}, nil
	}

	return first.Result, nil
}

// ---------------------------------------------------------------------------
//  Delete
// ---------------------------------------------------------------------------

func (r *surrealAdminRepo) Delete(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("admin.Delete: email must not be empty")
	}

	_, err := surrealdb.Query[any](
		ctx, r.db,
		"DELETE FROM allowed_admins WHERE email = $email",
		map[string]any{"email": email},
	)
	if err != nil {
		return fmt.Errorf("admin.Delete: %w", err)
	}

	return nil
}
