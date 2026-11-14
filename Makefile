STEPS ?= 1

.PHONY: env-up env-down migrate-prepare migrate-up migrate-down migrate-version seed run test test-integration
.PHONY: clickhouse-up migrate-clickhouse-up migrate-clickhouse-down migrate-clickhouse-version
.PHONY: init-env setup kafka-init storage-init analytics test-all test-web

init-env:
	sh scripts/init_env.sh

test:
	go test -mod=readonly -race ./...

test-web:
	node web/tests/api.test.mjs

test-integration:
	sh scripts/test_integration.sh

env-up: init-env
	python3 scripts/prepare_kafka_volume.py
	docker compose up -d --wait postgres jaeger prometheus grafana redis kafka clickhouse s3

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

setup: seed migrate-clickhouse-up kafka-init storage-init

kafka-init: env-up
	go run ./cmd/kafka-init

storage-init: env-up
	go run ./cmd/storage-init

analytics: setup
	go run ./cmd/booking-analytics

test-all:
	sh scripts/test_all.sh

run: setup
	go run ./cmd/TicketRush
