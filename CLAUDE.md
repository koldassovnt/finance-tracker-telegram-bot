# Project conventions

## Go style
- Follow standard Go formatting (`gofmt`/`goimports`); no custom formatting rules.
- Prefer explicit error handling over panics; wrap errors with `fmt.Errorf("...: %w", err)`.
- Keep package boundaries small and focused; avoid a god-package.
- Use context.Context as the first parameter for any function that does I/O or calls the Telegram API.
- Run `go vet` and `golangci-lint run` before considering a change done.

## Deployment
- Deploy with `./deploy.sh`, never a bare `docker compose up --build`. The image is `family-finance-telegram-bot:<version>`, the number in the `VERSION` file, which the script copies into `.env` as `VERSION` — bump it for every deploy.

## Telegram Bot API
- Never hardcode the bot token — read it from an environment variable (e.g. `TELEGRAM_BOT_TOKEN`).
- Use a maintained client library (e.g. `go-telegram-bot-api` or `telebot`) rather than raw HTTP calls, unless there's a specific reason not to.
- Respect Telegram's rate limits (roughly 30 messages/sec globally, 1/sec per chat) — batch or throttle where needed.
- Long-polling vs webhook: default to long polling for local dev, webhook for production behind HTTPS.
- Validate and sanitize all user input from Telegram updates before using it in commands or DB queries.