COMPOSE := docker compose
COMPOSE_PROD := docker compose -f docker-compose.yml -f docker-compose.prod.yml
TOOLS := $(COMPOSE) run --rm --no-deps tools
TOOLS_DB := $(COMPOSE) run --rm tools

.PHONY: up down logs build prod sqlc migrate migrate-down test shell-api shell-tools tidy

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f api web db

build:
	$(COMPOSE_PROD) build app

prod:
	$(COMPOSE_PROD) up --build -d db app

sqlc:
	$(TOOLS) "sqlc -f db/sqlc.yaml generate"

migrate:
	$(TOOLS_DB) 'migrate -path /app/db/migrations -database "$$DATABASE_URL" up'

migrate-down:
	$(TOOLS_DB) 'migrate -path /app/db/migrations -database "$$DATABASE_URL" down 1'

test:
	$(TOOLS_DB) "go test -buildvcs=false -tags=integration ./..."

shell-api:
	$(COMPOSE) exec api sh

shell-tools:
	$(TOOLS) "sh"

tidy:
	$(TOOLS) "go mod tidy"
