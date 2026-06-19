.PHONY: dev build test lint vet tidy up down logs status

dev:
	go run ./cmd/server/...

build:
	go build -o bin/raggo ./cmd/server/...
	go build -o bin/raggo-mcp ./cmd/mcp/...

test:
	go test ./... -v -count=1

lint:
	go vet ./...

tidy:
	go mod tidy

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f api

status:
	docker compose ps
