# TicketRush

Учебный Go backend для бронирования билетов: публичный каталог → бронь → имитация оплаты → билет с QR → однократный проход. Проект показывает транзакции, конкурентный доступ, разграничение прав, Redis и асинхронную аналитику.

## Быстрые ссылки

- [Путеводитель: как работает проект и как его разбирать](PROJECT_GUIDE.md)
- [Карта файлов: назначение каждого файла и основные функции](FILE_MAP.md)
- [HTTP smoke-сценарий](scripts/smoke.py)
- [Полные изолированные проверки](scripts/test_all.sh)

## Стек

Go 1.27.1, chi, pgx/PostgreSQL, Argon2id, JWT HS256 + opaque refresh tokens, Redis, kafka-go, ClickHouse, S3 API (локально SeaweedFS), Prometheus/Grafana, OpenTelemetry/Jaeger, zap, Docker Compose.

Это учебный MVP: `payment_mode=mock`, реального списания денег нет. Frontend, настоящий платёжный провайдер, возвраты, gRPC и Kubernetes в эту версию не входят.

## Запуск

Нужны Go 1.27.1, Docker с Compose V2, Make, Python 3 и OpenSSL. Для smoke-проверки также используется curl.

```sh
make init-env
# В .env: DEV_AUTH_ENABLED=false и MOCK_PAYMENTS_ENABLED=true
make run
```

`init-env` создаёт отсутствующий `.env` из шаблона и генерирует секрет JWT; существующий файл сохраняется. Если `.env` уже существует, проверь его значения по `.env.example`. Реальные секреты хранятся только локально.

`make run` поднимает инфраструктуру, применяет миграции PostgreSQL и ClickHouse, создаёт топик Kafka и приватный S3 bucket, добавляет demo-данные и запускает API. По умолчанию API слушает `127.0.0.1:8080`.

Во втором терминале запусти consumer:

```sh
make analytics
```

Он нужен для обновления отчётов. Бронирование, подтверждение и выдача билетов работают через PostgreSQL, даже если consumer остановлен. После его запуска накопленные события попадут в ClickHouse.

Если используется старый Kafka-контейнер без тома, `env-up` сначала останавливает его и переносит данные в том, оставляя резервную копию в игнорируемом `tmp/kafka-backup-*`. При конфликте с непустым целевым томом перенос останавливается без перезаписи данных. Для контейнера, изначально созданного новой версией Compose, перенос не нужен.

Не используй `docker compose down -v`, если хочешь сохранить данные.

## Demo

```sh
python3 scripts/smoke.py
```

Скрипт создаёт покупателя, проходит настоящий JWT login, покупает два билета через mock checkout, проверяет повторное подтверждение и повторный проход, загружает афишу в S3 и ждёт обновления отчёта. Используются demo-мероприятие и два служебных аккаунта:

| Аккаунт | Email | Пароль | Доступ |
|---|---|---|---|
| Организатор | organizer@example.test | TicketRush-demo-2026! | Мероприятия demo-организации, контроль входа |
| Владелец площадки | venue-owner@example.test | TicketRush-demo-2026! | Создание площадок |

Эти опубликованные данные предназначены только для локального demo. Организатор и владелец площадки — разные права. `DEV_AUTH_ENABLED=true` — отдельный локальный режим, подставляющий identity; smoke требует `false`.

Demo-мероприятие: `55555555-5555-4555-8555-555555555555`; сеанс: `66666666-6666-4666-8666-666666666666`; тариф: `77777777-7777-4777-8777-777777777777`; организация: `33333333-3333-4333-8333-333333333333`. При первом seed сеанс создаётся через 30 дней. Повторный seed не сбрасывает даты, запасы, статусы и созданные данные.

## Основные ручки

Защищённые запросы используют `Authorization: Bearer <access_token>`. Ручки организатора дополнительно требуют `X-Organizer-ID`; доступ проверяется в PostgreSQL.

