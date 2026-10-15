package bot

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/koldassovnt/finance-tracker-telegram-bot/internal/backend"
)

const topExpenseCategories = 5

func (a *App) showSummary(ctx context.Context, chatID, userID int64) {
	today := a.today()
	s, err := a.api.MonthlySummary(ctx, userID, today.Format("2006-01"))
	if err != nil {
		a.fail(ctx, chatID, err)
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📊 <b>%s</b>\n\n", formatMonth(today))
	fmt.Fprintf(&b, "Доходы: <b>%s</b>\n", formatMoney(s.TotalIncome, s.Currency))
	fmt.Fprintf(&b, "Расходы: <b>%s</b>\n", formatMoney(s.TotalExpense, s.Currency))
	net := formatMoney(s.Net, s.Currency)
	if toFloat(s.Net) > 0 {
		net = "+" + net
	}
	fmt.Fprintf(&b, "Итого: <b>%s</b>\n", net)

	cats := slices.Clone(s.ExpenseByCategory)
	// Float only for ordering; the amounts shown stay exact.
	slices.SortFunc(cats, func(x, y backend.CategorySummary) int {
		return cmp.Compare(toFloat(y.Total), toFloat(x.Total))
	})
	if len(cats) > 0 {
		b.WriteString("\n<b>Крупные расходы</b>\n")
		for _, c := range cats[:min(len(cats), topExpenseCategories)] {
			name := "Без категории"
			if c.CategoryName != nil {
				name = *c.CategoryName
			}
			fmt.Fprintf(&b, "• %s: %s\n", escape(name), formatMoney(c.Total, s.Currency))
		}
	}
	a.send(ctx, chatID, b.String(), nil)
}

func toFloat(n fmt.Stringer) float64 {
	f, _ := strconv.ParseFloat(n.String(), 64)
	return f
}
