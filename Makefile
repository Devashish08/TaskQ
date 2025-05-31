# Makefile for TaskQ Project

# Go parameters
BINARY_NAME=taskq-server
BINARY_DIR=./bin
CMD_PATH=./cmd/taskq-server

# Default target executed when you just run `make`
.PHONY: all
all: help

# Target to run the application
.PHONY: run
run:
	@echo "Running TaskQ server..."
	@DB_HOST=localhost DB_PORT=5432 DB_USER=taskq_user DB_PASSWORD=taskq_password DB_NAME=taskq_db \
	REDIS_ADDR=localhost:6379 REDIS_DB=0 \
	go run $(CMD_PATH)/main.go

# Target to build the application binary
.PHONY: build
build:
	@echo "Building TaskQ server binary..."
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_DIR)/$(BINARY_NAME) $(CMD_PATH)/main.go
	@echo "Binary available at $(BINARY_DIR)/$(BINARY_NAME)"

# Target to run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./... -v

# Target to run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Target to format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

# Target to clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_DIR)/$(BINARY_NAME)
	@echo "Cleaned."

# Docker Compose targets
.PHONY: docker-up
docker-up:
	@echo "Starting Docker services (PostgreSQL & Redis)..."
	@sudo docker compose up -d

.PHONY: docker-down
docker-down:
	@echo "Stopping Docker services..."
	@sudo docker compose down

.PHONY: docker-logs-db
docker-logs-db:
	@echo "Tailing PostgreSQL logs..."
	@sudo docker compose logs -f db

.PHONY: docker-logs-redis
docker-logs-redis:
	@echo "Tailing Redis logs..."
	@sudo docker compose logs -f redis

.PHONY: docker-ps
docker-ps:
	@echo "Docker services status:"
	@sudo docker compose ps

# Target to enter psql shell for the database
.PHONY: psql
psql: docker-up
	@echo "Connecting to PostgreSQL (taskq_db)..."
	@echo "Password is: taskq_password (or as defined in docker-compose.yml)"
	@psql -h localhost -p 5432 -U taskq_user -d taskq_db

# Target to enter redis-cli for Redis
.PHONY: redis-cli
redis-cli: docker-up
	@echo "Connecting to Redis..."
	@sudo docker compose exec redis redis-cli

# Help target to display available commands
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make run            - Run the Go application"
	@echo "  make build          - Build the Go application binary"
	@echo "  make test           - Run Go tests"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make fmt            - Format Go code"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make docker-up      - Start Docker services (db, redis) in detached mode"
	@echo "  make docker-down    - Stop Docker services"
	@echo "  make docker-ps      - Show status of Docker services"
	@echo "  make docker-logs-db - Tail logs from the PostgreSQL container"
	@echo "  make docker-logs-redis - Tail logs from the Redis container"
	@echo "  make psql           - Connect to PostgreSQL database via psql"
	@echo "  make redis-cli      - Connect to Redis via redis-cli"
	@echo "  make help           - Show this help message"