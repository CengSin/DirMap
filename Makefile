APP_NAME := system-agent-rag
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run clean test tidy docker-build docker-up docker-down docker-logs

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(APP_NAME) .

run: build
	./bin/$(APP_NAME) -config config.yaml

clean:
	rm -rf bin/

test:
	go test ./...

tidy:
	go mod tidy

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
