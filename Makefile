STEPS ?= 1

.PHONY: env-up env-down migrate-prepare migrate-up migrate-down migrate-version seed run test test-integration

test:
	go test -mod=readonly -race ./...

test-integration:
	sh scripts/test_integration.sh

env-up:
	docker compose up -d --wait postgres jaeger prometheus grafana redis

env-down:
	docker compose down

migrate-prepare: env-up
	docker compose exec -T postgres sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < scripts/migration_prepare.sql

migrate-up: migrate-prepare
	docker compose run --rm migrate up

migrate-down: env-up
	docker compose run --rm migrate down $(STEPS)

migrate-version: env-up
	docker compose run --rm migrate version

seed: migrate-up
	docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < scripts/seed.sql

run: seed
	go run ./cmd/TicketRush
