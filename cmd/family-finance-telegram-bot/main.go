// Command family-finance-telegram-bot is the Telegram front end for family-finance-crm.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/bot"
	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/config"
	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/scheduler"
)

// reminderTimeout bounds one run of the daily bills reminder.
const reminderTimeout = 5 * time.Minute

// version is the VERSION file's content, set by the Docker build; "dev" for go run.
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("bot stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting family-finance-telegram-bot", "version", version)
	// .env is for local development; in production the variables come from the environment.
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bot.New(ctx, cfg, backend.New(cfg.BackendURL, cfg.BackendToken))
	if err != nil {
		return err
	}

	go scheduler.Daily(ctx, cfg.Location, cfg.NotifyAt, reminderTimeout, app.SendDueBills)
	return app.Run(ctx)
}
