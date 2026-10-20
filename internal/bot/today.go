package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

const (
	// maxTodayListBytes keeps the day's message under Telegram's 4096-character
	// limit; bytes overcount Cyrillic, which leaves room for the totals.
	maxTodayListBytes = 3000
	// maxTodayNote trims long comments in the list, in runes.
	maxTodayNote = 60
)

// showToday lists today's operations with the day's income and expense in KZT.
func (a *App) showToday(ctx context.Context, chatID int64, msgID int, userID int64) {
	today := a.today()
	txs, err := a.api.Transactions(ctx, userID, today, today)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}
	accounts, err := a.api.Accounts(ctx, userID)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}
	byID := make(map[string]backend.Account, len(accounts))
	for _, acc := range accounts {
		byID[acc.ID] = acc
	}

	kb := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "🔄 Обновить", CallbackData: cbToday}},
	}}
	a.show(ctx, chatID, msgID, renderToday(today, txs, byID), kb)
}

// renderToday renders the day's operations in the order given, then the totals.
// Totals count INCOME and EXPENSE only, in KZT: a transfer moves money between
// your own accounts and is neither.
func renderToday(now time.Time, txs []backend.Transaction, accounts map[string]backend.Account) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📆 <b>Сегодня, %s</b>\n\n", formatLongDay(now))

	if len(txs) == 0 {
		b.WriteString("Операций пока нет.\n")
	}
	hasTransfer := false
	listed := 0
	for _, tx := range txs {
		hasTransfer = hasTransfer || tx.Type == backend.TypeTransfer
		line := operationLine(tx, accounts)
		if b.Len()+len(line) > maxTodayListBytes {
			break
		}
		b.WriteString(line + "\n")
		listed++
	}
	if rest := len(txs) - listed; rest > 0 {
		fmt.Fprintf(&b, "<i>…и ещё %d, полный список в приложении</i>\n", rest)
	}

	income := sumKZT(txs, backend.TypeIncome)
	expense := sumKZT(txs, backend.TypeExpense)
	net := new(big.Rat).Sub(income, expense)
	netText := formatMoney(ratNumber(net), backend.BaseCurrency)
	if net.Sign() > 0 {
		netText = "+" + netText
	}

	b.WriteString("\n<b>Итого за день</b>\n")
	b.WriteString("Доходы: <b>" + formatMoney(ratNumber(income), backend.BaseCurrency) + "</b>\n")
	b.WriteString("Расходы: <b>" + formatMoney(ratNumber(expense), backend.BaseCurrency) + "</b>\n")
	b.WriteString("Доходы − расходы: <b>" + netText + "</b>\n")
	if hasTransfer {
		b.WriteString("<i>Переводы между счетами в итоги не входят.</i>\n")
	}
	b.WriteString("<i>на " + now.Format("15:04") + "</i>")
	return b.String()
}

// operationLine renders one transaction: "➖ 5 000 ₸ · Kaspi · Еда · 💬 обед".
func operationLine(tx backend.Transaction, accounts map[string]backend.Account) string {
	var b strings.Builder
	switch tx.Type {
	case backend.TypeIncome:
		b.WriteString("➕ ")
	case backend.TypeExpense:
		b.WriteString("➖ ")
	case backend.TypeTransfer:
		b.WriteString("🔁 ")
	default:
		b.WriteString("• ")
	}
	b.WriteString("<b>" + formatMoney(tx.Amount, tx.Currency) + "</b>")
	if tx.Currency != backend.BaseCurrency && tx.AmountKZT != "" && tx.Type != backend.TypeTransfer {
		b.WriteString(" (≈ " + formatMoney(tx.AmountKZT, backend.BaseCurrency) + ")")
	}

	if acc, ok := accounts[tx.AccountID]; ok {
		b.WriteString(" · " + escape(acc.Name))
	}
	if tx.ToAccountID != "" {
		to, ok := accounts[tx.ToAccountID]
		if ok {
			b.WriteString(" → " + escape(to.Name))
		}
		if ok && tx.ToAmount != "" && to.Currency != tx.Currency {
			b.WriteString(" (" + formatMoney(tx.ToAmount, to.Currency) + ")")
		}
	}
	if tx.Category != nil {
		b.WriteString(" · " + escape(tx.Category.Name))
	}
	if tx.Topic != nil {
		b.WriteString(" · 🧳 " + escape(tx.Topic.Name))
	}
	if note := cleanText(tx.Note, maxNoteLen); note != "" {
		if utf8.RuneCountInString(note) > maxTodayNote {
			note = string([]rune(note)[:maxTodayNote-1]) + "…"
		}
		b.WriteString(" · 💬 " + escape(note))
	}
	return b.String()
}

// sumKZT adds up the KZT amounts of transactions of type txType, exactly.
func sumKZT(txs []backend.Transaction, txType string) *big.Rat {
	sum := new(big.Rat)
	for _, tx := range txs {
		if tx.Type != txType {
			continue
		}
		if r, ok := new(big.Rat).SetString(tx.AmountKZT.String()); ok {
			sum.Add(sum, r)
		}
	}
	return sum
}

// ratNumber renders r with the backend's four money decimals, for formatAmount.
func ratNumber(r *big.Rat) json.Number {
	return json.Number(r.FloatString(4))
}
