// Package bot is the Telegram front end: it turns updates into backend calls
// and renders the results as messages and inline keyboards.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/config"
)

const sessionTTL = 30 * time.Minute

// App wires the Telegram client to the backend.
type App struct {
	cfg      config.Config
	api      *backend.Client
	tg       *tg.Bot
	sessions *sessions
}

// New creates the bot. It calls getMe, so it fails fast on a bad token.
func New(ctx context.Context, cfg config.Config, api *backend.Client) (*App, error) {
	a := &App{cfg: cfg, api: api, sessions: newSessions(sessionTTL)}

	opts := []tg.Option{
		tg.WithDefaultHandler(a.handleUpdate),
		tg.WithAllowedUpdates(tg.AllowedUpdates{"message", "callback_query"}),
		tg.WithErrorsHandler(func(err error) { slog.Error("telegram", "err", err) }),
	}
	if cfg.WebhookURL != "" {
		opts = append(opts, tg.WithWebhookSecretToken(cfg.WebhookSecret))
	}

	b, err := tg.New(cfg.TelegramToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	a.tg = b

	if _, err := b.SetMyCommands(ctx, &tg.SetMyCommandsParams{Commands: commands}); err != nil {
		slog.Warn("set bot commands", "err", err)
	}
	return a, nil
}

// Run receives updates until ctx is done: long polling by default, or a
// webhook server when WEBHOOK_URL is set.
func (a *App) Run(ctx context.Context) error {
	go a.sweepSessions(ctx)

	if a.cfg.WebhookURL == "" {
		// A webhook left over from a production run would block getUpdates.
		if _, err := a.tg.DeleteWebhook(ctx, &tg.DeleteWebhookParams{}); err != nil {
			return fmt.Errorf("delete webhook: %w", err)
		}
		slog.Info("bot started", "mode", "long polling")
		a.tg.Start(ctx)
		return nil
	}

	if _, err := a.tg.SetWebhook(ctx, &tg.SetWebhookParams{
		URL:            a.cfg.WebhookURL,
		SecretToken:    a.cfg.WebhookSecret,
		AllowedUpdates: []string{"message", "callback_query"},
	}); err != nil {
		return fmt.Errorf("set webhook: %w", err)
	}

	srv := &http.Server{
		Addr:              a.cfg.ListenAddr,
		Handler:           a.tg.WebhookHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go a.tg.StartWebhook(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("bot started", "mode", "webhook", "listen", a.cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("webhook server: %w", err)
	}
	return nil
}

func (a *App) sweepSessions(ctx context.Context) {
	t := time.NewTicker(sessionTTL)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.sessions.sweep()
		}
	}
}

func (a *App) handleUpdate(ctx context.Context, _ *tg.Bot, u *models.Update) {
	switch {
	case u.Message != nil:
		a.onMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		a.onCallback(ctx, u.CallbackQuery)
	}
}

func (a *App) onMessage(ctx context.Context, msg *models.Message) {
	// Finances are private: only one-to-one chats, and only from a real user.
	if msg.Chat.Type != models.ChatTypePrivate || msg.From == nil || msg.From.IsBot {
		return
	}
	chatID, userID := msg.Chat.ID, msg.From.ID
	text := strings.TrimSpace(msg.Text)

	if cmd, ok := command(text); ok {
		a.onCommand(ctx, msg, cmd)
		return
	}
	if action, ok := menuActions[text]; ok {
		a.sessions.clear(userID)
		a.runAction(ctx, chatID, userID, action)
		return
	}

	sess := a.sessions.get(userID)
	switch sess.step {
	case stepAmount:
		a.onAmount(ctx, chatID, userID, sess, text)
	case stepToAmount:
		a.onToAmount(ctx, chatID, userID, sess, text)
	case stepPurchaseName:
		a.onPurchaseNames(ctx, chatID, userID, sess, msg.Text)
	case stepAccount, stepToAccount, stepCategory:
		a.send(ctx, chatID, "Выберите вариант кнопкой выше или нажмите «Отмена».", nil)
	default:
		// A bare amount starts an expense: "5000 обед" is the most common entry.
		if _, _, err := parseAmountNote(text); err == nil {
			a.onAmount(ctx, chatID, userID, session{step: stepAmount, txType: backend.TypeExpense}, text)
			return
		}
		a.send(ctx, chatID, "Не понял 🤔 Выберите действие в меню или отправьте /help.", mainMenu)
	}
}

func (a *App) onCommand(ctx context.Context, msg *models.Message, cmd string) {
	chatID, userID := msg.Chat.ID, msg.From.ID
	a.sessions.clear(userID)

	switch cmd {
	case "start":
		a.onStart(ctx, msg)
	case "help":
		a.send(ctx, chatID, a.help(), mainMenu)
	case "cancel":
		a.send(ctx, chatID, "Отменено.", mainMenu)
	default:
		if action, ok := commandActions[cmd]; ok {
			a.runAction(ctx, chatID, userID, action)
			return
		}
		a.send(ctx, chatID, "Не знаю такой команды. /help — список команд.", mainMenu)
	}
}

func (a *App) runAction(ctx context.Context, chatID, userID int64, act action) {
	switch act {
	case actExpense:
		a.startTransaction(ctx, chatID, userID, backend.TypeExpense)
	case actIncome:
		a.startTransaction(ctx, chatID, userID, backend.TypeIncome)
	case actTransfer:
		a.startTransaction(ctx, chatID, userID, backend.TypeTransfer)
	case actBills:
		a.showBills(ctx, chatID, userID)
	case actShopping:
		a.showPurchases(ctx, chatID, 0, userID)
	case actSummary:
		a.showSummary(ctx, chatID, userID)
	}
}

func (a *App) onCallback(ctx context.Context, cq *models.CallbackQuery) {
	msg := cq.Message.Message
	if msg == nil || msg.Chat.Type != models.ChatTypePrivate {
		a.answer(ctx, cq.ID, "Сообщение устарело.", false)
		return
	}
	chatID, msgID, userID := msg.Chat.ID, msg.ID, cq.From.ID
	kind, arg, _ := strings.Cut(cq.Data, ":")

	switch kind {
	case cbCancel:
		a.sessions.clear(userID)
		a.answer(ctx, cq.ID, "", false)
		a.edit(ctx, chatID, msgID, "Отменено.", nil)
	case cbAccount, cbToAccount, cbCategory:
		a.onTransactionChoice(ctx, cq, chatID, msgID, userID, kind, arg)
	case cbBillPaid:
		a.onBillPaid(ctx, cq, msg, userID, arg)
	case cbPurchase:
		a.onPurchaseCallback(ctx, cq, chatID, msgID, userID, arg)
	default:
		a.answer(ctx, cq.ID, "Неизвестная кнопка.", false)
	}
}

// command extracts "start" from "/start" or "/start@family_bot payload".
func command(text string) (string, bool) {
	if !strings.HasPrefix(text, "/") {
		return "", false
	}
	word, _, _ := strings.Cut(text[1:], " ")
	word, _, _ = strings.Cut(word, "@")
	return strings.ToLower(word), word != ""
}

// send posts an HTML message. Callers escape any user or backend text they embed.
func (a *App) send(ctx context.Context, chatID int64, text string, markup models.ReplyMarkup) {
	_, err := a.tg.SendMessage(ctx, &tg.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.Error("send message", "chat", chatID, "err", err)
	}
}

// edit replaces a message's text and inline keyboard; a nil keyboard removes it.
func (a *App) edit(ctx context.Context, chatID int64, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	params := &tg.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if kb != nil {
		params.ReplyMarkup = kb
	}
	if _, err := a.tg.EditMessageText(ctx, params); err != nil && !isNotModified(err) {
		slog.Error("edit message", "chat", chatID, "err", err)
	}
}

// show edits msgID in place when there is one, otherwise sends a new message.
func (a *App) show(ctx context.Context, chatID int64, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	if msgID != 0 {
		a.edit(ctx, chatID, msgID, text, kb)
		return
	}
	var markup models.ReplyMarkup
	if kb != nil {
		markup = kb
	}
	a.send(ctx, chatID, text, markup)
}

func (a *App) answer(ctx context.Context, callbackID, text string, alert bool) {
	_, err := a.tg.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       alert,
	})
	if err != nil {
		slog.Warn("answer callback", "err", err)
	}
}

