package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

const (
	// maxBillButtons caps the bills list keyboard.
	maxBillButtons = 30
	// reminderInterval paces the daily broadcast under Telegram's ~30 msg/s global limit.
	reminderInterval = time.Second / 20
)

// Bill messages are built without tags so that, when a bill is paid, the
// message can be rebuilt from its own plain text (Telegram returns it without
// markup) plus one more line.

// showBills lists unpaid bills due by the end of this month, overdue ones included.
func (a *App) showBills(ctx context.Context, chatID, userID int64) {
	bills, err := a.api.UnpaidBills(ctx, userID)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}

	today := a.today()
	todayStr := today.Format(time.DateOnly)
	monthEnd := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()).Format(time.DateOnly)

	var b strings.Builder
	var buttons [][]models.InlineKeyboardButton
	shown, later := 0, 0
	for _, bill := range bills {
		// ISO dates compare correctly as strings.
		if bill.DueDate > monthEnd {
			later++
			continue
		}
		shown++
		mark, suffix := "⚪", ""
		switch {
		case bill.DueDate < todayStr:
			mark, suffix = "🔴", " (просрочен)"
		case bill.DueDate == todayStr:
			mark, suffix = "🟡", " (сегодня)"
		}
		fmt.Fprintf(&b, "\n%s %s %s: %s%s", mark, formatDay(bill.DueDate), escape(bill.Name),
			formatMoney(bill.Amount, bill.Currency), suffix)
		if len(buttons) < maxBillButtons {
			buttons = append(buttons, []models.InlineKeyboardButton{{
				Text:         "✅ " + bill.Name + " · " + formatDay(bill.DueDate),
				CallbackData: cbBillPaid + ":" + bill.ID,
			}})
		}
	}

	if shown == 0 {
		text := "📅 До конца месяца неоплаченных платежей нет 🎉"
		if later > 0 {
			text += fmt.Sprintf("\nВ следующих месяцах: %d.", later)
		}
		a.send(ctx, chatID, text, nil)
		return
	}

	text := "📅 Неоплаченные платежи до конца месяца\n" + b.String()
	if later > 0 {
		text += fmt.Sprintf("\n\nЕщё %d в следующих месяцах, они есть в приложении.", later)
	}
	text += "\n\nНажмите на платёж, когда оплатите."
	a.send(ctx, chatID, text, &models.InlineKeyboardMarkup{InlineKeyboard: buttons})
}

// onBillPaid marks a bill paid and drops its button from the message, which
// may be the bills list or a daily reminder.
func (a *App) onBillPaid(ctx context.Context, cq *models.CallbackQuery, msg *models.Message, userID int64, billID string) {
	if !isUUID(billID) {
		a.answer(ctx, cq.ID, "Эта кнопка уже неактуальна.", true)
		return
	}
	bill, err := a.api.MarkBillPaid(ctx, userID, billID)
	if err != nil {
		a.failCallback(ctx, cq.ID, err)
		return
	}
	a.answer(ctx, cq.ID, "✅ «"+bill.Name+"» оплачен", false)

	var kb *models.InlineKeyboardMarkup
	if msg.ReplyMarkup != nil {
		kb = withoutButton(msg.ReplyMarkup, cq.Data)
	}
	a.edit(ctx, msg.Chat.ID, msg.ID, escape(msg.Text)+"\n✅ Оплачено: "+escape(bill.Name), kb)
}

// withoutButton returns the keyboard minus buttons with the given callback
// data, or nil if nothing is left.
func withoutButton(kb *models.InlineKeyboardMarkup, data string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	for _, row := range kb.InlineKeyboard {
		var kept []models.InlineKeyboardButton
		for _, btn := range row {
			if btn.CallbackData != data {
				kept = append(kept, btn)
			}
		}
		if len(kept) > 0 {
			rows = append(rows, kept)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SendDueBills sends each linked member a reminder about their bills due on
// the day of now. It is the scheduler's daily job.
func (a *App) SendDueBills(ctx context.Context, now time.Time) error {
	due, err := a.api.BillsDue(ctx, now)
	if err != nil {
		return fmt.Errorf("fetch bills due: %w", err)
	}

	pace := time.NewTicker(reminderInterval)
	defer pace.Stop()

	var sent, failed int
	for _, d := range due {
		if d.ChatID == 0 || len(d.Bills) == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("reminders interrupted after %d sent: %w", sent, ctx.Err())
		case <-pace.C:
		}

		text, kb := reminder(d, now)
		if err := a.sendReminder(ctx, d.ChatID, text, kb); err != nil {
			failed++
			slog.Error("send bill reminder", "user", d.UserID, "err", err)
			continue
		}
		sent++
	}

	slog.Info("bill reminders sent", "date", now.Format(time.DateOnly), "sent", sent, "failed", failed)
	if failed > 0 {
		return fmt.Errorf("%d of %d bill reminders failed", failed, sent+failed)
	}
	return nil
}

func reminder(d backend.DueBills, now time.Time) (string, *models.InlineKeyboardMarkup) {
	var b strings.Builder
	fmt.Fprintf(&b, "🔔 Платежи на сегодня, %s\n", formatLongDay(now))
	var rows [][]models.InlineKeyboardButton
	for _, bill := range d.Bills {
		fmt.Fprintf(&b, "\n• %s: %s", escape(bill.Name), formatMoney(bill.Amount, bill.Currency))
		if len(rows) < maxBillButtons {
			rows = append(rows, []models.InlineKeyboardButton{{
				Text:         "✅ " + bill.Name,
				CallbackData: cbBillPaid + ":" + bill.ID,
			}})
		}
	}
	b.WriteString("\n\nНажмите на платёж, когда оплатите.")
	return b.String(), &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// sendReminder sends one message, waiting out a single 429 if Telegram asks.
func (a *App) sendReminder(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) error {
	params := &tg.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML, ReplyMarkup: kb}
	_, err := a.tg.SendMessage(ctx, params)

	var tooMany *tg.TooManyRequestsError
	if errors.As(err, &tooMany) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(tooMany.RetryAfter) * time.Second):
		}
		_, err = a.tg.SendMessage(ctx, params)
	}
	if err != nil {
		return fmt.Errorf("send to chat %d: %w", chatID, err)
	}
	return nil
}