| Метод и путь | Назначение |
|---|---|
| POST /api/v1/auth/register | Регистрация |
| POST /api/v1/auth/login | Access и refresh tokens |
| POST /api/v1/auth/refresh | Ротация refresh token |
| POST /api/v1/auth/logout | Отзыв auth-сессии |
| GET /api/v1/events | Публичный каталог |
| GET /api/v1/events/{event_id} | Опубликованное мероприятие |
| GET /api/v1/events/{event_id}/sessions | Опубликованные сеансы |
| GET /api/v1/events/{event_id}/sessions/{session_id}/ticket-types | Тарифы сеанса |
| POST /api/v1/events/{event_id}/sessions/{session_id}/bookings | Удержать билеты: ticket_type_id, quantity (1–10) |
| GET /api/v1/bookings/{booking_id} | Своя бронь с позициями и ценой на момент бронирования |
| POST /api/v1/bookings/{booking_id}/cancel | Отменить свою действующую бронь |
| POST /api/v1/bookings/{booking_id}/confirm | Mock checkout; включается MOCK_PAYMENTS_ENABLED |
| GET /api/v1/me/tickets?limit=20&offset=0 | Свои билеты |
| GET /api/v1/tickets/{ticket_id} | Свой билет и qr_payload |
| GET /api/v1/tickets/{ticket_id}/qr | PNG с подписанным QR payload |
| POST /api/v1/controller/sessions/{session_id}/check-ins | Однократный проход: JSON {"token":"<qr_payload>"} |
| POST /api/v1/organizer/events/{event_id}/poster | Афиша: raw PNG/JPEG body, Content-Type image/png или image/jpeg, до 5 MiB |
| GET /api/v1/events/{event_id}/poster | Афиша опубликованного мероприятия через API |
| GET /api/v1/organizer/events/{event_id}/analytics/bookings?from=...&to=... | Отчёт по действиям за период |
| POST /api/v1/venues | Создать площадку; право ownerVenue из user_roles |
| GET /livez, GET /readyz | Процесс жив / PostgreSQL доступен |
| GET /metrics | Prometheus |

CRUD и публикация мероприятий, сеансов и тарифов находятся под `/api/v1/organizer/events/...`; полный список маршрутов — в `cmd/TicketRush/main.go`. Для работы с обычным каталогом JWT не нужен.

`confirm` возвращает 200 и `payment_mode: mock`; повторный запрос к уже подтверждённой своей брони возвращает тот же статус без повторного списания инвентаря, билетов и outbox. QR доступен владельцу активного билета. После первого check-in статус `used`, повторный check-in возвращает 409. Сотрудник проверяется по текущему членству `organizer/controller` именно в организации данного сеанса.

Отчёт принимает RFC3339 `from` и `to`, период `[from,to)`. В ответе `created`, `confirmed`, `cancelled`, `expired`, в каждом `bookings` и `tickets`. Это число действий за период, а не текущий баланс остатков. Например, вчерашняя бронь, отменённая сегодня, входит в сегодняшние отмены. Данные приходят с задержкой outbox/consumer.

## Команды и порты

| Команда | Действие |
|---|---|
| make setup | Инфраструктура, обе схемы БД, топик, bucket, demo-данные |
| make run | setup и HTTP API |
| make analytics | setup и consumer |
| make test | Go-тесты с race detector; интеграционные без test env пропускаются |
| make test-integration | Отдельный временный PostgreSQL и все Go-тесты |
| make test-all | Отдельные PostgreSQL, Redis, Kafka, ClickHouse, S3; интеграционные тесты и полный HTTP smoke |
| make migrate-up / migrate-down STEPS=1 | PostgreSQL |
| make migrate-clickhouse-up / migrate-clickhouse-down STEPS=1 | ClickHouse |
| make env-down | Остановить Compose, сохранив named volumes |

Локальные порты: API 8080, PostgreSQL 5433, Redis 6380, Kafka 9092, ClickHouse 9000/8123, S3 9002, Grafana 3000, Prometheus 9090, Jaeger 16686. База ClickHouse в текущих SQL-миграциях называется `ticketrush`. S3 endpoint задаётся как `host:port`, TLS переключает `S3_SECURE`.

`make test-all` использует случайные локальные порты и удаляет только собственные временные контейнеры. Он не применяет миграции к рабочей БД и не меняет `.env`. При ошибке HTTP smoke путь к временным логам выводится в терминал.

## Границы версии

- Один outbox publisher, один dev-брокер Kafka; это не HA-конфигурация.
- Доставка at least once: дубликаты возможны, отчёт удаляет их по неизменному message_id до суммирования.
- Неизвестное/повреждённое Kafka-сообщение останавливает consumer; DLQ пока отсутствует.
- Подтверждение идемпотентно по booking_id. Универсальной Idempotency-Key для создания брони пока нет: повторный POST создания может создать новую бронь.
- Роли назначаются администратором через БД; самостоятельной регистрации организаторов/владельцев и управления сотрудниками через API нет.
- Публичный event cache допускает устаревшие метаданные до TTL; создание брони всегда повторно проверяет статусы и остатки в PostgreSQL.
- Старые версии афиш автоматически не удаляются; при ошибке записи метаданных новый объект удаляется по возможности. Полноценного GC ещё нет.
- Ротация JWT-секрета меняет производный ключ QR: прежние QR станут недействительны, владелец сможет получить новый. Для production понадобятся отдельный жизненный цикл QR-ключей и HTTPS.

S3 adapter использует [minio-go SDK](https://github.com/minio/minio-go) и совместимый API; локальный сервер — [SeaweedFS](https://github.com/seaweedfs/seaweedfs/tree/4.53), а не сервер MinIO. QR кодируется через [go-qrcode](https://github.com/skip2/go-qrcode).
