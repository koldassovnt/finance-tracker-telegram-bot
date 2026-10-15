// Package config loads the bot's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	// Embed the timezone database so Asia/Almaty resolves on hosts without one (e.g. Windows).
	_ "time/tzdata"
)

// Config holds everything the bot needs at startup.
type Config struct {
	// TelegramToken is the bot token from @BotFather.
	TelegramToken string
	// BackendURL is the family-finance-crm base URL, without the /api/v1 suffix.
	BackendURL string
	// BackendToken is the shared secret sent as X-Bot-Token.
	BackendToken string
	// Location is the timezone "today" and the daily reminder are computed in.
	Location *time.Location
	// NotifyAt is the time of day of the bills reminder, as an offset from midnight.
	NotifyAt time.Duration
	// WebhookURL switches the bot to webhook mode when set; empty means long polling.
	WebhookURL string
	// WebhookSecret is checked against Telegram's X-Telegram-Bot-Api-Secret-Token header.
	WebhookSecret string
	// ListenAddr is where the webhook HTTP server listens.
	ListenAddr string
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		BackendURL:    strings.TrimRight(envOr("BACKEND_URL", "http://localhost:8080"), "/"),
		BackendToken:  os.Getenv("BOT_API_TOKEN"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		ListenAddr:    envOr("LISTEN_ADDR", ":8081"),
	}

	var errs []error
	if cfg.TelegramToken == "" {
		errs = append(errs, errors.New("TELEGRAM_BOT_TOKEN is required"))
	}
	if cfg.BackendToken == "" {
		errs = append(errs, errors.New("BOT_API_TOKEN is required"))
	}
	if _, err := url.ParseRequestURI(cfg.BackendURL); err != nil {
		errs = append(errs, fmt.Errorf("BACKEND_URL: %w", err))
	}

	loc, err := time.LoadLocation(envOr("APP_TIMEZONE", "Asia/Almaty"))
	if err != nil {
		errs = append(errs, fmt.Errorf("APP_TIMEZONE: %w", err))
	}
	cfg.Location = loc

	notifyAt, err := ParseClock(envOr("NOTIFY_AT", "10:00"))
	if err != nil {
		errs = append(errs, fmt.Errorf("NOTIFY_AT: %w", err))
	}
	cfg.NotifyAt = notifyAt

	if cfg.WebhookURL != "" {
		if !strings.HasPrefix(cfg.WebhookURL, "https://") {
			errs = append(errs, errors.New("WEBHOOK_URL must be https"))
		}
		if cfg.WebhookSecret == "" {
			errs = append(errs, errors.New("WEBHOOK_SECRET is required in webhook mode"))
		}
	}

	return cfg, errors.Join(errs...)
}

// ParseClock parses "HH:MM" into an offset from midnight.
func ParseClock(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("want HH:MM, got %q", s)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
