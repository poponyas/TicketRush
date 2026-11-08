STEPS ?= 1

.PHONY: env-up env-down migrate-prepare migrate-up migrate-down migrate-version seed run test test-integration
.PHONY: clickhouse-up migrate-clickhouse-up migrate-clickhouse-down migrate-clickhouse-version

test:
	go test -mod=readonly -race ./...

test-integration:
	sh scripts/test_integration.sh

env-up:
	docker compose up -d --wait postgres jaeger prometheus grafana redis kafka clickhouse

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

clickhouse-up:
	docker compose up -d --wait clickhouse

migrate-clickhouse-up: clickhouse-up
	docker compose run --rm migrate-clickhouse up

migrate-clickhouse-down: clickhouse-up
	docker compose run --rm migrate-clickhouse down $(STEPS)

migrate-clickhouse-version: clickhouse-up
	docker compose run --rm migrate-clickhouse version

seed: migrate-up
	docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < scripts/seed.sql

run: seed
	go run ./cmd/TicketRush
