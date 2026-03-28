APP_NAME := mygo
COMPOSE_FILE := deployments/docker-compose.yml

.PHONY: run fmt test docker-up docker-down docker-logs docker-ps

run:
	go run ./cmd/server

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

docker-up:
	docker compose -f $(COMPOSE_FILE) up -d --build

docker-down:
	docker compose -f $(COMPOSE_FILE) down -v

docker-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

docker-ps:
	docker compose -f $(COMPOSE_FILE) ps
