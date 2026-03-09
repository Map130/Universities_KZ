# ──────────────────────────────────────────────────────────────
#  Universities KZ — Makefile
# ──────────────────────────────────────────────────────────────

.PHONY: help build run swagger swagger-install lint test clean tidy

# По умолчанию показываем справку
help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Build & Run ──────────────────────────────────────────────

build: ## Собрать бинарник
	go build -o bin/api ./cmd/api

run: swagger build ## Сгенерировать Swagger и запустить сервер
	./bin/api

# ── Swagger ──────────────────────────────────────────────────

SWAG := $(shell command -v swag 2>/dev/null || echo "$(HOME)/go/bin/swag")

swagger-install: ## Установить swag CLI (если отсутствует)
	@command -v swag >/dev/null 2>&1 && echo "swag already installed" || \
		go install github.com/swaggo/swag/cmd/swag@latest

swagger: swagger-install ## Сгенерировать OpenAPI/Swagger спецификацию
	$(SWAG) init \
		--generalInfo cmd/api/main.go \
		--parseDependency \
		--parseInternal \
		--output docs/swagger
	@echo ""
	@echo "  ✅ Swagger spec generated:"
	@echo "     docs/swagger/swagger.json"
	@echo "     docs/swagger/swagger.yaml"
	@echo "     docs/swagger/docs.go"
	@echo ""
	@echo "  🌐 Swagger UI: http://localhost:3000/swagger/index.html"

swagger-fmt: ## Отформатировать Swagger-аннотации в исходниках
	$(SWAG) fmt --dir internal/handlers --dir cmd/api

# ── Quality ──────────────────────────────────────────────────

tidy: ## go mod tidy
	go mod tidy

lint: ## Запустить go vet
	go vet ./...

test: ## Запустить тесты
	go test -v -race ./...

# ── Clean ────────────────────────────────────────────────────

clean: ## Удалить артефакты сборки
	rm -rf bin/
	@echo "  🧹 cleaned"
