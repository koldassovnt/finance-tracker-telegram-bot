FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# The image's tag, from the VERSION file; ./deploy.sh passes it via compose.
ARG VERSION
RUN test -n "$VERSION" || { echo "VERSION is not set: deploy with ./deploy.sh" >&2; exit 1; }
# Static binary; the timezone database is embedded via time/tzdata.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o family-finance-telegram-bot ./cmd/family-finance-telegram-bot

FROM alpine:3.21

ARG VERSION
LABEL org.opencontainers.image.version="${VERSION}"

# CA certificates for HTTPS to api.telegram.org.
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 bot

WORKDIR /app

COPY --from=builder /app/family-finance-telegram-bot .

USER bot

CMD ["./family-finance-telegram-bot"]