// isNotModified reports Telegram's harmless refusal to edit a message into
// identical content, e.g. after a double tap.
func isNotModified(err error) bool {
	return errors.Is(err, tg.ErrorBadRequest) && strings.Contains(err.Error(), "message is not modified")
}

// userMessage turns a backend error into something to tell the user, and logs
// whatever the user can't act on.
func userMessage(err error) string {
	var apiErr *backend.APIError
	switch {
	case backend.IsNotLinked(err):
		return "Я вас пока не знаю. Отправьте /start, чтобы привязать аккаунт."
	case !errors.As(err, &apiErr):
		slog.Error("backend unreachable", "err", err)
		return "Сервер не отвечает, попробуйте чуть позже."
	case apiErr.Status == http.StatusUnauthorized:
		slog.Error("backend rejected the bot token; check BOT_API_TOKEN", "err", err)
		return "Бот временно не может связаться с сервером. Сообщите администратору."
	case apiErr.Status == http.StatusForbidden:
		return "Нет доступа к этой записи."
	case apiErr.Status == http.StatusNotFound:
		return "Запись не найдена: возможно, её уже удалили."
	case apiErr.Status == http.StatusBadRequest:
		return "Не получилось: " + escape(apiErr.Message)
	default:
		slog.Error("backend error", "err", err)
		return "Ошибка на сервере, попробуйте чуть позже."
	}
}

func (a *App) fail(ctx context.Context, chatID int64, err error) {
	a.send(ctx, chatID, "⚠️ "+userMessage(err), nil)
}

// failCallback reports an error from a button press as a popup.
func (a *App) failCallback(ctx context.Context, callbackID string, err error) {
	a.answer(ctx, callbackID, plainText(userMessage(err)), true)
}

func (a *App) help() string {
	h, m := int(a.cfg.NotifyAt/time.Hour), int(a.cfg.NotifyAt%time.Hour/time.Minute)
	return fmt.Sprintf(helpText, fmt.Sprintf("%02d:%02d", h, m))
}

// today is the current date in the family's timezone, which is also what the
// backend uses for due dates and "future date" checks.
func (a *App) today() time.Time {
	return time.Now().In(a.cfg.Location)
}
