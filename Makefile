.PHONY: help build run test test-integration test-integration-docker clean docker-build docker-up docker-down docker-logs lint fmt deps

# Цвета для вывода
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m

help: ## Показать справку
	@echo "$(GREEN)Доступные команды:$(NC)"
	@echo "  $(YELLOW)build$(NC)                     - Собрать приложение"
	@echo "  $(YELLOW)run$(NC)                       - Запустить приложение локально"
	@echo "  $(YELLOW)test$(NC)                      - Запустить unit тесты"
	@echo "  $(YELLOW)test-integration$(NC)          - Запустить интеграционные тесты"
	@echo "  $(YELLOW)test-integration-docker$(NC)   - Запустить интеграционные тесты через Docker"
	@echo "  $(YELLOW)test-all$(NC)                  - Запустить все тесты"
	@echo "  $(YELLOW)clean$(NC)                     - Очистить артефакты сборки"
	@echo "  $(YELLOW)docker-build$(NC)              - Собрать Docker образ"
	@echo "  $(YELLOW)docker-up$(NC)                 - Запустить через docker-compose"
	@echo "  $(YELLOW)docker-down$(NC)               - Остановить контейнеры"
	@echo "  $(YELLOW)docker-logs$(NC)               - Показать логи приложения"
	@echo "  $(YELLOW)lint$(NC)                      - Запустить линтер"
	@echo "  $(YELLOW)fmt$(NC)                       - Форматировать код"
	@echo "  $(YELLOW)deps$(NC)                      - Установить зависимости"

build: ## Собрать приложение
	@echo "$(GREEN)Сборка приложения...$(NC)"
	@go build -o bin/server ./cmd/server
	@echo "$(GREEN)Готово: bin/server$(NC)"

run: build ## Запустить приложение локально
	@echo "$(GREEN)Запуск приложения...$(NC)"
	@./bin/server

test: ## Запустить unit тесты
	@echo "$(GREEN)Запуск unit тестов...$(NC)"
	@go test -v ./internal/domain/

test-integration: ## Запустить интеграционные тесты (требуется PostgreSQL на localhost)
	@echo "$(GREEN)Запуск интеграционных тестов...$(NC)"
	@echo "$(YELLOW)Убедитесь что PostgreSQL запущен на localhost:5432$(NC)"
	@TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_service?sslmode=disable" \
		go test -v -count=1 ./test/integration/

test-integration-docker: ## Запустить интеграционные тесты через Docker
	@echo "$(GREEN)Запуск PostgreSQL для тестов...$(NC)"
	@docker-compose up -d postgres
	@echo "$(YELLOW)Ожидание готовности PostgreSQL...$(NC)"
	@sleep 5
	@echo "$(GREEN)Запуск интеграционных тестов...$(NC)"
	@TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/pr_service?sslmode=disable" \
		go test -v -count=1 ./test/integration/
	@echo "$(GREEN)Тесты завершены$(NC)"


test-all: test test-integration-docker  ## Запустить все тесты

clean: ## Очистить артефакты сборки
	@echo "$(GREEN)Очистка артефактов...$(NC)"
	@rm -rf bin/
	@go clean
	@echo "$(GREEN)Очистка завершена$(NC)"

docker-build: ## Собрать Docker образ
	@echo "$(GREEN)Сборка Docker образа...$(NC)"
	@docker-compose build
	@echo "$(GREEN)Образ собран$(NC)"

docker-up: ## Запустить через docker-compose
	@echo "$(GREEN)Запуск docker-compose...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)Сервис запущен на http://localhost:8080$(NC)"
	@echo "Проверка: curl http://localhost:8080/health"

docker-down: ## Остановить контейнеры
	@echo "$(GREEN)Остановка docker-compose...$(NC)"
	@docker-compose down

docker-logs: ## Показать логи приложения
	@docker-compose logs -f app

lint: ## Запустить линтер
	@echo "$(GREEN)Запуск golangci-lint...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint не установлен$(NC)"; \
		echo "Установка: https://golangci-lint.run/usage/install/"; \
		echo "macOS: brew install golangci-lint"; \
		echo "Linux: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
		exit 1; \
	fi

fmt: ## Форматировать код
	@echo "$(GREEN)Форматирование кода...$(NC)"
	@go fmt ./...

deps: ## Установить зависимости
	@echo "$(GREEN)Установка зависимостей...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Зависимости установлены$(NC)"