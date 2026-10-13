#!/bin/sh
# A new disposable database is used every time. The application database is untouched.
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
container_id=

cleanup() {
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
until docker exec "$container_id" pg_isready -U ticketrush_test -d ticketrush_test >/dev/null 2>&1; do
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

port=$(docker port "$container_id" 5432/tcp)
port=${port##*:}
export TICKETRUSH_TEST_DATABASE_URL="postgres://ticketrush_test:integration_test@127.0.0.1:$port/ticketrush_test?sslmode=disable"

go test -mod=readonly -race -count=1 ./... "$@"
