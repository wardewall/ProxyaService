# ProxyService Telegram Bot (Go)

Телеграм‑бот на Go, который выдаёт кнопку подключения к SOCKS5‑прокси, а также поддерживает аутентификацию по токенам, роли и rate‑limit. Есть хранение в PostgreSQL, docker-compose для локального запуска.

## Быстрый старт

1) Установить Go 1.24+
2) Создать `.env` (см. ниже) или использовать переменные окружения
3) Локальный запуск:
```bash
go run .
```

Или через Docker:
```bash
docker compose up -d --build
docker compose logs -f app
```

## Переменные окружения (.env пример)
```env
BOT_TOKEN=123456:ABCDEF
PROXY_HOST=your.server.com
PROXY_PORT=1080
PROXY_USER=
PROXY_PASS=
AUTH_TOKENS=secret1,secret2
ALLOWED_USER_IDS=123456789
LOG_LEVEL=info

# Postgres (compose использует внутренний DSN ниже)
PG_DSN=postgres://postgres:postgres@localhost:5432/proxyabot?sslmode=disable

# Роли и лимиты
DEFAULT_ROLE=free
RATE_LIMIT_FREE_PER_MIN=10
RATE_LIMIT_PREMIUM_PER_MIN=60
RATE_LIMIT_ADMIN_PER_MIN=500
THROTTLE_SECONDS=2
```

> В Docker Compose приложение использует DSN `postgres://postgres:postgres@db:5432/proxyabot?sslmode=disable` (контейнер `db`).

## Команды бота
- `/start` — главное меню
- `/proxy` — отправить кнопку подключения к прокси
- `/disable` — как отключить прокси в Telegram
- `/status` — роль и состояние аутентификации
- `/auth <token>` — аутентификация токеном
- `/issue_token <role> [ttl]` — выдать одноразовый токен (для админов)

## Docker
- `Dockerfile` — multistage build, статический бинарь
- `docker-compose.yml` — сервисы `app` и `db` (PostgreSQL 16), `healthcheck`

## Разработка
```bash
go mod tidy
go build ./...
```

## Безопасность
- Не коммитьте `.env`. Для продакшна используйте секреты/Vault/CI‑vars.
- Откройте порт PostgreSQL наружу только при необходимости.

## Лицензия
MIT
