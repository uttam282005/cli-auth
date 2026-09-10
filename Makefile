.PHONY: all build run test lint docker-build docker-run docker-clean clean

all: lint test build

build:
	go build -o bin/cli ./cmd/cli

run:
	go run ./cmd/cli

test:
	go test -v -race ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found. Run gofmt -w ." && exit 1)

docker-build:
	docker compose build

docker-run:
	docker compose run --rm app

docker-clean:
	docker compose down -v

clean:
	rm -rf bin/ data/
