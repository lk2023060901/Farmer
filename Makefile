.PHONY: dev dev-server dev-client infra-up infra-down db-migrate generate

# Start all services
dev: infra-up
	@echo "Starting backend and frontend..."
	@make -j2 dev-server dev-client

# Start infrastructure (PostgreSQL + Redis)
infra-up:
	docker compose up -d
	@echo "Waiting for postgres..."
	@sleep 3

infra-down:
	docker compose down

# Backend
dev-server:
	cd server && go run ./cmd/server

build-server:
	cd server && go build -o bin/server ./cmd/server

# Ent code generation
generate:
	cd server && go generate ./ent/...

# Frontend
dev-client-h5:
	cd client && npm run dev:h5

dev-client-weapp:
	cd client && npm run dev:weapp

build-client-h5:
	cd client && npm run build:h5

# Setup from scratch
setup:
	@echo "Installing server deps..."
	cd server && go mod tidy
	@echo "Installing client deps..."
	cd client && npm install
	@echo "Starting infrastructure..."
	make infra-up
	@echo "✅ Setup complete. Run 'make dev' to start."
