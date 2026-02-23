package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/surrealdb/surrealdb.go"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	surrealURL := getEnv("SURREAL_URL", "ws://localhost:8000/rpc")
	surrealUser := getEnv("SURREAL_USER", "root")
	surrealPass := getEnv("SURREAL_PASS", "root")
	surrealNS := getEnv("SURREAL_NS", "test")
	surrealDB := getEnv("SURREAL_DB", "test")
	appPort := getEnv("APP_PORT", "8080")

	// 1. Коннект к SurrealDB
	db, err := surrealdb.FromEndpointURLString(context.Background(), surrealURL)
	if err != nil {
		log.Fatalf("Ошибка подключения к Surreal: %v", err)
	}

	if _, err = db.SignIn(context.Background(), map[string]any{
		"user": surrealUser,
		"pass": surrealPass,
	}); err != nil {
		log.Fatal(err)
	}

	if err = db.Use(context.Background(), surrealNS, surrealDB); err != nil {
		log.Fatal(err)
	}

	// 2. Инициализация Fiber
	app := fiber.New(fiber.Config{
		AppName: "Universities KZ v1.0",
	})

	// Эндпоинт проверки здоровья
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "online",
			"db":     "connected",
		})
	})

	// 3. Запуск
	log.Fatal(app.Listen(":" + appPort))
}
