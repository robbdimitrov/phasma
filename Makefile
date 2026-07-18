.DEFAULT_GOAL := all

IMAGE_PREFIX ?= phasma
GIT_SHA ?= $(shell git rev-parse --short HEAD)

.PHONY: all
all: backend database frontend

.PHONY: help
help:
	@printf 'Phasma support targets:\n'
	@printf '  make              Build all images\n'
	@printf '  make backend      Build the backend image\n'
	@printf '  make database     Build the database image\n'
	@printf '  make frontend     Build the frontend image\n'
	@printf '  make format       Format backend Go code\n'
	@printf '  make lint         Run backend and frontend lint checks\n'
	@printf '  make test         Run backend and frontend unit tests\n'

.PHONY: backend
backend:
	docker build -t $(IMAGE_PREFIX)/backend:$(GIT_SHA) apps/backend

.PHONY: database
database:
	docker build -t $(IMAGE_PREFIX)/database:$(GIT_SHA) apps/database

.PHONY: frontend
frontend:
	docker build -t $(IMAGE_PREFIX)/frontend:$(GIT_SHA) apps/frontend

.PHONY: format
format:
	@gofmt -w $$(find apps/backend -name '*.go')

.PHONY: lint
lint:
	@test -z "$$(gofmt -l $$(find apps/backend -name '*.go'))"
	@cd apps/frontend && npm run lint

.PHONY: test
test:
	@echo "Testing backend..."
	@cd apps/backend && go test ./...
	@echo "Testing frontend..."
	@cd apps/frontend && npm test
