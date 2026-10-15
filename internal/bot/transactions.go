package bot

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

// maxCategoryButtons keeps the keyboard well under Telegram's 100-button limit.
const maxCategoryButtons = 60

// The transaction dialog: amount [+ note] → account → category (income and
// expense) or destination account [→ received amount] (transfer).

func (a *App) startTransaction(ctx context.Context, chatID, userID int64, txType string) {
	a.sessions.set(userID, session{step: stepAmount, txType: txType})

	var prompt string
	switch txType {
	case backend.TypeIncome:
		prompt = "➕ <b>Доход</b>\nВведите сумму и, если нужно, комментарий:\n<code>350000 зарплата</code>"
	case backend.TypeTransfer:
		prompt = "🔁 <b>Перевод</b>\nВведите сумму списания и, если нужно, комментарий:\n<code>100000 на депозит</code>"
	default:
		prompt = "➖ <b>Расход</b>\nВведите сумму и, если нужно, комментарий:\n<code>5000 обед</code>"
	}
	a.send(ctx, chatID, prompt, cancelKeyboard())
}

func (a *App) onAmount(ctx context.Context, chatID, userID int64, sess session, text string) {
	amount, note, err := parseAmountNote(text)
	if err != nil {
		a.sessions.set(userID, sess)
		a.send(ctx, chatID, "Не понял сумму. Напишите число, например <code>5000</code> или <code>5000 обед</code>.", cancelKeyboard())
		return
	}

	all, err := a.api.Accounts(ctx, userID)
	if err != nil {
		a.sessions.clear(userID)
		a.fail(ctx, chatID, err)
		return
	}
	accounts := slices.DeleteFunc(all, func(acc backend.Account) bool { return acc.Access != backend.AccessOwner })

	isTransfer := sess.txType == backend.TypeTransfer
	switch {
	case len(accounts) == 0:
		a.sessions.clear(userID)
		a.send(ctx, chatID, "У вас пока нет счетов. Добавьте счёт в приложении и попробуйте снова.", nil)
		return
	case isTransfer && len(accounts) < 2:
		a.sessions.clear(userID)
		a.send(ctx, chatID, "Для перевода нужно хотя бы два счёта.", nil)
		return
	}

	sess.amount, sess.note, sess.accounts = amount, note, accounts
	if !isTransfer && len(accounts) == 1 {
		sess.account = &accounts[0]
		a.askCategory(ctx, chatID, 0, userID, sess)
		return
	}

	sess.step = stepAccount
	a.sessions.set(userID, sess)
	question := "С какого счёта?"
	if sess.txType == backend.TypeIncome {
		question = "На какой счёт?"
	}
	a.send(ctx, chatID, draftText(sess)+"\n\n"+question, accountsKeyboard(accounts, cbAccount, ""))
}

func (a *App) onTransactionChoice(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64, kind, arg string) {
	sess := a.sessions.get(userID)
	want := map[string]step{cbAccount: stepAccount, cbToAccount: stepToAccount, cbCategory: stepCategory}[kind]
	if sess.step != want || (arg != noCategory && !isUUID(arg)) {
		a.answer(ctx, cq.ID, "Эта кнопка уже неактуальна. Начните заново.", true)
		return
	}
	a.answer(ctx, cq.ID, "", false)

	switch kind {
	case cbAccount:
		acc := findAccount(sess.accounts, arg)
		if acc == nil {
			a.edit(ctx, chatID, msgID, "Счёт не найден. Начните заново.", nil)
			a.sessions.clear(userID)
			return
		}
		sess.account = acc
		if sess.txType != backend.TypeTransfer {
			a.askCategory(ctx, chatID, msgID, userID, sess)
			return
		}
		sess.step = stepToAccount
		a.sessions.set(userID, sess)
		a.edit(ctx, chatID, msgID, draftText(sess)+"\n\nНа какой счёт?", accountsKeyboard(sess.accounts, cbToAccount, acc.ID))

	case cbToAccount:
		acc := findAccount(sess.accounts, arg)
		if acc == nil || acc.ID == sess.account.ID {
			a.edit(ctx, chatID, msgID, "Счёт не найден. Начните заново.", nil)
			a.sessions.clear(userID)
			return
		}
		sess.toAccount = acc
		if acc.Currency == sess.account.Currency {
			a.createTransaction(ctx, chatID, msgID, userID, sess)
			return
		}
		sess.step = stepToAmount
		a.sessions.set(userID, sess)
		a.edit(ctx, chatID, msgID, draftText(sess)+"\n\nСчета в разных валютах. Сколько поступит на «"+
			escape(acc.Name)+"» в "+escape(acc.Currency)+"?", cancelKeyboard())

	case cbCategory:
		if arg != noCategory {
			i := slices.IndexFunc(sess.categories, func(c backend.Category) bool { return c.ID == arg })
			if i < 0 {
				a.edit(ctx, chatID, msgID, "Категория не найдена. Начните заново.", nil)
				a.sessions.clear(userID)
				return
			}
			sess.category = &sess.categories[i]
		}
		a.createTransaction(ctx, chatID, msgID, userID, sess)
	}
}

