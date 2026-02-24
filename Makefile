.PHONY: build run test lint migrate-up migrate-down docker-up docker-down

run:
	go run cmd/api/main.go

build:
	go build -o bin/api cmd/api/main.go

test:
	go test ./... -v -cover

lint:
	golangci-lint run

migrate-up:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shortener?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://user:password@localhost:5432/shortener?sslmode=disable" down

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down