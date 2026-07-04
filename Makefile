.PHONY: run build test test-cover vet tidy docker

run:
	go run ./cmd/api/...

build:
	go build -o bin/api ./cmd/api/...
	go build -o bin/healthcheck ./cmd/healthcheck/...

test:
	go test ./...

test-cover:
	go test -cover ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker:
	docker build -t afwilto-certifications-mngr .
