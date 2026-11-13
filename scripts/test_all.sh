#!/bin/sh
# Fully isolated dependencies; application volumes and .env are not modified.
set -eu
cd "$(dirname "$0")/.."
task_test_containers=
cleanup_all() {
    for task_test_container in $task_test_containers; do
        docker stop --time 5 "$task_test_container" >/dev/null 2>&1 || true
    done
}
trap cleanup_all EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

task_ch=$(docker run --rm -d -p 127.0.0.1::9000 -e CLICKHOUSE_DB=ticketrush -e CLICKHOUSE_USER=analytics_test -e CLICKHOUSE_PASSWORD=integration_test -e CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1 clickhouse:26.9.27.3)
task_test_containers="$task_ch"
task_s3=$(docker run --rm -d -p 127.0.0.1::8333 -e AWS_ACCESS_KEY_ID=integration_test -e AWS_SECRET_ACCESS_KEY=integration_test_secret ghcr.io/chrislusf/seaweedfs:4.53 mini -dir=/data)
task_test_containers="$task_test_containers $task_s3"
task_redis=$(docker run --rm -d -p 127.0.0.1::6379 redis:8.10.3-alpine)
task_test_containers="$task_test_containers $task_redis"
task_kafka_port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
task_kafka=$(docker run --rm -d -p "127.0.0.1:$task_kafka_port:9092" -e "KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:$task_kafka_port" -e KAFKA_NODE_ID=1 -e KAFKA_PROCESS_ROLES=broker,controller -e KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093 -e KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 -e KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0 -e "KAFKA_HEAP_OPTS=-Xms256m -Xmx256m" apache/kafka:4.4.0)
task_test_containers="$task_test_containers $task_kafka"
task_ch_port=$(docker port "$task_ch" 9000/tcp)
task_s3_port=$(docker port "$task_s3" 8333/tcp)
task_redis_port=$(docker port "$task_redis" 6379/tcp)
export TICKETRUSH_TEST_CLICKHOUSE_ADDR="127.0.0.1:${task_ch_port##*:}"
export TICKETRUSH_TEST_CLICKHOUSE_USER=analytics_test TICKETRUSH_TEST_CLICKHOUSE_PASSWORD=integration_test
export TICKETRUSH_TEST_S3_ENDPOINT="127.0.0.1:${task_s3_port##*:}"
export KAFKA_BROKERS="127.0.0.1:$task_kafka_port"
export REDIS_TIMEOUT=1s CLICKHOUSE_TIMEOUT=5s S3_TIMEOUT=5s S3_SECURE=false
export REDIS_ADDR="127.0.0.1:${task_redis_port##*:}"
export S3_ENDPOINT="$TICKETRUSH_TEST_S3_ENDPOINT" S3_ACCESS_KEY=integration_test S3_SECRET_KEY=integration_test_secret S3_BUCKET=ticketrush-posters
export CLICKHOUSE_ADDR="$TICKETRUSH_TEST_CLICKHOUSE_ADDR" CLICKHOUSE_DB=ticketrush CLICKHOUSE_USER=analytics_test CLICKHOUSE_PASSWORD=integration_test
task_attempt=0
until docker exec "$task_ch" sh -c 'clickhouse-client --host "$(hostname -i)" --user analytics_test --password integration_test --database ticketrush --query "SELECT 1"' >/dev/null 2>&1 && docker exec "$task_s3" wget -q -O /dev/null http://127.0.0.1:9333/dir/status >/dev/null 2>&1; do
    task_attempt=$((task_attempt+1))
    if [ "$task_attempt" -ge 90 ]; then echo 'Test dependencies did not become ready' >&2; exit 1; fi
    sleep 1
done
docker exec "$task_ch" clickhouse-client --host 127.0.0.1 --user analytics_test --password integration_test --query 'CREATE DATABASE IF NOT EXISTS ticketrush'
docker exec -i "$task_ch" clickhouse-client --host 127.0.0.1 --user analytics_test --password integration_test --multiquery < clickhouse/migrations/001_booking_events.up.sql
export TICKETRUSH_RUN_SMOKE=1
if ! sh scripts/test_integration.sh; then
    docker logs --tail 60 "$task_kafka" >&2
    exit 1
fi
