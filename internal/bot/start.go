package bot

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

// onStart links the Telegram account to the family member whose profile has
// this username. Telegram only reveals the chat id once the user writes first,
// so /start is also what makes the daily reminders reachable.
func (a *App) onStart(ctx context.Context, msg *models.Message) {
	chatID := msg.Chat.ID
	username := msg.From.Username
	if username == "" {
		a.send(ctx, chatID, "У вас в Telegram не задан username. Укажите его в настройках Telegram "+
			"(Настройки → Имя пользователя), попросите администратора добавить его в приложении и снова отправьте /start.", nil)
		return
	}

	who, err := a.api.Link(ctx, backend.LinkRequest{
		TelegramUsername: username,
		TelegramUserID:   msg.From.ID,
		ChatID:           chatID,
	})
	switch {
	case backend.HasStatus(err, http.StatusNotFound):
		a.send(ctx, chatID, fmt.Sprintf("Не нашёл пользователя с username <b>@%s</b>. "+
			"Попросите администратора указать его в вашем профиле в приложении и снова отправьте /start.", escape(username)), nil)
		return
	case backend.HasStatus(err, http.StatusConflict):
		a.send(ctx, chatID, "Этот Telegram-аккаунт уже привязан к другому пользователю. Обратитесь к администратору.", nil)
		return
	case err != nil:
		a.fail(ctx, chatID, err)
		return
	}

	a.send(ctx, chatID, fmt.Sprintf("Привет, <b>%s</b>! 👋 Аккаунт привязан.\n\n%s", escape(who.DisplayName), a.help()), mainMenu)
}
