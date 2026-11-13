#!/bin/sh
# A new disposable database is used every time. The application database is untouched.
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
container_id=
task_api_pid=
task_analytics_pid=
task_binary_dir=

cleanup() {
    if [ -n "$task_api_pid" ]; then kill "$task_api_pid" 2>/dev/null || true; wait "$task_api_pid" 2>/dev/null || true; fi
    if [ -n "$task_analytics_pid" ]; then kill "$task_analytics_pid" 2>/dev/null || true; wait "$task_analytics_pid" 2>/dev/null || true; fi
    if [ -n "$container_id" ]; then
        docker stop --time 5 "$container_id" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

container_id=$(docker run --rm -d \
    --publish 127.0.0.1::5432 \
    --env POSTGRES_USER=ticketrush_test \
    --env POSTGRES_PASSWORD=integration_test \
    --env POSTGRES_DB=ticketrush_test \
    postgres:18)

attempt=0
# The initialization server accepts Unix-socket connections before the target
# database is ready. Wait for a successful query over the final TCP listener.
until docker exec -e PGPASSWORD=integration_test "$container_id" \
    psql -h 127.0.0.1 -XAt -U ticketrush_test -d ticketrush_test -c 'SELECT 1' >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
        echo "Test PostgreSQL did not become ready" >&2
        docker logs "$container_id" >&2
        exit 1
    fi
    sleep 1
done

for migration in "$repo_dir"/migrations/*.up.sql; do
    docker exec -i "$container_id" psql -X -q -v ON_ERROR_STOP=1 \
        -U ticketrush_test -d ticketrush_test < "$migration"
done

# Verify that development fixtures match the latest schema and can be reapplied.
for seed_attempt in 1 2; do
    docker exec -i "$container_id" psql -X -q -v ON_ERROR_STOP=1 \
        -U ticketrush_test -d ticketrush_test < "$repo_dir/scripts/seed.sql"
done

port=$(docker port "$container_id" 5432/tcp)
port=${port##*:}
export TICKETRUSH_TEST_DATABASE_URL="postgres://ticketrush_test:integration_test@127.0.0.1:$port/ticketrush_test?sslmode=disable"

go test -mod=readonly -race -count=1 ./... "$@"

if [ "${TICKETRUSH_RUN_SMOKE:-0}" = 1 ]; then
    task_binary_dir=$(mktemp -d /tmp/ticketrush-smoke.XXXXXX)
    export DATABASE_URL="$TICKETRUSH_TEST_DATABASE_URL" DATABASE_TIMEOUT=5s
    export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:1 OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://127.0.0.1:1/v1/traces
    export LOGGER_FOLDER="$task_binary_dir/logs" LOGGER_LEVEL=INFO
    export JWT_SECRET_HEX="$(openssl rand -hex 32)" DEV_AUTH_ENABLED=false MOCK_PAYMENTS_ENABLED=true
    task_api_port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
    export HTTP_ADDR="127.0.0.1:$task_api_port" TICKETRUSH_SMOKE_URL="http://127.0.0.1:$task_api_port"
    go build -o "$task_binary_dir/kafka-init" ./cmd/kafka-init
    go build -o "$task_binary_dir/api" ./cmd/TicketRush
    go build -o "$task_binary_dir/analytics" ./cmd/booking-analytics
    task_attempt=0
    until "$task_binary_dir/kafka-init" >"$task_binary_dir/kafka-init.log" 2>&1; do
        task_attempt=$((task_attempt+1)); if [ "$task_attempt" -ge 12 ]; then cat "$task_binary_dir/kafka-init.log"; exit 1; fi
        sleep 1
    done
    go run ./cmd/storage-init
    "$task_binary_dir/api" >"$task_binary_dir/api.log" 2>&1 & task_api_pid=$!
    "$task_binary_dir/analytics" >"$task_binary_dir/analytics.log" 2>&1 & task_analytics_pid=$!
    task_attempt=0
    until curl -fsS "$TICKETRUSH_SMOKE_URL/readyz" >/dev/null 2>&1; do
        task_attempt=$((task_attempt+1)); if [ "$task_attempt" -ge 30 ]; then cat "$task_binary_dir/api.log"; exit 1; fi
        sleep 1
    done
    if ! python3 scripts/smoke.py; then
        echo "Smoke logs: $task_binary_dir" >&2
        cat "$task_binary_dir/analytics.log" >&2
        exit 1
    fi
fi
