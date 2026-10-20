package bot

import (
	"html"

	"github.com/go-telegram/bot/models"
)

type action int

const (
	actExpense action = iota + 1
	actIncome
	actTransfer
	actBills
	actShopping
	actSummary
	actBalance
	actTopicExpense
)

// Main menu buttons; their text is what Telegram sends back when tapped.
const (
	btnExpense  = "➖ Расход"
	btnIncome   = "➕ Доход"
	btnTransfer = "🔁 Перевод"
	btnBills    = "📅 Платежи"
	btnShopping = "🛒 Покупки"
	btnSummary  = "📊 Итоги месяца"
	btnBalance  = "💰 Баланс"
	btnTopic    = "🧳 Расход по теме"
)

var menuActions = map[string]action{
	btnExpense:  actExpense,
	btnIncome:   actIncome,
	btnTransfer: actTransfer,
	btnBills:    actBills,
	btnShopping: actShopping,
	btnSummary:  actSummary,
	btnBalance:  actBalance,
	btnTopic:    actTopicExpense,
}

var commandActions = map[string]action{
	"expense":  actExpense,
	"income":   actIncome,
	"transfer": actTransfer,
	"bills":    actBills,
	"shop":     actShopping,
	"summary":  actSummary,
	"balance":  actBalance,
	"topic":    actTopicExpense,
}

var commands = []models.BotCommand{
	{Command: "expense", Description: "Записать расход"},
	{Command: "income", Description: "Записать доход"},
	{Command: "topic", Description: "Расход по теме, например поездке"},
	{Command: "transfer", Description: "Перевод между своими счетами"},
	{Command: "bills", Description: "Неоплаченные платежи"},
	{Command: "shop", Description: "Список покупок"},
	{Command: "summary", Description: "Итоги месяца"},
	{Command: "balance", Description: "Баланс счёта"},
	{Command: "cancel", Description: "Отменить текущее действие"},
	{Command: "start", Description: "Привязать аккаунт"},
	{Command: "help", Description: "Что умеет бот"},
}

var mainMenu = &models.ReplyKeyboardMarkup{
	Keyboard: [][]models.KeyboardButton{
		{{Text: btnExpense}, {Text: btnIncome}, {Text: btnTransfer}},
		{{Text: btnTopic}},
		{{Text: btnBills}, {Text: btnShopping}},
		{{Text: btnSummary}, {Text: btnBalance}},
	},
	ResizeKeyboard: true,
	IsPersistent:   true,
}

// Callback data prefixes, "<kind>:<arg>". Telegram caps callback data at 64
// bytes; the longest here is "bal:" or "p:t:" plus a 36-character UUID. cbRate
// carries no argument: the rate it accepts is the one kept in the session.
const (
	cbCancel    = "x"
	cbAccount   = "a"
	cbToAccount = "to"
	cbCategory  = "c"
	cbBillPaid  = "b"
	cbPurchase  = "p"
	cbBalance   = "bal"
	cbTopic     = "tp"
	cbRate      = "r"
)

// noCategory is the cbCategory argument for "no category".
const noCategory = "-"

// helpText takes the daily reminder time, "10:00".
const helpText = `<b>Что умеет бот</b>

➖ <b>Расход</b> / ➕ <b>Доход</b>: сумма, счёт и категория. Можно просто написать «5000 обед», и это запишется как расход.
🧳 <b>Расход по теме</b>: выберите тему из приложения, например поездку, и расход попадёт в её итоги.
🔁 <b>Перевод</b>: между своими счетами, в том числе в разных валютах.
📅 <b>Платежи</b>: неоплаченные платежи до конца месяца, отметка «оплачено».
🛒 <b>Покупки</b>: семейный и личный списки.
📊 <b>Итоги месяца</b>: доходы, расходы и крупнейшие категории.
💰 <b>Баланс</b>: текущий остаток на выбранном счёте.

Для счёта не в тенге я спрошу курс: сколько тенге стоит 1 единица валюты.
Каждый день в %s я пишу, какие платежи нужно оплатить сегодня.
/cancel отменяет текущее действие.`

func cancelRow() []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{{Text: "✖ Отмена", CallbackData: cbCancel}}
}

func cancelKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{cancelRow()}}
}

// grid lays buttons out perRow to a row.
func grid(buttons []models.InlineKeyboardButton, perRow int) [][]models.InlineKeyboardButton {
	var rows [][]models.InlineKeyboardButton
	for len(buttons) > 0 {
		n := min(perRow, len(buttons))
		rows = append(rows, buttons[:n])
		buttons = buttons[n:]
	}
	return rows
}

func escape(s string) string { return html.EscapeString(s) }

// plainText turns an escaped, tag-free HTML message back into plain text, for
// callback popups, which don't support markup.
func plainText(s string) string { return html.UnescapeString(s) }
