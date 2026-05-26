COMPOSE_FILE ?= docker-compose.local.yml
POSTGRES_CONTAINER ?= food-delivery-postgres
REDIS_CONTAINER ?= food-delivery-redis
KAFKA_CONTAINER ?= food-delivery-kafka
ZOOKEEPER_CONTAINER ?= food-delivery-zookeeper
MINIO_CONTAINER ?= food-delivery-minio
BACKEND_CONTAINER ?= food-delivery-backend
POSTGRES_DB ?= food_delivery
POSTGRES_USER ?= postgres

.PHONY: compose-up compose-down compose-logs compose-ps compose-restart compose-build up down logs reset run test lint \
postgres-connect postgres-shell redis-connect redis-monitor redis-flushall \
kafka-logs zookeeper-logs backend-logs minio-logs service-logs \
backend-shell minio-shell

compose-up:
	docker compose -f $(COMPOSE_FILE) up -d

compose-down:
	docker compose -f $(COMPOSE_FILE) down

compose-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

compose-ps:
	docker compose -f $(COMPOSE_FILE) ps

compose-restart:
	docker compose -f $(COMPOSE_FILE) restart

compose-build:
	docker compose -f $(COMPOSE_FILE) up -d --build

up: compose-up

down: compose-down

logs: compose-logs

reset:
	docker compose -f $(COMPOSE_FILE) down -v --remove-orphans
	docker compose -f $(COMPOSE_FILE) up -d --build

postgres-connect:
	docker exec -it $(POSTGRES_CONTAINER) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

postgres-shell:
	docker exec -it $(POSTGRES_CONTAINER) sh

redis-connect:
	docker exec -it $(REDIS_CONTAINER) redis-cli

redis-monitor:
	docker exec -it $(REDIS_CONTAINER) redis-cli MONITOR

redis-flushall:
	docker exec -it $(REDIS_CONTAINER) redis-cli FLUSHALL

kafka-logs:
	docker logs -f $(KAFKA_CONTAINER)

zookeeper-logs:
	docker logs -f $(ZOOKEEPER_CONTAINER)

backend-logs:
	docker logs -f $(BACKEND_CONTAINER)

minio-logs:
	docker logs -f $(MINIO_CONTAINER)

service-logs:
	docker compose -f $(COMPOSE_FILE) logs -f $(svc)

backend-shell:
	docker exec -it $(BACKEND_CONTAINER) sh

minio-shell:
	docker exec -it $(MINIO_CONTAINER) sh

run:
	go run ./cmd/server

test:
	go test -race ./...
lint:
	golangci-lint run
