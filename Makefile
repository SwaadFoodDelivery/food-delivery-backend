COMPOSE_FILE ?= docker-compose.local.yml

.PHONY: compose-up compose-down compose-logs up down logs reset run test lint

compose-up:
	docker compose -f $(COMPOSE_FILE) up -d

compose-down:
	docker compose -f $(COMPOSE_FILE) down

compose-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

up: compose-up

down: compose-down

logs: compose-logs

reset:
	docker compose -f $(COMPOSE_FILE) down -v --remove-orphans
	docker compose -f $(COMPOSE_FILE) up -d --build

run:
	go run ./cmd/server

test:
	go test -race ./...
lint:
	golangci-lint run
