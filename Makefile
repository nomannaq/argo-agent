.PHONY: build run test lint clean tidy

build:
	go build -o bin/argo ./cmd/argo

run:
	go run ./cmd/argo

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

tidy:
	go mod tidy
