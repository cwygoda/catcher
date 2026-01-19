.PHONY: help build run test lint clean install

APP := catcher
BIN := ./bin/$(APP)

## help: show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

## build: compile binary to ./bin/
build:
	go build -o $(BIN) ./cmd/catcher

## run: build and run with default config
run: build
	$(BIN)

## test: run all tests
test:
	go test ./...

## test-v: run all tests verbose
test-v:
	go test -v ./...

## cover: run tests with coverage
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "coverage report: coverage.html"

## lint: run golangci-lint
lint:
	golangci-lint run

## tidy: tidy and verify modules
tidy:
	go mod tidy
	go mod verify

## clean: remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

## install: install binary to GOPATH/bin
install:
	go install ./cmd/catcher
