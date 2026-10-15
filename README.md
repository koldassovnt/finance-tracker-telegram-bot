# Family finance Telegram bot

Telegram-бот для [family-finance-crm](https://github.com/koldassovnt/family-finance-crm): доходы, расходы, переводы, платежи с напоминанием в 10:00 и списки покупок.

## Как подключить члена семьи

1. В приложении (админка или свой профиль) указать Telegram username пользователя.
2. Пользователь открывает бота и отправляет `/start`. Бот находит его по username и запоминает Telegram id и чат, поэтому после смены username в Telegram связь не рвётся. Если username поменяли в приложении, связь сбрасывается и нужен повторный `/start`.

Без `/start` бот не может написать человеку первым: так устроен Telegram.

## Запуск

Нужен бэкенд версии 1.6.0 или новее (эндпоинты `/api/v1/bot/**`) с тем же `BOT_API_TOKEN`.

```sh
cp .env.example .env   # заполнить TELEGRAM_BOT_TOKEN и BOT_API_TOKEN
```

### В Docker (основной способ)

```sh
docker compose up -d --build
docker compose logs -f bot
```

Сначала должен быть поднят compose бэкенда: бот подключается к его сети `family-finance-crm_default` и обращается к `http://family-finance-app:8080`. `BACKEND_URL` из `.env` внутри контейнера не используется, его переопределяет `compose.yaml`.

### Без Docker

```sh
go run ./cmd/family-finance-telegram-bot
```

Здесь `BACKEND_URL` берётся из `.env`, например `http://localhost:8080`.

Локально бот работает через long polling. В проде задайте `WEBHOOK_URL` (HTTPS) и `WEBHOOK_SECRET`, и бот поднимет webhook-сервер на `LISTEN_ADDR`.

## Структура

- `internal/config`: переменные окружения.
- `internal/backend`: REST-клиент бэкенда (`X-Bot-Token` + `X-Telegram-User-Id`).
- `internal/bot`: команды, кнопки и пошаговые диалоги.
- `internal/scheduler`: ежедневный запуск напоминаний.

## Проверки

```sh
go vet ./... && go test ./... && golangci-lint run
```
