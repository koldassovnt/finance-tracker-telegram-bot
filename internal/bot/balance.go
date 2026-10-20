package bot

import (
	"context"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

// Arguments of cbBalance callbacks: "bal:<id>" shows or refreshes one account,
// "bal:all" goes back to the account list.
const balAll = "all"

// showBalancePicker lists the member's accounts as buttons; with a single
// account it shows that account's balance straight away.
func (a *App) showBalancePicker(ctx context.Context, chatID int64, msgID int, userID int64) {
	accounts, err := a.api.Accounts(ctx, userID)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}
	switch len(accounts) {
	case 0:
		a.show(ctx, chatID, msgID, "У вас пока нет счетов. Добавьте счёт в приложении.", nil)
	case 1:
		text, kb := renderBalance(accounts[0], false, a.today())
		a.show(ctx, chatID, msgID, text, kb)
	default:
		text, kb := renderBalancePicker(accounts)
		a.show(ctx, chatID, msgID, text, kb)
	}
}

func (a *App) onBalanceCallback(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64, arg string) {
	if arg == balAll {
		a.answer(ctx, cq.ID, "", false)
		a.showBalancePicker(ctx, chatID, msgID, userID)
		return
	}
	if !isUUID(arg) {
		a.answer(ctx, cq.ID, "Эта кнопка уже неактуальна.", true)
		return
	}
	// Re-read rather than trusting the button: the balance moves with every
	// transaction, and the account may have been deleted since.
	accounts, err := a.api.Accounts(ctx, userID)
	if err != nil {
		a.failCallback(ctx, cq.ID, err)
		return
	}
	acc := findAccount(accounts, arg)
	if acc == nil {
		a.answer(ctx, cq.ID, "Этого счёта уже нет.", false)
		a.showBalancePicker(ctx, chatID, msgID, userID)
		return
	}
	a.answer(ctx, cq.ID, "", false)
	text, kb := renderBalance(*acc, len(accounts) > 1, a.today())
	a.edit(ctx, chatID, msgID, text, kb)
}

func renderBalancePicker(accounts []backend.Account) (string, *models.InlineKeyboardMarkup) {
	buttons := make([]models.InlineKeyboardButton, 0, len(accounts))
	for _, acc := range accounts {
		buttons = append(buttons, models.InlineKeyboardButton{
			Text:         acc.Name + " · " + acc.Currency,
			CallbackData: cbBalance + ":" + acc.ID,
		})
	}
	return "💰 <b>Баланс</b>\nВыберите счёт:", &models.InlineKeyboardMarkup{InlineKeyboard: grid(buttons, 2)}
}

// renderBalance shows one account's balance as of now. The timestamp also
// makes a refresh visibly change the message even when the balance hasn't.
func renderBalance(acc backend.Account, withBack bool, now time.Time) (string, *models.InlineKeyboardMarkup) {
	text := "💰 <b>" + escape(acc.Name) + "</b>\n" +
		"Баланс: <b>" + formatMoney(acc.Balance, acc.Currency) + "</b>\n" +
		"<i>на " + formatLongDay(now) + ", " + now.Format("15:04") + "</i>"

	row := []models.InlineKeyboardButton{{Text: "🔄 Обновить", CallbackData: cbBalance + ":" + acc.ID}}
	if withBack {
		row = append(row, models.InlineKeyboardButton{Text: "⬅ Все счета", CallbackData: cbBalance + ":" + balAll})
	}
	return text, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
}
