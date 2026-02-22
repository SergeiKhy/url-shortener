.PHONY: build run test migrate-up migrate-down docker-up docker-down

run:
	go run cmd/api/main.go

build:
	go build -o bin/api cmd/api/main.go

test:
	go test ./... -v -cover

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Запуск миграций (требуется установленный migrate CLI)
migrate-up:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shortener?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shortener?sslmode=disable" down