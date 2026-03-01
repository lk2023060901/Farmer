.PHONY: run build generate lint swag tidy

# ── Development ──────────────────────────────────────────────────────────────

## run: start the server with hot-reload (requires 'air' to be installed)
run:
	go run ./cmd/server

## build: compile the server binary to bin/server
build:
	go build -o bin/server ./cmd/server

## tidy: tidy go modules
tidy:
	go mod tidy

# ── Code generation ──────────────────────────────────────────────────────────

## generate: run all go:generate directives (Ent ORM, mocks, …)
generate:
	go generate ./ent/...

## swag: regenerate Swagger docs from source annotations
swag:
	swag init -g cmd/server/main.go -o docs/swagger

# ── Quality ──────────────────────────────────────────────────────────────────

## lint: run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run ./...

## test: run all tests
test:
	go test -race -count=1 ./...

# ── Helpers ──────────────────────────────────────────────────────────────────

## help: print this help message
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
