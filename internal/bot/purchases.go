package bot

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

const (
	// maxPurchaseButtons leaves room under Telegram's 100-button limit for the action rows.
	maxPurchaseButtons = 80
	// maxItemsPerMessage bounds how many items one message can add.
	maxItemsPerMessage = 20
)

// Arguments of cbPurchase callbacks: "p:t:<id>" toggles an item, "p:add:f" and
// "p:add:p" add to the family or personal list, "p:clean" deletes bought items
// and "p:r" refreshes.
const (
	ppToggle    = "t"
	ppAdd       = "add"
	ppClean     = "clean"
	ppRefresh   = "r"
	ppFamily    = "f"
	ppPersonal  = "p"
	ppSeparator = ":"
)

func (a *App) showPurchases(ctx context.Context, chatID int64, msgID int, userID int64) {
	items, err := a.api.PlannedPurchases(ctx, userID)
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}
	text, kb := renderPurchases(items)
	a.show(ctx, chatID, msgID, text, kb)
}

func (a *App) onPurchaseCallback(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64, arg string) {
	op, param, _ := strings.Cut(arg, ppSeparator)
	switch op {
	case ppToggle:
		a.togglePurchase(ctx, cq, chatID, msgID, userID, param)
	case ppAdd:
		isFamily := param == ppFamily
		a.sessions.set(userID, session{step: stepPurchaseName, isFamily: isFamily})
		a.answer(ctx, cq.ID, "", false)
		list := "личный"
		if isFamily {
			list = "семейный"
		}
		a.send(ctx, chatID, fmt.Sprintf("Что добавить в %s список? Можно несколько позиций, каждую с новой строки.", list), cancelKeyboard())
	case ppClean:
		a.cleanPurchases(ctx, cq, chatID, msgID, userID)
	case ppRefresh:
		a.answer(ctx, cq.ID, "", false)
		a.showPurchases(ctx, chatID, msgID, userID)
	default:
		a.answer(ctx, cq.ID, "Неизвестная кнопка.", false)
	}
}

func (a *App) togglePurchase(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64, id string) {
	if !isUUID(id) {
		a.answer(ctx, cq.ID, "Эта кнопка уже неактуальна.", true)
		return
	}
	// Re-read the item rather than trusting the button: another family member
	// may have changed it since the list was drawn.
	items, err := a.api.PlannedPurchases(ctx, userID)
	if err != nil {
		a.failCallback(ctx, cq.ID, err)
		return
	}
	i := slices.IndexFunc(items, func(p backend.PlannedPurchase) bool { return p.ID == id })
	if i < 0 {
		a.answer(ctx, cq.ID, "Этой позиции уже нет в списке.", false)
		text, kb := renderPurchases(items)
		a.edit(ctx, chatID, msgID, text, kb)
		return
	}

	updated, err := a.api.SetPlannedPurchaseBought(ctx, userID, id, !items[i].IsBought)
	if err != nil {
		a.failCallback(ctx, cq.ID, err)
		return
	}
	items[i] = updated
	a.answer(ctx, cq.ID, "", false)
	text, kb := renderPurchases(items)
	a.edit(ctx, chatID, msgID, text, kb)
}

func (a *App) cleanPurchases(ctx context.Context, cq *models.CallbackQuery, chatID int64, msgID int, userID int64) {
	items, err := a.api.PlannedPurchases(ctx, userID)
	if err != nil {
		a.failCallback(ctx, cq.ID, err)
		return
	}
	removed := 0
	for _, p := range items {
		if !p.IsBought {
			continue
		}
		if err := a.api.DeletePlannedPurchase(ctx, userID, p.ID); err != nil {
			a.failCallback(ctx, cq.ID, err)
			a.showPurchases(ctx, chatID, msgID, userID)
			return
		}
		removed++
	}
	if removed == 0 {
		a.answer(ctx, cq.ID, "Купленных позиций нет.", false)
		return
	}
	a.answer(ctx, cq.ID, fmt.Sprintf("Удалено: %d", removed), false)
	a.showPurchases(ctx, chatID, msgID, userID)
}

func (a *App) onPurchaseNames(ctx context.Context, chatID, userID int64, sess session, text string) {
	names := splitItems(text, maxItemsPerMessage)
	if len(names) == 0 {
		a.sessions.set(userID, sess)
		a.send(ctx, chatID, "Напишите название, например <code>Молоко 2 л</code>.", cancelKeyboard())
		return
	}
	a.sessions.clear(userID)

	for _, name := range names {
		req := backend.CreatePlannedPurchaseRequest{Name: name, IsFamily: sess.isFamily}
		if _, err := a.api.CreatePlannedPurchase(ctx, userID, req); err != nil {
			a.fail(ctx, chatID, err)
			break
		}
	}
	a.showPurchases(ctx, chatID, 0, userID)
}

func renderPurchases(items []backend.PlannedPurchase) (string, *models.InlineKeyboardMarkup) {
	// Not bought first, then by name, family list before personal.
	slices.SortStableFunc(items, func(x, y backend.PlannedPurchase) int {
		switch {
		case x.IsFamily != y.IsFamily:
			return boolOrder(!x.IsFamily, !y.IsFamily)
		case x.IsBought != y.IsBought:
			return boolOrder(x.IsBought, y.IsBought)
		}
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})

	var b strings.Builder
	b.WriteString("🛒 <b>Список покупок</b>\n")
	var buttons []models.InlineKeyboardButton
	for _, section := range []struct {
		title    string
		isFamily bool
	}{{"👨‍👩‍👧 Семейный", true}, {"👤 Личный", false}} {
		b.WriteString("\n<b>" + section.title + "</b>\n")
		empty := true
		for _, p := range items {
			if p.IsFamily != section.isFamily {
				continue
			}
			empty = false
			mark := "⬜"
			if p.IsBought {
				mark = "✅"
			}
			b.WriteString(mark + " " + escape(p.Name))
			if p.IsFamily {
				b.WriteString(" <i>(" + escape(p.AddedBy.DisplayName) + ")</i>")
			}
			b.WriteString("\n")
			if len(buttons) < maxPurchaseButtons {
				buttons = append(buttons, models.InlineKeyboardButton{
					Text:         mark + " " + p.Name,
					CallbackData: cbPurchase + ":" + ppToggle + ppSeparator + p.ID,
				})
			}
		}
		if empty {
			b.WriteString("<i>пусто</i>\n")
		}
	}
	if len(buttons) > 0 {
		b.WriteString("\nНажмите на позицию, чтобы отметить её купленной.")
	}

	rows := grid(buttons, 2)
	rows = append(rows,
		[]models.InlineKeyboardButton{
			{Text: "➕ В семейный", CallbackData: cbPurchase + ":" + ppAdd + ppSeparator + ppFamily},
			{Text: "➕ В личный", CallbackData: cbPurchase + ":" + ppAdd + ppSeparator + ppPersonal},
		},
		[]models.InlineKeyboardButton{
			{Text: "🧹 Убрать купленные", CallbackData: cbPurchase + ":" + ppClean},
			{Text: "🔄 Обновить", CallbackData: cbPurchase + ":" + ppRefresh},
		},
	)
	return b.String(), &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// boolOrder sorts false before true.
func boolOrder(x, y bool) int {
	switch {
	case x == y:
		return 0
	case !x:
		return -1
	}
	return 1
}
