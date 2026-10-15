FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static binary; the timezone database is embedded via time/tzdata.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o family-finance-telegram-bot ./cmd/family-finance-telegram-bot

FROM alpine:3.21

# CA certificates for HTTPS to api.telegram.org.
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 bot

WORKDIR /app

COPY --from=builder /app/family-finance-telegram-bot .

USER bot

CMD ["./family-finance-telegram-bot"]