func (a *App) askCategory(ctx context.Context, chatID int64, msgID int, userID int64, sess session) {
	all, err := a.api.Categories(ctx, userID)
	if err != nil {
		a.sessions.clear(userID)
		a.fail(ctx, chatID, err)
		return
	}
	kind := backend.KindExpense
	if sess.txType == backend.TypeIncome {
		kind = backend.KindIncome
	}
	cats := slices.DeleteFunc(all, func(c backend.Category) bool { return c.Kind != kind })
	if len(cats) == 0 {
		a.createTransaction(ctx, chatID, msgID, userID, sess)
		return
	}
	slices.SortFunc(cats, func(x, y backend.Category) int {
		return cmp.Compare(categoryLabel(x, cats), categoryLabel(y, cats))
	})

	sess.categories = cats
	sess.step = stepCategory
	a.sessions.set(userID, sess)

	buttons := make([]models.InlineKeyboardButton, 0, min(len(cats), maxCategoryButtons))
	for _, c := range cats[:min(len(cats), maxCategoryButtons)] {
		buttons = append(buttons, models.InlineKeyboardButton{Text: categoryLabel(c, cats), CallbackData: cbCategory + ":" + c.ID})
	}
	rows := grid(buttons, 2)
	rows = append(rows,
		[]models.InlineKeyboardButton{{Text: "Без категории", CallbackData: cbCategory + ":" + noCategory}},
		cancelRow(),
	)
	a.show(ctx, chatID, msgID, draftText(sess)+"\n\nКатегория?", &models.InlineKeyboardMarkup{InlineKeyboard: rows})
}

func (a *App) onToAmount(ctx context.Context, chatID, userID int64, sess session, text string) {
	amount, err := parseAmount(text)
	if err != nil {
		a.sessions.set(userID, sess)
		a.send(ctx, chatID, "Не понял сумму. Напишите только число, например <code>47000</code>.", cancelKeyboard())
		return
	}
	sess.toAmount = amount
	a.createTransaction(ctx, chatID, 0, userID, sess)
}

func (a *App) createTransaction(ctx context.Context, chatID int64, msgID int, userID int64, sess session) {
	// Clear first so a double tap can't record the same transaction twice.
	a.sessions.clear(userID)

	req := backend.CreateTransactionRequest{
		Type:       sess.txType,
		AccountID:  sess.account.ID,
		Amount:     sess.amount,
		OccurredOn: a.today().Format(time.DateOnly),
		Note:       sess.note,
	}
	if sess.category != nil {
		req.CategoryID = sess.category.ID
	}
	if sess.toAccount != nil {
		req.ToAccountID = sess.toAccount.ID
		req.ToAmount = sess.toAmount
	}

	tx, err := a.api.CreateTransaction(ctx, userID, req)
	if err != nil {
		if msgID != 0 {
			a.edit(ctx, chatID, msgID, draftText(sess)+"\n\n❌ Не записано.", nil)
		}
		a.fail(ctx, chatID, err)
		return
	}

	var b strings.Builder
	switch sess.txType {
	case backend.TypeIncome:
		b.WriteString("✅ Доход записан\n")
	case backend.TypeTransfer:
		b.WriteString("✅ Перевод записан\n")
	default:
		b.WriteString("✅ Расход записан\n")
	}
	b.WriteString("<b>" + formatMoney(tx.Amount, tx.Currency) + "</b> · " + escape(sess.account.Name))
	if sess.toAccount != nil {
		b.WriteString(" → " + escape(sess.toAccount.Name))
		if sess.toAmount != "" {
			b.WriteString(" (<b>" + formatMoney(sess.toAmount, sess.toAccount.Currency) + "</b>)")
		}
	}
	if sess.category != nil {
		b.WriteString("\nКатегория: " + escape(categoryLabel(*sess.category, sess.categories)))
	}
	if sess.note != "" {
		b.WriteString("\n💬 " + escape(sess.note))
	}
	a.show(ctx, chatID, msgID, b.String(), nil)
}

// draftText summarises the transaction entered so far.
func draftText(sess session) string {
	var b strings.Builder
	switch sess.txType {
	case backend.TypeIncome:
		b.WriteString("➕ Доход ")
	case backend.TypeTransfer:
		b.WriteString("🔁 Перевод ")
	default:
		b.WriteString("➖ Расход ")
	}
	if sess.account != nil {
		b.WriteString("<b>" + formatMoney(sess.amount, sess.account.Currency) + "</b> · " + escape(sess.account.Name))
	} else {
		b.WriteString("<b>" + formatAmount(sess.amount) + "</b>")
	}
	if sess.toAccount != nil {
		b.WriteString(" → " + escape(sess.toAccount.Name))
	}
	if sess.note != "" {
		b.WriteString("\n💬 " + escape(sess.note))
	}
	return b.String()
}

func accountsKeyboard(accounts []backend.Account, kind, excludeID string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	for _, acc := range accounts {
		if acc.ID == excludeID {
			continue
		}
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         acc.Name + " · " + formatMoney(acc.Balance, acc.Currency),
			CallbackData: kind + ":" + acc.ID,
		}})
	}
	rows = append(rows, cancelRow())
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func findAccount(accounts []backend.Account, id string) *backend.Account {
	for i := range accounts {
		if accounts[i].ID == id {
			return &accounts[i]
		}
	}
	return nil
}

// categoryLabel prefixes a sub-category with its parent: "Еда › Кафе".
func categoryLabel(c backend.Category, all []backend.Category) string {
	if c.ParentID == nil {
		return c.Name
	}
	for _, p := range all {
		if p.ID == *c.ParentID {
			return p.Name + " › " + c.Name
		}
	}
	return c.Name
}
