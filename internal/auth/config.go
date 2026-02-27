// Package auth реализует Google OAuth 2.0 аутентификацию для админ-панели.
// Использует github.com/markbates/goth для взаимодействия с Google
// и github.com/gofiber/fiber/v2/middleware/session для хранения сессий.
package auth

import (
	"fmt"
	"os"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/google"
)

// Config хранит параметры OAuth 2.0, загруженные из переменных окружения.
type Config struct {
	// ClientID — Google OAuth 2.0 Client ID.
	ClientID string

	// ClientSecret — Google OAuth 2.0 Client Secret.
	ClientSecret string

	// CallbackURL — полный URL callback-эндпоинта
	// (например, "http://localhost:3000/auth/google/callback").
	CallbackURL string

	// SessionSecret — секрет для подписи сессионных кук.
	SessionSecret string
}

// LoadConfigFromEnv загружает конфигурацию OAuth из переменных окружения.
// Возвращает ошибку, если хотя бы одна обязательная переменная не задана.
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		ClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		CallbackURL:   os.Getenv("GOOGLE_CALLBACK_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}

	if cfg.ClientID == "" {
		return nil, fmt.Errorf("auth: GOOGLE_CLIENT_ID environment variable is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("auth: GOOGLE_CLIENT_SECRET environment variable is required")
	}
	if cfg.CallbackURL == "" {
		return nil, fmt.Errorf("auth: GOOGLE_CALLBACK_URL environment variable is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("auth: SESSION_SECRET environment variable is required")
	}

	return cfg, nil
}

// InitGothProviders регистрирует Google-провайдер в goth.
// Запрашивает scope email и profile для получения данных пользователя.
//
// Должна вызываться один раз при старте приложения, до обработки запросов.
func InitGothProviders(cfg *Config) {
	goth.UseProviders(
		google.New(
			cfg.ClientID,
			cfg.ClientSecret,
			cfg.CallbackURL,
			// Scopes: email для whitelist-проверки, profile для имени.
			"email", "profile",
		),
	)
}
