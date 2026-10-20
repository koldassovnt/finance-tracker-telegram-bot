package bot

import (
	"context"
	"slices"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

// maxTopicButtons keeps the keyboard well under Telegram's 100-button limit.
const maxTopicButtons = 60

// The topic expense dialog: topic → the regular expense dialog, with the
// topic attached to the transaction it records.

func (a *App) startTopicExpense(ctx context.Context, chatID, userID int64) {
	topics, err := a.api.ActiveTopics(ctx, userID)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}
	switch len(topics) {
	case 0:
		a.send(ctx, chatID, "У вас нет активных тем. Создайте тему в приложении, например поездку, и попробуйте снова.", nil)
	case 1:
		a.askTopicAmount(ctx, chatID, 0, userID, topics[0])
	default:
		a.sessions.set(userID, session{step: stepTopic, txType: backend.TypeExpense, topics: topics})
		a.send(ctx, chatID, "🧳 <b>Расход по теме</b>\nВыберите тему:", topicsKeyboard(topics))
	}
}

func (a *App) onTopicChoice(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64, arg string) {
	sess := a.sessions.get(userID)
	if sess.step != stepTopic || !isUUID(arg) {
		a.answer(ctx, cq.ID, "Эта кнопка уже неактуальна. Начните заново.", true)
		return
	}
	i := slices.IndexFunc(sess.topics, func(t backend.Topic) bool { return t.ID == arg })
	if i < 0 {
		a.answer(ctx, cq.ID, "", false)
		a.edit(ctx, chatID, msgID, "Тема не найдена. Начните заново.", nil)
		a.sessions.clear(userID)
		return
	}
	a.answer(ctx, cq.ID, "", false)
	a.askTopicAmount(ctx, chatID, msgID, userID, sess.topics[i])
}

// askTopicAmount starts the expense dialog with topic attached.
func (a *App) askTopicAmount(ctx context.Context, chatID int64, msgID int, userID int64, topic backend.Topic) {
	a.sessions.set(userID, session{step: stepAmount, txType: backend.TypeExpense, topic: &topic})
	a.show(ctx, chatID, msgID, topicHeader(topic)+
		"\n\nВведите сумму расхода и, если нужно, комментарий:\n<code>5000 обед</code>", cancelKeyboard())
}

func topicsKeyboard(topics []backend.Topic) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, min(len(topics), maxTopicButtons)+1)
	for _, t := range topics[:min(len(topics), maxTopicButtons)] {
		rows = append(rows, []models.InlineKeyboardButton{{Text: "🧳 " + t.Name, CallbackData: cbTopic + ":" + t.ID}})
	}
	rows = append(rows, cancelRow())
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// topicHeader names the topic with what has been spent on it so far, in KZT.
func topicHeader(t backend.Topic) string {
	var b strings.Builder
	b.WriteString("🧳 <b>" + escape(t.Name) + "</b>\n")
	b.WriteString("Потрачено: " + formatMoney(t.Spent, backend.BaseCurrency))
	if t.PlannedAmount != "" {
		b.WriteString(" из " + formatMoney(t.PlannedAmount, backend.BaseCurrency))
	}
	if t.Remaining != "" {
		b.WriteString("\nОсталось: " + formatMoney(t.Remaining, backend.BaseCurrency))
	}
	return b.String()
}
